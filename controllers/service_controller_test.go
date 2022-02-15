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
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prksu/kngrok/util"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RunnerReconciler", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	var (
		ctx = context.Background()
		svc *corev1.Service
	)

	BeforeEach(func() {
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-svc-" + util.RandomString(4),
				Namespace: testns.Name,
			},
			Spec: corev1.ServiceSpec{
				Type:              corev1.ServiceTypeLoadBalancer,
				LoadBalancerClass: pointer.String("service.k-ngrok.io/controller"),
			},
		}
	})

	AfterEach(func() {
		By("Cleanup service")
		Expect(client.IgnoreNotFound(crclient.Delete(ctx, svc))).Should(Succeed())
		Eventually(func() bool {
			err := crclient.Get(ctx, client.ObjectKeyFromObject(svc), svc)
			return apierrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
	})

	Describe("Reconcile", func() {
		Context("When Loadbalancer Service just created", func() {
			It("Should start tunnel and propagate ingress status", func() {
				svc.Spec.Ports = []corev1.ServicePort{
					{
						Protocol: corev1.ProtocolTCP,
						Port:     1234,
					},
				}

				By("Creating new Loadbalancer Service")
				Expect(crclient.Create(ctx, svc)).Should(Succeed())

				By("Waiting Loadbalancer Ingress hostname to be propagated")
				Eventually(func() bool {
					Expect(crclient.Get(ctx, client.ObjectKeyFromObject(svc), svc)).ToNot(HaveOccurred())
					return len(svc.Status.LoadBalancer.Ingress) == 1
				}, timeout, interval).Should(BeTrue())
			})
		})

		Context("When Loadbalancer Service with two ports just created", func() {
			It("Should start two tunnel and propagate ingress status", func() {
				svc.Spec.Ports = []corev1.ServicePort{
					{
						Name:     "port-a",
						Protocol: corev1.ProtocolTCP,
						Port:     1234,
					},
					{
						Name:     "port-b",
						Protocol: corev1.ProtocolTCP,
						Port:     6789,
					},
				}

				By("Creating new Loadbalancer Service")
				Expect(crclient.Create(ctx, svc)).Should(Succeed())

				By("Waiting Loadbalancer Ingress hostname to be propagated")
				Eventually(func() bool {
					Expect(crclient.Get(ctx, client.ObjectKeyFromObject(svc), svc)).ToNot(HaveOccurred())
					return len(svc.Status.LoadBalancer.Ingress) == 2
				}, timeout, interval).Should(BeTrue())
			})
		})

		Context("When updating Loadbalancer Service from single to multiple ports", func() {
			It("Should start new tunnel with named port and stop the unnamed port (stale) tunnel", func() {
				svc.Spec.Ports = []corev1.ServicePort{
					{
						Protocol: corev1.ProtocolTCP,
						Port:     1234,
					},
				}

				By("Creating new Loadbalancer Service")
				Expect(crclient.Create(ctx, svc)).Should(Succeed())

				By("Waiting Loadbalancer Ingress hostname to be propagated")
				Eventually(func() bool {
					Expect(crclient.Get(ctx, client.ObjectKeyFromObject(svc), svc)).ToNot(HaveOccurred())
					return len(svc.Status.LoadBalancer.Ingress) == 1
				}, timeout, interval).Should(BeTrue())

				svc.Spec.Ports = []corev1.ServicePort{
					{
						Name:     "port-a",
						Protocol: corev1.ProtocolTCP,
						Port:     1234,
					},
					{
						Name:     "port-b",
						Protocol: corev1.ProtocolTCP,
						Port:     5678,
					},
				}

				By("Updating Loadbalancer Service ports")
				Expect(crclient.Update(ctx, svc)).Should(Succeed())

				By("Waiting Loadbalancer Ingress hostname to be propagated")
				Eventually(func() bool {
					Expect(crclient.Get(ctx, client.ObjectKeyFromObject(svc), svc)).ToNot(HaveOccurred())
					return len(svc.Status.LoadBalancer.Ingress) == 2
				}, timeout, interval).Should(BeTrue())

			})
		})
	})
})
