// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsattributelimitprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	if f == nil {
		t.Fatal("NewFactory returned nil")
	}
	if f.Type().String() != "awsattributelimit" {
		t.Errorf("expected type 'awsattributelimit', got %q", f.Type().String())
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	c, ok := cfg.(*Config)
	if !ok {
		t.Fatalf("expected *Config, got %T", cfg)
	}
	if c.MaxTotalAttributes != 150 {
		t.Errorf("expected default MaxTotalAttributes=150, got %d", c.MaxTotalAttributes)
	}
}

func TestCreateMetricsProcessor_ValidConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	p, err := f.CreateMetrics(
		t.Context(),
		processortest.NewNopSettings(component.MustNewType("awsattributelimit")),
		cfg,
		consumertest.NewNop(),
	)
	if err != nil {
		t.Fatalf("unexpected error creating processor: %v", err)
	}
	if p == nil {
		t.Fatal("processor should not be nil")
	}
}
