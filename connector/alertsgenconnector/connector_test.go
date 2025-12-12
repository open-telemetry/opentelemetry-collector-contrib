// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/state"
)

func TestConnectorLifecycle(t *testing.T) {
	ctx := context.Background()
	cfg := createTestConfig()
	cfg.WindowSize = 100 * time.Millisecond
	cfg.Step = 50 * time.Millisecond

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Set downstream consumers
	conn.nextLogs = consumertest.NewNop()
	conn.nextMetrics = consumertest.NewNop()

	// Start connector
	err = conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Let it run for a bit
	time.Sleep(200 * time.Millisecond)

	// Shutdown
	err = conn.Shutdown(ctx)
	require.NoError(t, err)
}

func TestConnectorConsumeTraces(t *testing.T) {
	ctx := context.Background()
	cfg := createTestConfig()
	cfg.WindowSize = 100 * time.Millisecond
	cfg.Rules = []RuleCfg{
		{
			Name:     "high_latency",
			Signal:   "traces",
			Enabled:  true,
			Severity: "critical",
			Window:   100 * time.Millisecond,
			Step:     100 * time.Millisecond,
			For:      0,
			Select: map[string]string{
				"service.name": ".*",
			},
			GroupBy: []string{"service.name"},
			Expr: ExprCfg{
				Type:     "avg_over_time",
				Field:    "duration_ns",
				Op:       ">",
				Value:    1000000, // 1ms
				Quantile: 0,
			},
		},
	}

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	// Capture generated logs
	logsSink := &consumertest.LogsSink{}
	conn.nextLogs = logsSink

	// Start connector
	err = conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer conn.Shutdown(ctx)

	// Create test traces
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetStartTimestamp(100)
	span.SetEndTimestamp(200)
	span.Attributes().PutStr("service.name", "test-service")

	// Consume traces
	err = conn.ConsumeTraces(ctx, td)
	require.NoError(t, err)

	// Wait for evaluation
	time.Sleep(150 * time.Millisecond)

	assert.NotNil(t, conn.ing)
}

func TestConnectorConsumeLogs(t *testing.T) {
	ctx := context.Background()
	cfg := createTestConfig()
	cfg.WindowSize = 100 * time.Millisecond
	cfg.Rules = []RuleCfg{
		{
			Name:     "error_logs",
			Signal:   "logs",
			Enabled:  true,
			Severity: "warning",
			Window:   100 * time.Millisecond,
			Step:     100 * time.Millisecond,
			For:      0,
			Select: map[string]string{
				"severity": "ERROR|FATAL",
			},
			GroupBy: []string{"service.name"},
			Expr: ExprCfg{
				Type:  "count_over_time",
				Field: "value",
				Op:    ">",
				Value: 5,
			},
		},
	}

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	logsSink := &consumertest.LogsSink{}
	conn.nextLogs = logsSink

	err = conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer conn.Shutdown(ctx)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	for i := 0; i < 10; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetSeverityText("ERROR")
		lr.Attributes().PutStr("service.name", "test-service")
		lr.Attributes().PutStr("severity", "ERROR")
		lr.Body().SetStr("Error message")
	}

	err = conn.ConsumeLogs(ctx, ld)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}

func TestConnectorConsumeMetrics(t *testing.T) {
	ctx := context.Background()
	cfg := createTestConfig()
	cfg.WindowSize = 100 * time.Millisecond
	cfg.Rules = []RuleCfg{
		{
			Name:     "high_cpu",
			Signal:   "metrics",
			Enabled:  true,
			Severity: "warning",
			Window:   100 * time.Millisecond,
			Step:     100 * time.Millisecond,
			For:      0,
			Select: map[string]string{
				"__name__": "system.cpu.utilization",
			},
			GroupBy: []string{"host.name"},
			Expr: ExprCfg{
				Type:  "avg_over_time",
				Field: "value",
				Op:    ">",
				Value: 0.8,
			},
		},
	}

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	metricsSink := &consumertest.MetricsSink{}
	conn.nextMetrics = metricsSink

	err = conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer conn.Shutdown(ctx)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("system.cpu.utilization")
	m.SetEmptyGauge()
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(0.9)
	dp.Attributes().PutStr("host.name", "test-host")
	dp.Attributes().PutStr("__name__", "system.cpu.utilization")

	err = conn.ConsumeMetrics(ctx, md)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}

