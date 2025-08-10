// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// processor_test.go - Tests for the isolationForestProcessor behavior
package isolationforestprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func baseTestConfig(t *testing.T) *Config {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig must return *Config")

	// Make the defaults deterministic and small for tests
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

func makeTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "frontend")
	ss := rs.ScopeSpans().AppendEmpty()

	sp := ss.Spans().AppendEmpty()
	sp.SetName("GET /health")
	start := time.Now()
	end := start.Add(75 * time.Millisecond)
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
	sp.Attributes().PutInt("http.status_code", 200)
	return td
}

func makeLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "frontend")
	sl := rl.ScopeLogs().AppendEmpty()

	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	lr.SetSeverityNumber(plog.SeverityNumberInfo)
	lr.Body().SetStr("hello")
	return ld
}

func makeMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "frontend")
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("requests_total")
	dps := m.SetEmptySum().DataPoints()
	dp := dps.AppendEmpty()
	dp.SetDoubleValue(1)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return md
}

func Test_newIsolationForestProcessor_Basic(t *testing.T) {
	cfg := baseTestConfig(t)
	logger := zaptest.NewLogger(t)

	p, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, p)

	// Sanity: single-model by default unless models configured
	assert.False(t, cfg.IsMultiModelMode())
}

func Test_processFeatures_SaneOutputs(t *testing.T) {
	cfg := baseTestConfig(t)
	logger := zaptest.NewLogger(t)

	p, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)

	features := map[string][]float64{
		"duration": {50.0},
	}
	attrs := map[string]interface{}{
		"service.name": "frontend",
	}

	score, isAnomaly, model := p.processFeatures(features, attrs)
	assert.True(t, score >= 0.0 && score <= 1.0, "score must be in [0,1]")
	// We don't assert on isAnomaly (model behavior dependent), but ensure it's a boolean by using it
	if isAnomaly {
		assert.NotEmpty(t, model) // if anomalous, a model name should exist (default or specific)
	} else {
		// ok even if not anomalous
	}
}

func Test_processTraces_EnrichesAttributes(t *testing.T) {
	cfg := baseTestConfig(t)
	logger := zaptest.NewLogger(t)

	p, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)

	tdIn := makeTrace()
	tdOut, err := p.processTraces(context.Background(), tdIn)
	require.NoError(t, err)

	rs := tdOut.ResourceSpans().At(0)
	ss := rs.ScopeSpans().At(0)
	sp := ss.Spans().At(0)

	_, ok := sp.Attributes().Get(cfg.ScoreAttribute)
	assert.True(t, ok, "expected score attribute on span")
	_, ok = sp.Attributes().Get(cfg.ClassificationAttribute)
	assert.True(t, ok, "expected classification attribute on span")
}

func Test_processLogs_EnrichesAttributes(t *testing.T) {
	cfg := baseTestConfig(t)
	logger := zaptest.NewLogger(t)

	p, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)

	ldIn := makeLogs()
	ldOut, err := p.processLogs(context.Background(), ldIn)
	require.NoError(t, err)

	rl := ldOut.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	lr := sl.LogRecords().At(0)

	_, ok := lr.Attributes().Get(cfg.ScoreAttribute)
	assert.True(t, ok, "expected score attribute on log record")
	_, ok = lr.Attributes().Get(cfg.ClassificationAttribute)
	assert.True(t, ok, "expected classification attribute on log record")
}

func Test_processMetrics_EnrichesAttributes(t *testing.T) {
	cfg := baseTestConfig(t)
	logger := zaptest.NewLogger(t)

	p, err := newIsolationForestProcessor(cfg, logger)
	require.NoError(t, err)

	mdIn := makeMetrics()
	mdOut, err := p.processMetrics(context.Background(), mdIn)
	require.NoError(t, err)

	rm := mdOut.ResourceMetrics().At(0)
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
		t.Fatalf("unexpected metric type: %v", m.Type())
	}
}
