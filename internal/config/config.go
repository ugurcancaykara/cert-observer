package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	ClusterName    string
	ReportEndpoint string
	ReportInterval time.Duration
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		ClusterName:    getEnv("CLUSTER_NAME", "local-cluster"),
		ReportEndpoint: getEnv("REPORT_ENDPOINT", "http://localhost:8080/report"),
	}

	// Parse report interval
	intervalStr := getEnv("REPORT_INTERVAL", "30s")
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid REPORT_INTERVAL: %w", err)
	}
	cfg.ReportInterval = interval

	return cfg, nil
}

// getEnv retrieves environment variable with fallback to default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
