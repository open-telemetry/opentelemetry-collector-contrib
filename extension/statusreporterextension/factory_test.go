// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statusreporterextension

import (
	"testing"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	if cfg == nil {
		t.Fatal("Failed to create default config")
	}
	
	config, ok := cfg.(*Config)
	if !ok {
		t.Fatal("Config is not of type *Config")
	}
	
	// Verify default values
	if config.MetricsEndpoint != "http://localhost:8888/metrics" {
		t.Errorf("Expected default MetricsEndpoint to be http://localhost:8888/metrics, got %s", config.MetricsEndpoint)
	}
	
	if config.StaleThreshold != 300 {
		t.Errorf("Expected default StaleThreshold to be 300, got %d", config.StaleThreshold)
	}
	
	if config.EngineID != "unknown" {
		t.Errorf("Expected default EngineID to be unknown, got %s", config.EngineID)
	}
	
	if config.PodName != "unknown" {
		t.Errorf("Expected default PodName to be unknown, got %s", config.PodName)
	}
	
	if config.Port != 8080 {
		t.Errorf("Expected default Port to be 8080, got %d", config.Port)
	}
	
	if config.AuthEnabled != false {
		t.Errorf("Expected default AuthEnabled to be false, got %t", config.AuthEnabled)
	}
	
	if config.SecretValue != "secret" {
		t.Errorf("Expected default SecretValue to be secret, got %s", config.SecretValue)
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	if factory == nil {
		t.Fatal("Failed to create factory")
	}
	
	// Check factory type
	if factory.Type().String() != "statusreporter" {
		t.Errorf("Expected factory type to be statusreporter, got %s", factory.Type().String())
	}
}

