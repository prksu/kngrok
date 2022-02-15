/*
Copyright 2022 The Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/prksu/kngrok/ngrok"
	nerrors "github.com/prksu/kngrok/ngrok/errors"
	"github.com/prksu/kngrok/util"
	"github.com/prksu/kngrok/util/patch"
)

const ControllerName = "service.k-ngrok.io/controller"

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Recorder          record.EventRecorder
	LoadBalancerClass string
}

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, builder.WithPredicates(r.ServiceWithLoadBalancerClass())).
		Complete(r)
}

// ServiceWithLoadBalancerClass returns predicate funcs that filter the service
// with given LoadBalancer class name on CREATE, UPDATE, DELETE and GENERIC events.
func (r *ServiceReconciler) ServiceWithLoadBalancerClass() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			return false
		}

		return r.LoadBalancerClass == pointer.StringDeref(svc.Spec.LoadBalancerClass, "")
	})
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	svc := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, svc); err != nil {
		if apierrors.IsNotFound(err) {
			// Return early if requested service is not found.
			log.V(1).Info("Requested service is not found or already deleted")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if svc.Spec.ClusterIP == "" {
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	patcher, err := patch.NewPatcher(r.Client, svc)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := patcher.Patch(ctx, svc, client.FieldOwner(ControllerName)); err != nil {
			reterr = err
		}
	}()

	if !svc.GetDeletionTimestamp().IsZero() {
		return r.reconcileDeletion(ctx, svc)
	}

	return r.reconcile(ctx, svc)
}

func (r *ServiceReconciler) reconcile(ctx context.Context, svc *corev1.Service) (_ ctrl.Result, reterr error) {
	var (
		log              = ctrl.LoggerFrom(ctx)
		errs             []error
		ingress          []corev1.LoadBalancerIngress
		currentTunnelSet = sets.NewString()
		desiredTunnelSet = sets.NewString()
	)

	if svc.Annotations == nil {
		svc.Annotations = make(map[string]string)
	}

	if r, ok := svc.Annotations["service.k-ngrok.io/tunnels"]; ok {
		// populate currentTunnelSet record from registry annotation.
		// it will be compared against desiredTunnelSet that will be
		// populated according to the desired service ports
		var cur []string
		if err := json.NewDecoder(bytes.NewReader([]byte(r))).Decode(&cur); err != nil {
			return ctrl.Result{}, err
		}

		currentTunnelSet.Insert(cur...)
	}

	defer func() {
		// always store actual running tunnel names into registry annotation.
		registry := new(bytes.Buffer)
		if err := json.NewEncoder(registry).Encode(desiredTunnelSet.List()); err != nil {
			reterr = err
		}

		svc.Annotations["service.k-ngrok.io/tunnels"] = registry.String()
	}()

	controllerutil.AddFinalizer(svc, ControllerName)
	svckey := client.ObjectKeyFromObject(svc)
	for _, sp := range svc.Spec.Ports {
		tunnelName := strings.ReplaceAll(svckey.String(), "/", "-")
		if len(svc.Spec.Ports) > 1 || sp.Name != "" {
			// add port name in the end of tunnelName when it's defined or
			// more than one ports is defined.
			tunnelName = tunnelName + "-" + sp.Name
		}

		log.V(1).Info("Find existing tunnel", "tunnelName", tunnelName)
		tunnel, err := ngrok.DefaultAgent.Find(ctx, tunnelName)
		if err != nil && !nerrors.IsNotFound(err) {
			log.V(1).Error(err, "Unable to find existing tunnel")
			errs = append(errs, err)
			continue
		}

		if tunnel == nil || nerrors.IsNotFound(err) {
			// start new tunnel if it is not exist.
			log.V(1).Info("No existing tunnel found. Starting new tunnel", "tunnelName", tunnelName)
			if tunnel, err = ngrok.DefaultAgent.Start(ctx, tunnelName, ngrok.TunnelConfig{
				Addr:  net.JoinHostPort(svc.Spec.ClusterIP, strconv.Itoa(int(sp.Port))),
				Proto: strings.ToLower(string(sp.Protocol)),
			}); err != nil {
				log.Error(err, "Unable to starting new tunnel", "tunnelName", tunnelName)
				errs = append(errs, err)
				continue
			}

			u, _ := url.Parse(tunnel.PublicURL)
			log.V(1).Info("Started ngrok tunnel", "tunnelName", tunnelName, "port", sp.Port, "on", u.Host)
			r.Recorder.Eventf(svc, corev1.EventTypeNormal, "TunnelStarted", "Started ngrok tunnel for port: '%d' on addr: %s", sp.Port, u.Host)
		}

		hostname, port, err := util.SplitHostPort(tunnel.PublicURL)
		if err != nil {
			log.Error(err, "Unable to parse tunnel public_url", "tunnelName", tunnelName)
			errs = append(errs, err)
			// insert the new started tunnel into currentTunnelSet if we got unexpected error here
			// and treat the tunnel to be stale so it will be stopped soon.
			currentTunnelSet.Insert(tunnelName)
			continue
		}

		desiredTunnelSet.Insert(tunnelName)
		ingress = append(ingress, corev1.LoadBalancerIngress{
			Hostname: hostname,
			Ports: []corev1.PortStatus{
				{
					Port:     port,
					Protocol: corev1.Protocol(strings.ToUpper(tunnel.Proto)),
				},
			},
		})
	}

	if err := func() error {
		log.V(1).Info("Ensure no stale tunnel is running")
		var errs []error
		for _, tunnelName := range currentTunnelSet.Difference(desiredTunnelSet).List() {
			// stop any tunnel in the currentTunnelSet that are not in the desiredTunnelSet
			// to keep the actual tunnel running as desired.
			log.V(1).Info("Stopping stale tunnel", "tunnelName", tunnelName)
			if err := ngrok.DefaultAgent.Stop(ctx, tunnelName); err != nil && !nerrors.IsNotFound(err) {
				errs = append(errs, err)
				continue
			}

			log.V(1).Info("Stopped stale tunnel", "tunnelName", tunnelName)
		}

		return kerrors.NewAggregate(errs)
	}(); err != nil {
		return ctrl.Result{}, err
	}

	if err := kerrors.NewAggregate(errs); err != nil {
		return ctrl.Result{}, err
	}

	svc.Status.LoadBalancer.Ingress = ingress
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) reconcileDeletion(ctx context.Context, svc *corev1.Service) (ctrl.Result, error) {
	var (
		log  = ctrl.LoggerFrom(ctx)
		errs []error
	)

	svckey := client.ObjectKeyFromObject(svc)
	for _, sp := range svc.Spec.Ports {
		tunnelName := strings.ReplaceAll(svckey.String(), "/", "-")
		if len(svc.Spec.Ports) > 1 || sp.Name != "" {
			// add port name in the end of tunnelName when it's defined or
			// more than one ports is defined.
			tunnelName = tunnelName + "-" + sp.Name
		}

		log.Info("Stopping tunnel", "tunnelName", tunnelName)
		if err := ngrok.DefaultAgent.Stop(ctx, tunnelName); err != nil && !nerrors.IsNotFound(err) {
			log.Error(err, "Failed stopping the tunnel", "tunnelName", tunnelName)
			errs = append(errs, err)
			continue
		}

		log.V(1).Info("Stopped tunnel", "tunnelName", tunnelName)
	}

	if err := kerrors.NewAggregate(errs); err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(svc, ControllerName)
	return ctrl.Result{}, nil
}
