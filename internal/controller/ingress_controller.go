package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/ugurcancaykara/cert-observer/internal/cache"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	ctx := context.Background()

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

	// Fetch certificate expiry for all secrets
	certExpiry := make(map[string]*cache.CertificateInfo)
	for _, tls := range ingress.Spec.TLS {
		if tls.SecretName != "" {
			if _, exists := certExpiry[tls.SecretName]; !exists {
				// Fetch secret and extract expiry
				var secret corev1.Secret
				if err := r.Get(ctx, types.NamespacedName{
					Namespace: ingress.Namespace,
					Name:      tls.SecretName,
				}, &secret); err != nil {
					// Secret doesn't exist or can't be fetched, create cert info without expiry
					certExpiry[tls.SecretName] = &cache.CertificateInfo{
						Name:    tls.SecretName,
						Expires: nil,
					}
				} else {
					// Extract certificate expiry
					expiryTime, err := r.extractCertificateExpiry(&secret)
					certExpiry[tls.SecretName] = &cache.CertificateInfo{
						Name:    tls.SecretName,
						Expires: expiryTime,
					}
					if err != nil {
						// Log but don't fail - we still want to track the ingress
						ctrl.Log.V(1).Info("failed to extract certificate expiry",
							"secret", tls.SecretName,
							"error", err.Error())
					}
				}
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
			if certInfo, exists := certExpiry[certName]; exists {
				hostInfo.Certificate = certInfo
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

// extractCertificateExpiry parses the certificate and extracts the NotAfter time
func (r *IngressReconciler) extractCertificateExpiry(secret *corev1.Secret) (*time.Time, error) {
	// Get certificate data
	certData, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, fmt.Errorf("secret does not contain tls.crt")
	}

	// Try to decode PEM block
	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &cert.NotAfter, nil
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
