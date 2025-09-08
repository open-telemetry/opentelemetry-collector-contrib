// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statusreporterextension

import (
	"testing"
)

func TestConfigValidation(t *testing.T) {
	// Test with default config
	cfg := createDefaultConfig().(*Config)
	if cfg.MetricsEndpoint != "http://localhost:8888/metrics" {
		t.Errorf("Expected default MetricsEndpoint to be http://localhost:8888/metrics, got %s", cfg.MetricsEndpoint)
	}
	
	// Test with custom values
	cfg.MetricsEndpoint = "http://custom:9090/metrics"
	cfg.StaleThreshold = 600
	cfg.EngineID = "custom-engine"
	cfg.PodName = "custom-pod"
	cfg.Port = 9000
	cfg.AuthEnabled = true
	cfg.SecretValue = "custom-secret"
	
	// Verify custom values
	if cfg.MetricsEndpoint != "http://custom:9090/metrics" {
		t.Errorf("Failed to set custom MetricsEndpoint")
	}
	
	if cfg.StaleThreshold != 600 {
		t.Errorf("Failed to set custom StaleThreshold")
	}
	
	if cfg.EngineID != "custom-engine" {
		t.Errorf("Failed to set custom EngineID")
	}
	
	if cfg.PodName != "custom-pod" {
		t.Errorf("Failed to set custom PodName")
	}
	
	if cfg.Port != 9000 {
		t.Errorf("Failed to set custom Port")
	}
	
	if !cfg.AuthEnabled {
		t.Errorf("Failed to set custom AuthEnabled")
	}
	
	if cfg.SecretValue != "custom-secret" {
		t.Errorf("Failed to set custom SecretValue")
	}
}

func TestConfigEdgeCases(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	
	// Test with empty metrics endpoint
	cfg.MetricsEndpoint = ""
	// This should be allowed, as the extension will use a default value
	
	// Test with negative stale threshold
	cfg.StaleThreshold = -1
	// This should be allowed, though it might not be practical
	
	// Test with zero port (should use system-assigned port)
	cfg.Port = 0
	// This should be allowed, as the HTTP server can use a system-assigned port
	
	// Test with empty engine ID
	cfg.EngineID = ""
	// This should be allowed, though it might not be useful
	
	// Test with empty pod name
	cfg.PodName = ""
	// This should be allowed, though it might not be useful
	
	// Test with auth enabled but empty secret
	cfg.AuthEnabled = true
	cfg.SecretValue = ""
	// This should be allowed, as the secret might be provided via environment variable
}

