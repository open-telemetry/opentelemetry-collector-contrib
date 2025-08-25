// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"context"
	"strings"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// helper: build connector.Settings with no external deps.
func testSettings() connector.Settings {
	return connector.Settings{
		ID:                component.NewID(component.MustNewType(typeStr)),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
}

func TestFactory_DefaultConfig_StartShutdown_OK(t *testing.T) {
	ctx := context.Background()
	set := testSettings()
	host := componenttest.NewNopHost()

	// Default config (TSDB disabled by default).
	cfg := CreateDefaultConfig().(*Config)
	if cfg.TSDB != nil {
		cfg.TSDB.Enabled = false // be explicit to avoid surprises if defaults change
	}

	// traces -> logs
	{
		nextLogs, err := consumer.NewLogs(func(context.Context, plog.Logs) error { return nil })
		if err != nil {
			t.Fatalf("creating next logs consumer failed: %v", err)
		}
		tr, err := createTracesToLogs(ctx, set, cfg, nextLogs)
		if err != nil {
			t.Fatalf("createTracesToLogs (default cfg) returned error: %v", err)
		}
		if err := tr.Start(ctx, host); err != nil {
			t.Fatalf("traces->logs Start failed: %v", err)
		}
		if err := tr.Shutdown(ctx); err != nil {
			t.Fatalf("traces->logs Shutdown failed: %v", err)
		}
	}

	// metrics -> logs
	{
		nextLogs, err := consumer.NewLogs(func(context.Context, plog.Logs) error { return nil })
		if err != nil {
			t.Fatalf("creating next logs consumer failed: %v", err)
		}
		m2l, err := createMetricsToLogs(ctx, set, cfg, nextLogs)
		if err != nil {
			t.Fatalf("createMetricsToLogs (default cfg) returned error: %v", err)
		}
		if err := m2l.Start(ctx, host); err != nil {
			t.Fatalf("metrics->logs Start failed: %v", err)
		}
		if err := m2l.Shutdown(ctx); err != nil {
			t.Fatalf("metrics->logs Shutdown failed: %v", err)
		}
	}

	// metrics -> metrics
	{
		nextMetrics, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error { return nil })
		if err != nil {
			t.Fatalf("creating next metrics consumer failed: %v", err)
		}
		m2m, err := createMetricsToMetrics(ctx, set, cfg, nextMetrics)
		if err != nil {
			t.Fatalf("createMetricsToMetrics (default cfg) returned error: %v", err)
		}
		if err := m2m.Start(ctx, host); err != nil {
			t.Fatalf("metrics->metrics Start failed: %v", err)
		}
		if err := m2m.Shutdown(ctx); err != nil {
			t.Fatalf("metrics->metrics Shutdown failed: %v", err)
		}
	}
}

func TestFactory_TSDBEnabledWithoutURL_Errors(t *testing.T) {
	ctx := context.Background()
	set := testSettings()

	cfg := CreateDefaultConfig().(*Config)
	if cfg.TSDB == nil {
		cfg.TSDB = &TSDBConfig{}
	}
	cfg.TSDB.Enabled = true
	cfg.TSDB.QueryURL = "" // intentionally missing to trigger guard

	wantSubstr := "tsdb.enabled=true"

	// traces -> logs should fail
	{
		nextLogs, err := consumer.NewLogs(func(context.Context, plog.Logs) error { return nil })
		if err != nil {
			t.Fatalf("creating next logs consumer failed: %v", err)
		}
		_, err = createTracesToLogs(ctx, set, cfg, nextLogs)
		if err == nil {
			t.Fatalf("createTracesToLogs expected error when tsdb.enabled=true and query_url missing")
		}
		if !strings.Contains(strings.ToLower(err.Error()), wantSubstr) {
			t.Fatalf("unexpected error: %v (want substring %q)", err, wantSubstr)
		}
	}

	// metrics -> logs should fail
	{
		nextLogs, err := consumer.NewLogs(func(context.Context, plog.Logs) error { return nil })
		if err != nil {
			t.Fatalf("creating next logs consumer failed: %v", err)
		}
		_, err = createMetricsToLogs(ctx, set, cfg, nextLogs)
		if err == nil {
			t.Fatalf("createMetricsToLogs expected error when tsdb.enabled=true and query_url missing")
		}
		if !strings.Contains(strings.ToLower(err.Error()), wantSubstr) {
			t.Fatalf("unexpected error: %v (want substring %q)", err, wantSubstr)
		}
	}

	// metrics -> metrics should fail
	{
		nextMetrics, err := consumer.NewMetrics(func(context.Context, pmetric.Metrics) error { return nil })
		if err != nil {
			t.Fatalf("creating next metrics consumer failed: %v", err)
		}
		_, err = createMetricsToMetrics(ctx, set, cfg, nextMetrics)
		if err == nil {
			t.Fatalf("createMetricsToMetrics expected error when tsdb.enabled=true and query_url missing")
		}
		if !strings.Contains(strings.ToLower(err.Error()), wantSubstr) {
			t.Fatalf("unexpected error: %v (want substring %q)", err, wantSubstr)
		}
	}
}
