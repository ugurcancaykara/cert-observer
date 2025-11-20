package metrics

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"

	"github.com/ugurcancaykara/cert-observer/internal/cache"
)

// Handler serves a simple metrics endpoint
type Handler struct {
	cache *cache.IngressCache
	log   logr.Logger
}

// NewHandler creates a new metrics handler
func NewHandler(ingressCache *cache.IngressCache, logger logr.Logger) *Handler {
	return &Handler{
		cache: ingressCache,
		log:   logger,
	}
}

// ServeHTTP handles /metrics requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ingresses := h.cache.GetAll()
	count := len(ingresses)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprintf(w, "# HELP cert_observer_ingresses_total Total number of observed ingresses\n"); err != nil {
		h.log.V(1).Info("failed to write metrics help line", "error", err.Error())
	}
	if _, err := fmt.Fprintf(w, "# TYPE cert_observer_ingresses_total gauge\n"); err != nil {
		h.log.V(1).Info("failed to write metrics type line", "error", err.Error())
	}
	if _, err := fmt.Fprintf(w, "cert_observer_ingresses_total %d\n", count); err != nil {
		h.log.V(1).Info("failed to write metrics value", "error", err.Error())
	}
}