func TestConnectorEvaluation(t *testing.T) {
	ctx := context.Background()
	cfg := createTestConfig()
	cfg.WindowSize = 50 * time.Millisecond
	cfg.Step = 50 * time.Millisecond

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	conn.ing = &ingester{
		cfg:     cfg,
		logger:  set.Logger,
		traces:  NewSliceBuffer(100, 1024),
		logs:    NewSliceBuffer(100, 512),
		metrics: NewSliceBuffer(100, 768),
	}

	conn.ing.traces.Add(traceRow{
		durationNs: 5000000,
		ts:         time.Now(),
		attrs:      map[string]string{"service.name": "test"},
	})

	conn.evaluateOnce(time.Now())
	assert.NotNil(t, conn.rs)
}

func TestConnectorBatchFlush(t *testing.T) {
	cfg := createTestConfig()
	cfg.TSDB = &TSDBConfig{
		QueryURL:                 "http://test",
		RemoteWriteURL:           "http://test/write",
		EnableRemoteWrite:        true,
		RemoteWriteBatchSize:     10,
		RemoteWriteFlushInterval: 100 * time.Millisecond,
	}

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	ctx := context.Background()
	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	conn.tsdb = &state.TSDBSyncer{}

	events := []state.AlertEvent{
		{
			Rule:     "test_rule",
			State:    "firing",
			Severity: "warning",
			Labels:   map[string]string{"test": "label"},
			Value:    1.0,
		},
	}
	conn.addEventsToBatch(events)

	assert.Equal(t, 1, len(conn.eventBatch))

	conn.flushEventBatch()
	assert.Equal(t, 0, len(conn.eventBatch))
}

func TestConnectorMemoryReporting(t *testing.T) {
	cfg := createTestConfig()
	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	ctx := context.Background()
	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	conn.reportMemoryUsage()
}

func TestConnectorWithTSDBRestore(t *testing.T) {
	cfg := createTestConfig()
	cfg.TSDB = &TSDBConfig{
		QueryURL:     "http://prometheus:9090",
		QueryTimeout: 5 * time.Second,
	}

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	ctx := context.Background()
	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	// Use conn so it isn't "declared and not used"
	err = conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	err = conn.Shutdown(ctx)
	require.NoError(t, err)
}

func TestConnectorConcurrency(t *testing.T) {
	ctx := context.Background()
	cfg := createTestConfig()
	cfg.WindowSize = 100 * time.Millisecond
	cfg.Step = 50 * time.Millisecond

	set := connectortest.NewNopSettings(component.MustNewType("alertsgen"))
	set.Logger = zaptest.NewLogger(t)

	conn, err := newAlertsConnector(ctx, set, cfg)
	require.NoError(t, err)

	conn.nextLogs = consumertest.NewNop()
	conn.nextMetrics = consumertest.NewNop()
	conn.nextTraces = consumertest.NewNop()

	err = conn.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	defer conn.Shutdown(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			td := ptrace.NewTraces()
			conn.ConsumeTraces(ctx, td)
		}()

		go func() {
			defer wg.Done()
			ld := plog.NewLogs()
			conn.ConsumeLogs(ctx, ld)
		}()

		go func() {
			defer wg.Done()
			md := pmetric.NewMetrics()
			conn.ConsumeMetrics(ctx, md)
		}()
	}

	wg.Wait()
}

// Helper functions

func createTestConfig() *Config {
	cfg := CreateDefaultConfig().(*Config)
	cfg.WindowSize = 5 * time.Second
	cfg.Step = 5 * time.Second
	cfg.InstanceID = "test-instance"

	cfg.TSDB = &TSDBConfig{
		QueryURL:     "http://test",
		QueryTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		DedupWindow:  30 * time.Second,
	}

	return cfg
}
