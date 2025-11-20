package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	"github.com/ugurcancaykara/cert-observer/internal/cache"
	"github.com/ugurcancaykara/cert-observer/internal/config"
)

// Report represents the JSON structure sent to the endpoint
type Report struct {
	Cluster   string               `json:"cluster"`
	Ingresses []*cache.IngressInfo `json:"ingresses"`
}

// HTTPReporter periodically sends reports to an HTTP endpoint
type HTTPReporter struct {
	config       *config.Config
	cache        *cache.IngressCache
	client       *http.Client
	log          logr.Logger
	failureCount int
}

// NewHTTPReporter creates a new HTTPReporter instance
func NewHTTPReporter(cfg *config.Config, ingressCache *cache.IngressCache, log logr.Logger) *HTTPReporter {
	return &HTTPReporter{
		config: cfg,
		cache:  ingressCache,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		log: log,
	}
}

// Start begins the periodic reporting loop
func (r *HTTPReporter) Start(ctx context.Context) {
	r.log.Info("starting HTTP reporter", "interval", r.config.ReportInterval, "endpoint", r.config.ReportEndpoint)

	// Send initial report
	if err := r.sendReport(ctx); err != nil {
		r.handleReportError(err, true)
	}

	ticker := time.NewTicker(r.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("stopping HTTP reporter")
			return
		case <-ticker.C:
			if err := r.sendReport(ctx); err != nil {
				r.handleReportError(err, false)
			}
		}
	}
}

// handleReportError provides intelligent error logging based on error type and state
func (r *HTTPReporter) handleReportError(err error, isInitial bool) {
	r.failureCount++

	// Check if this is a DNS/connection error (server not available)
	if isServerUnavailable(err) {
		if isInitial || r.failureCount == 1 {
			r.log.Info("waiting for report server to be available", "endpoint", r.config.ReportEndpoint)
		} else if r.failureCount%5 == 0 {
			// Log every 5th failure to avoid spam
			r.log.V(1).Info("report server still unavailable", "failures", r.failureCount, "endpoint", r.config.ReportEndpoint)
		} else {
			// Use debug level for other retries
			r.log.V(2).Info("report server not reachable, will retry", "endpoint", r.config.ReportEndpoint)
		}
		return
	}

	// For other errors, always log
	if isInitial {
		r.log.Error(err, "failed to send initial report")
	} else {
		r.log.Error(err, "failed to send periodic report")
	}
}

// isServerUnavailable checks if the error is due to server being unavailable
func isServerUnavailable(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isServerUnavailable(urlErr.Err)
	}

	return false
}

// sendReport generates and sends a report to the configured endpoint
func (r *HTTPReporter) sendReport(ctx context.Context) error {
	// Get all ingress data from cache
	ingresses := r.cache.GetAll()

	report := Report{
		Cluster:   r.config.ClusterName,
		Ingresses: ingresses,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Retry logic with exponential backoff
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "POST", r.config.ReportEndpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.client.Do(req)
		if err != nil {
			// Only log detailed errors on last attempt or non-connection errors
			if attempt == maxRetries && !isServerUnavailable(err) {
				r.log.Error(err, "failed to send report after retries", "endpoint", r.config.ReportEndpoint, "attempts", maxRetries)
			}
			if attempt < maxRetries {
				// Exponential backoff: 2s, 4s
				backoff := time.Duration(attempt) * 2 * time.Second
				time.Sleep(backoff)
				continue
			}
			return err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				r.log.V(1).Info("failed to close response body", "error", err.Error())
			}
		}()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			r.log.Info("report sent successfully", "endpoint", r.config.ReportEndpoint, "status", resp.StatusCode, "ingress_count", len(ingresses))
			r.failureCount = 0 // Reset failure count on success
			return nil
		}

		// Non-2xx status code
		if attempt < maxRetries {
			r.log.V(1).Info("retrying after non-success status", "status", resp.StatusCode, "attempt", attempt)
			backoff := time.Duration(attempt) * 2 * time.Second
			time.Sleep(backoff)
			continue
		}

		return fmt.Errorf("received non-success status code: %d", resp.StatusCode)
	}

	return fmt.Errorf("failed to send report after %d attempts", maxRetries)
}
