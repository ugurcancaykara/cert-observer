package controller

import (
	"context"
	"fmt"

	"github.com/ugurcancaykara/cert-observer/internal/cache"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IngressReconciler reconciles Ingress resources
type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  *cache.IngressCache
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles Ingress resource changes
func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("reconciling ingress", "namespace", req.Namespace, "name", req.Name)

	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Ingress deleted, remove from cache
			log.Info("ingress deleted, removing from cache", "namespace", req.Namespace, "name", req.Name)
			r.Cache.Delete(req.Namespace, req.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get ingress", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, fmt.Errorf("failed to get ingress %s/%s: %w", req.Namespace, req.Name, err)
	}

	// Extract and cache Ingress information
	if err := r.updateCache(&ingress); err != nil {
		log.Error(err, "failed to update cache", "ingress", req.NamespacedName)
		return ctrl.Result{}, err
	}

	log.V(1).Info("successfully updated cache", "ingress", req.NamespacedName)
	return ctrl.Result{}, nil
}

// updateCache extracts Ingress information and updates the cache
func (r *IngressReconciler) updateCache(ingress *networkingv1.Ingress) error {
	// Preserve certificate expiry dates from cache when rebuilding IngressInfo
	existingEntries := r.Cache.GetAll()
	existingCerts := make(map[string]map[string]*cache.CertificateInfo)
	for _, entry := range existingEntries {
		if entry.Namespace == ingress.Namespace {
			if existingCerts[entry.Namespace] == nil {
				existingCerts[entry.Namespace] = make(map[string]*cache.CertificateInfo)
			}
			for _, host := range entry.Hosts {
				if host.Certificate != nil && host.Certificate.Expires != nil {
					if existing, ok := existingCerts[entry.Namespace][host.Certificate.Name]; !ok || existing.Expires == nil {
						existingCerts[entry.Namespace][host.Certificate.Name] = host.Certificate
					}
				}
			}
		}
	}

	// Extract hosts from rules
	hosts := make(map[string]bool)
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hosts[rule.Host] = true
		}
	}

	// If no rules with hosts, check TLS for hosts
	if len(hosts) == 0 {
		for _, tls := range ingress.Spec.TLS {
			for _, host := range tls.Hosts {
				if host != "" {
					hosts[host] = true
				}
			}
		}
	}

	// Create a map of host to certificate (from TLS spec)
	hostToCert := make(map[string]string)
	for _, tls := range ingress.Spec.TLS {
		for _, host := range tls.Hosts {
			if tls.SecretName != "" {
				hostToCert[host] = tls.SecretName
			}
		}
	}

	// Build single IngressInfo with all hosts
	info := &cache.IngressInfo{
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
		Hosts:     make([]cache.HostInfo, 0, len(hosts)),
	}

	// Add each host with its certificate info
	for host := range hosts {
		hostInfo := cache.HostInfo{
			Host: host,
		}

		// Add certificate info if available
		if certName, ok := hostToCert[host]; ok {
			// Check if we have existing certificate info with expiry date
			if existingCert, exists := existingCerts[ingress.Namespace][certName]; exists {
				hostInfo.Certificate = existingCert
			} else {
				hostInfo.Certificate = &cache.CertificateInfo{
					Name:    certName,
					Expires: nil,
				}
			}
		}

		info.Hosts = append(info.Hosts, hostInfo)
	}

	// If no hosts found at all, create an entry with empty host
	if len(hosts) == 0 {
		info.Hosts = append(info.Hosts, cache.HostInfo{
			Host: "",
		})
	}

	r.Cache.Add(info)
	return nil
}

// findIngressesForSecret returns reconcile requests for all Ingresses that use the given Secret
func (r *IngressReconciler) findIngressesForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	log := log.FromContext(ctx)

	var ingressList networkingv1.IngressList
	if err := r.List(ctx, &ingressList, client.InNamespace(secret.GetNamespace())); err != nil {
		log.Error(err, "failed to list ingresses", "namespace", secret.GetNamespace())
		return []reconcile.Request{}
	}

	var requests []reconcile.Request
	for _, ingress := range ingressList.Items {
		for _, tls := range ingress.Spec.TLS {
			if tls.SecretName == secret.GetName() {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&ingress),
				})
				log.V(1).Info("secret change triggers ingress reconciliation",
					"secret", secret.GetName(),
					"ingress", ingress.Name,
					"namespace", ingress.Namespace)
				break
			}
		}
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findIngressesForSecret),
		).
		Complete(r)
}
