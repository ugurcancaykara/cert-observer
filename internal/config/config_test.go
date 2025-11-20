package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name         string
		envVars      map[string]string
		wantCluster  string
		wantURL      string
		wantInterval time.Duration
		wantErr      bool
	}{
		{
			name:         "default values",
			envVars:      map[string]string{},
			wantCluster:  "local-cluster",
			wantURL:      "http://localhost:8080/report",
			wantInterval: 30 * time.Second,
			wantErr:      false,
		},
		{
			name: "custom values",
			envVars: map[string]string{
				"CLUSTER_NAME":    "prod-cluster",
				"REPORT_ENDPOINT": "http://collector.example.com/report",
				"REPORT_INTERVAL": "1m",
			},
			wantCluster:  "prod-cluster",
			wantURL:      "http://collector.example.com/report",
			wantInterval: 1 * time.Minute,
			wantErr:      false,
		},
		{
			name: "invalid interval",
			envVars: map[string]string{
				"REPORT_INTERVAL": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for k, v := range tt.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("failed to set env var %s: %v", k, err)
				}
			}

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if cfg.ClusterName != tt.wantCluster {
				t.Errorf("ClusterName = %v, want %v", cfg.ClusterName, tt.wantCluster)
			}
			if cfg.ReportEndpoint != tt.wantURL {
				t.Errorf("ReportEndpoint = %v, want %v", cfg.ReportEndpoint, tt.wantURL)
			}
			if cfg.ReportInterval != tt.wantInterval {
				t.Errorf("ReportInterval = %v, want %v", cfg.ReportInterval, tt.wantInterval)
			}
		})
	}
}
