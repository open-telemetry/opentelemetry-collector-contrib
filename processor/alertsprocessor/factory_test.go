// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsprocessor

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

func nopSettings() processor.Settings {
	s := processor.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
	}
	// Some older versions return a zero logger; ensure it's set.
	if s.TelemetrySettings.Logger == nil {
		s.TelemetrySettings.Logger = zap.NewNop()
	}
	return s
}

func TestFactory_TypeAndDefaultConfig(t *testing.T) {
	f := NewFactory()

	if got := f.Type().String(); got != typeStr {
		t.Fatalf("factory type = %q, want %q", got, typeStr)
	}

	cfg := f.CreateDefaultConfig()
	acfg, ok := cfg.(*Config)
	if !ok {
		t.Fatalf("default config type = %T, want *Config", cfg)
	}

	if err := acfg.Validate(); err != nil {
		t.Fatalf("default config did not validate: %v", err)
	}
}

func TestFactory_CreateMetrics_Success(t *testing.T) {
	ctx := context.Background()
	set := nopSettings()

	cfg := createDefaultConfig().(*Config)
	cfg.Evaluation.Timeout = 50 * time.Millisecond

	// consumertest.NewNop() implements consumer.Metrics/Logs/Traces
	var next consumer.Metrics = consumertest.NewNop()

	p, err := createMetrics(ctx, set, cfg, next)
	if err != nil {
		t.Fatalf("createMetrics returned error: %v", err)
	}
	if p == nil {
		t.Fatal("createMetrics returned nil processor")
	}

	if err := p.Start(ctx, componenttest.NewNopHost()); err != nil {
		t.Fatalf("metrics Start error: %v", err)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("metrics Shutdown error: %v", err)
	}
}

func TestFactory_CreateLogs_Success(t *testing.T) {
	ctx := context.Background()
	set := nopSettings()

	cfg := createDefaultConfig().(*Config)
	cfg.Evaluation.Timeout = 50 * time.Millisecond

	var next consumer.Logs = consumertest.NewNop()

	p, err := createLogs(ctx, set, cfg, next)
	if err != nil {
		t.Fatalf("createLogs returned error: %v", err)
	}
	if p == nil {
		t.Fatal("createLogs returned nil processor")
	}

	if err := p.Start(ctx, componenttest.NewNopHost()); err != nil {
		t.Fatalf("logs Start error: %v", err)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("logs Shutdown error: %v", err)
	}
}

func TestFactory_CreateTraces_Success(t *testing.T) {
	ctx := context.Background()
	set := nopSettings()

	cfg := createDefaultConfig().(*Config)
	cfg.Evaluation.Timeout = 50 * time.Millisecond

	var next consumer.Traces = consumertest.NewNop()

	p, err := createTraces(ctx, set, cfg, next)
	if err != nil {
		t.Fatalf("createTraces returned error: %v", err)
	}
	if p == nil {
		t.Fatal("createTraces returned nil processor")
	}

	if err := p.Start(ctx, componenttest.NewNopHost()); err != nil {
		t.Fatalf("traces Start error: %v", err)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("traces Shutdown error: %v", err)
	}
}

func TestFactory_Create_ErrOnInvalidConfig(t *testing.T) {
	ctx := context.Background()
	set := nopSettings()

	// Make the config invalid.
	cfg := createDefaultConfig().(*Config)
	cfg.SlidingWindow.Duration = 0 // invalid: must be > 0

	if _, err := createMetrics(ctx, set, cfg, consumertest.NewNop()); err == nil {
		t.Error("createMetrics: expected error for invalid config, got nil")
	}
	if _, err := createLogs(ctx, set, cfg, consumertest.NewNop()); err == nil {
		t.Error("createLogs: expected error for invalid config, got nil")
	}
	if _, err := createTraces(ctx, set, cfg, consumertest.NewNop()); err == nil {
		t.Error("createTraces: expected error for invalid config, got nil")
	}
}
