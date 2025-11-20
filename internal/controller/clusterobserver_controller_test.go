/*
Copyright 2025.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	observerv1alpha1 "github.com/ugurcancaykara/cert-observer/api/v1alpha1"
	"github.com/ugurcancaykara/cert-observer/internal/cache"
)

var _ = Describe("ClusterObserver Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		clusterobserver := &observerv1alpha1.ClusterObserver{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ClusterObserver")
			err := k8sClient.Get(ctx, typeNamespacedName, clusterobserver)
			if err != nil && errors.IsNotFound(err) {
				resource := &observerv1alpha1.ClusterObserver{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: observerv1alpha1.ClusterObserverSpec{
						ClusterName:    "test-cluster",
						ReportEndpoint: "http://test-server:8080/report",
						ReportInterval: "30s",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &observerv1alpha1.ClusterObserver{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ClusterObserver")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			ingressCache := cache.NewIngressCache("test-cluster")
			controllerReconciler := &ClusterObserverReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Cache:  ingressCache,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
