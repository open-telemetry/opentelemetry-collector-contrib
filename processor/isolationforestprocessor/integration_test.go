// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// integration_test.go - Integration-style tests through the factory
package isolationforestprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func makeTestTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "frontend")
	ss := rs.ScopeSpans().AppendEmpty()

	span := ss.Spans().AppendEmpty()
	span.SetName("GET /api")

	start := time.Now()
	end := start.Add(100 * time.Millisecond)

	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
	span.Attributes().PutInt("http.status_code", 200)
	return td
}

func makeTestLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "frontend")
	sl := rl.ScopeLogs().AppendEmpty()

	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	lr.SetSeverityNumber(plog.SeverityNumberInfo)
	lr.Body().SetStr("hello world")
	return ld
}

func makeTestMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "frontend")
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("requests_total")
	m.SetEmptySum().DataPoints().AppendEmpty().SetDoubleValue(1)
	return md
}

func baseConfigEnrich(t *testing.T) *Config {
	factory := NewFactory()
	raw := factory.CreateDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok)

	cfg.Mode = "enrich"
	cfg.ScoreAttribute = "anomaly.score"
	cfg.ClassificationAttribute = "anomaly.is_anomaly"
	cfg.ForestSize = 10
	cfg.SubsampleSize = 32
	cfg.ContaminationRate = 0.1
	cfg.Threshold = 0.5
	cfg.MinSamples = 1
	cfg.TrainingWindow = "1h"
	cfg.UpdateFrequency = "5m"
	cfg.Performance.BatchSize = 64
	cfg.Features = FeatureConfig{
		Traces:  []string{"duration", "error", "operation.name", "service.name"},
		Metrics: []string{"value"},
		Logs:    []string{"severity_number", "message_length"},
	}
	require.NoError(t, cfg.Validate())
	return cfg
}

func TestTraces_Enrich_AddsAttributes(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory()
	cfg := baseConfigEnrich(t)

	trSink := new(consumertest.TracesSink)
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	p, err := factory.CreateTraces(ctx, settings, cfg, trSink)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() { _ = p.Shutdown(ctx) }()

	td := makeTestTrace()
	require.NoError(t, p.ConsumeTraces(ctx, td))

	got := trSink.AllTraces()
	require.NotEmpty(t, got)

	rs := got[0].ResourceSpans().At(0)
	ss := rs.ScopeSpans().At(0)
	sp := ss.Spans().At(0)
	attrs := sp.Attributes()

	_, ok := attrs.Get(cfg.ScoreAttribute)
	assert.True(t, ok, "expected score attribute on span")
	_, ok = attrs.Get(cfg.ClassificationAttribute)
	assert.True(t, ok, "expected classification attribute on span")
}

func TestLogs_Enrich_AddsAttributes(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory()
	cfg := baseConfigEnrich(t)

	logSink := new(consumertest.LogsSink)
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	p, err := factory.CreateLogs(ctx, settings, cfg, logSink)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() { _ = p.Shutdown(ctx) }()

	ld := makeTestLogs()
	require.NoError(t, p.ConsumeLogs(ctx, ld))

	got := logSink.AllLogs()
	require.NotEmpty(t, got)

	rl := got[0].ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)
	attrs := lr.Attributes()

	_, ok := attrs.Get(cfg.ScoreAttribute)
	assert.True(t, ok, "expected score attribute on log record")
	_, ok = attrs.Get(cfg.ClassificationAttribute)
	assert.True(t, ok, "expected classification attribute on log record")
}

func TestMetrics_Enrich_AddsAttributes(t *testing.T) {
	ctx := context.Background()
	factory := NewFactory()
	cfg := baseConfigEnrich(t)

	metSink := new(consumertest.MetricsSink)
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	p, err := factory.CreateMetrics(ctx, settings, cfg, metSink)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() { _ = p.Shutdown(ctx) }()

	md := makeTestMetrics()
	require.NoError(t, p.ConsumeMetrics(ctx, md))

	got := metSink.AllMetrics()
	require.NotEmpty(t, got)

	rm := got[0].ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	m := sm.Metrics().At(0)

	switch m.Type() {
	case pmetric.MetricTypeSum:
		dps := m.Sum().DataPoints()
		require.True(t, dps.Len() > 0)
		attrs := dps.At(0).Attributes()

		_, ok := attrs.Get(cfg.ScoreAttribute)
		assert.True(t, ok, "expected score attribute on metric datapoint")
		_, ok = attrs.Get(cfg.ClassificationAttribute)
		assert.True(t, ok, "expected classification attribute on metric datapoint")
	default:
		t.Fatalf("unexpected metric type in test: %v", m.Type())
	}
}
