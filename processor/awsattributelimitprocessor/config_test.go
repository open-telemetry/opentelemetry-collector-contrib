// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsattributelimitprocessor

import (
	"testing"
)

func TestConfigValidate_Default(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	if cfg.MaxTotalAttributes != 150 {
		t.Errorf("expected default MaxTotalAttributes=150, got %d", cfg.MaxTotalAttributes)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid, got error: %v", err)
	}
}

func TestConfigValidate_Valid(t *testing.T) {
	cfg := &Config{MaxTotalAttributes: 100}
	if err := cfg.Validate(); err != nil {
		t.Errorf("config with MaxTotalAttributes=100 should be valid, got error: %v", err)
	}
}

func TestConfigValidate_Zero(t *testing.T) {
	cfg := &Config{MaxTotalAttributes: 0}
	if err := cfg.Validate(); err == nil {
		t.Error("config with MaxTotalAttributes=0 should return error")
	}
}

func TestConfigValidate_Negative(t *testing.T) {
	cfg := &Config{MaxTotalAttributes: -1}
	if err := cfg.Validate(); err == nil {
		t.Error("config with MaxTotalAttributes=-1 should return error")
	}
}

func TestConfigValidate_ExceedsMax(t *testing.T) {
	cfg := &Config{MaxTotalAttributes: 200}
	if err := cfg.Validate(); err == nil {
		t.Error("config with MaxTotalAttributes=200 should return error")
	}
}

func TestConfigValidate_AtMax(t *testing.T) {
	cfg := &Config{MaxTotalAttributes: 150}
	if err := cfg.Validate(); err != nil {
		t.Errorf("config with MaxTotalAttributes=150 should be valid, got error: %v", err)
	}
}
