package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/ugurcancaykara/cert-observer/internal/cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SecretReconciler reconciles Secret objects to extract certificate expiry
type SecretReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cache  *cache.IngressCache
}

//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile handles Secret resource changes
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.V(1).Info("reconciling secret", "namespace", req.Namespace, "name", req.Name)

	var secret corev1.Secret
	if err := r.Get(ctx, req.NamespacedName, &secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Secret deleted, clear expiry information
			log.V(1).Info("secret deleted, clearing expiry", "namespace", req.Namespace, "name", req.Name)
			r.Cache.UpdateCertificate(req.Namespace, req.Name, nil)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get secret", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, fmt.Errorf("failed to get secret %s/%s: %w", req.Namespace, req.Name, err)
	}

	// Only process TLS secrets
	if secret.Type != corev1.SecretTypeTLS {
		log.V(1).Info("skipping non-TLS secret", "type", secret.Type)
		return ctrl.Result{}, nil
	}

	// Parse certificate and extract expiry
	expiryTime, err := r.extractCertificateExpiry(&secret)
	if err != nil {
		log.Error(err, "failed to parse certificate, skipping", "secret", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Update cache with expiry information
	if expiryTime != nil {
		r.Cache.UpdateCertificate(req.Namespace, req.Name, expiryTime)
		log.Info("updated certificate expiry", "secret", req.NamespacedName, "expires", expiryTime.Format("2006-01-02"))
	}

	return ctrl.Result{}, nil
}

// extractCertificateExpiry parses the certificate and extracts the NotAfter time
func (r *SecretReconciler) extractCertificateExpiry(secret *corev1.Secret) (*time.Time, error) {
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

// SetupWithManager sets up the controller with the Manager
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only watch TLS secrets
	tlsSecretPredicate := predicate.NewPredicateFuncs(func(object client.Object) bool {
		secret, ok := object.(*corev1.Secret)
		if !ok {
			return false
		}
		return secret.Type == corev1.SecretTypeTLS
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(tlsSecretPredicate).
		Complete(r)
}
