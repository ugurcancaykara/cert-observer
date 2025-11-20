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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	observerv1alpha1 "github.com/ugurcancaykara/cert-observer/api/v1alpha1"
	"github.com/ugurcancaykara/cert-observer/internal/cache"
)

// ClusterObserverReconciler reconciles a ClusterObserver object
type ClusterObserverReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  *cache.IngressCache
}

// +kubebuilder:rbac:groups=observer.cert-observer.io,resources=clusterobservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=observer.cert-observer.io,resources=clusterobservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=observer.cert-observer.io,resources=clusterobservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ClusterObserverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ClusterObserver instance
	observer := &observerv1alpha1.ClusterObserver{}
	if err := r.Get(ctx, req.NamespacedName, observer); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get ClusterObserver")
		return ctrl.Result{}, err
	}

	// Validate report interval
	if _, err := time.ParseDuration(observer.Spec.ReportInterval); err != nil {
		logger.Error(err, "invalid report interval", "interval", observer.Spec.ReportInterval)
		return ctrl.Result{}, err
	}

	// Update status with current ingress count
	ingresses := r.Cache.GetAll()
	observer.Status.IngressCount = len(ingresses)

	if err := r.Status().Update(ctx, observer); err != nil {
		logger.Error(err, "failed to update ClusterObserver status")
		return ctrl.Result{}, err
	}

	logger.Info("reconciled ClusterObserver",
		"name", observer.Name,
		"cluster", observer.Spec.ClusterName,
		"ingress_count", observer.Status.IngressCount)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterObserverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&observerv1alpha1.ClusterObserver{}).
		Named("clusterobserver").
		Complete(r)
}
