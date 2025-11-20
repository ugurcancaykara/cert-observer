package metrics

import (
	"fmt"
	"net/http"

	"github.com/ugurcancaykara/cert-observer/internal/cache"
)

// Handler serves a simple metrics endpoint
type Handler struct {
	cache *cache.IngressCache
}

// NewHandler creates a new metrics handler
func NewHandler(ingressCache *cache.IngressCache) *Handler {
	return &Handler{
		cache: ingressCache,
	}
}

// ServeHTTP handles /metrics requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ingresses := h.cache.GetAll()
	count := len(ingresses)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "# HELP cert_observer_ingresses_total Total number of observed ingresses\n")
	_, _ = fmt.Fprintf(w, "# TYPE cert_observer_ingresses_total gauge\n")
	_, _ = fmt.Fprintf(w, "cert_observer_ingresses_total %d\n", count)
}
