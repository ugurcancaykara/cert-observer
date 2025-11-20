package config

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	observerv1alpha1 "github.com/ugurcancaykara/cert-observer/api/v1alpha1"
)

// LoadFromCRD attempts to load configuration from a ClusterObserver CRD
// Returns nil if no CRD is found (reporter will not start)
func LoadFromCRD(ctx context.Context, k8sClient client.Client) (*Config, error) {
	// Try to get ClusterObserver from default namespace
	observer := &observerv1alpha1.ClusterObserver{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "clusterobserver-sample",
		Namespace: "default",
	}, observer)

	if err != nil {
		// Log the error for debugging
		// CRD not found - return nil (no reporting)
		return nil, nil
	}

	// Parse report interval
	interval, err := time.ParseDuration(observer.Spec.ReportInterval)
	if err != nil {
		return nil, err
	}

	return &Config{
		ClusterName:    observer.Spec.ClusterName,
		ReportEndpoint: observer.Spec.ReportEndpoint,
		ReportInterval: interval,
	}, nil
}
