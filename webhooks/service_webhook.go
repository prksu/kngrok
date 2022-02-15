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

package webhooks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type ServiceWebhook struct {
	Client            client.Reader
	LoadBalancerClass string
}

// SetupWithManager sets up the webhook with the Manager.
func (w *ServiceWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Service{}).
		WithDefaulter(w).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:path=/mutate--v1-service,mutating=true,failurePolicy=fail,sideEffects=None,groups=core,resources=services,verbs=create;update,versions=v1,name=mservice.k-ngrok.io,admissionReviewVersions=v1

var _ webhook.CustomDefaulter = &ServiceWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *ServiceWebhook) Default(ctx context.Context, obj runtime.Object) error {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Service but got a %T", obj))
	}

	if w.LoadBalancerClass != pointer.StringDeref(svc.Spec.LoadBalancerClass, "") {
		return nil
	}

	svc.Spec.AllocateLoadBalancerNodePorts = pointer.Bool(false)
	return nil
}

// +kubebuilder:webhook:path=/validate--v1-service,mutating=false,failurePolicy=fail,sideEffects=None,groups=core,resources=services,verbs=create;update,versions=v1,name=vservice.k-ngrok.io,admissionReviewVersions=v1

var _ webhook.CustomValidator = &ServiceWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *ServiceWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *ServiceWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *ServiceWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
