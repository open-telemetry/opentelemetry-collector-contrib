// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// factory_test.go - Factory wiring & creation tests
package isolationforestprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestFactory_Type_And_DefaultConfig(t *testing.T) {
	factory := NewFactory()

	// Ensure the registered type matches what we expect.
	wantType := component.MustNewType("isolationforest")
	assert.Equal(t, wantType, factory.Type())

	// Default config should be non-nil and of our concrete *Config type.
	rawCfg := factory.CreateDefaultConfig()
	require.NotNil(t, rawCfg)
	_, ok := rawCfg.(*Config)
	require.True(t, ok, "CreateDefaultConfig should return *Config")
}

func TestFactory_CreateTraces(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()

	// NewNopSettings() has NO args in current API.
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	next := consumertest.NewNop()
	p, err := factory.CreateTraces(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestFactory_CreateMetrics(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	next := consumertest.NewNop()
	p, err := factory.CreateMetrics(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(t.Context()))
}

func TestFactory_CreateLogs(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	next := consumertest.NewNop()
	p, err := factory.CreateLogs(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(t.Context()))
}

// Additional tests for 100% coverage

func TestFactory_CreateTraces_InvalidConfig(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Test with wrong config type
	invalidCfg := struct{}{}
	_, err := factory.CreateTraces(t.Context(), settings, invalidCfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration is not of type *Config")
}

func TestFactory_CreateMetrics_InvalidConfig(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Test with wrong config type
	invalidCfg := struct{}{}
	_, err := factory.CreateMetrics(t.Context(), settings, invalidCfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration is not of type *Config")
}

func TestFactory_CreateLogs_InvalidConfig(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Test with wrong config type
	invalidCfg := struct{}{}
	_, err := factory.CreateLogs(t.Context(), settings, invalidCfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration is not of type *Config")
}

func TestFactory_CreateTraces_ValidationError(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create invalid config that will fail validation
	cfg := &Config{
		ForestSize: -1, // Invalid forest size
	}

	_, err := factory.CreateTraces(t.Context(), settings, cfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestFactory_CreateMetrics_ValidationError(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create invalid config that will fail validation
	cfg := &Config{
		ForestSize: -1, // Invalid forest size
	}

	_, err := factory.CreateMetrics(t.Context(), settings, cfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestFactory_CreateLogs_ValidationError(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create invalid config that will fail validation
	cfg := &Config{
		ForestSize: -1, // Invalid forest size
	}

	_, err := factory.CreateLogs(t.Context(), settings, cfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

// Fixed tests - check for "invalid configuration" instead of "failed to create processor"
// since validation happens before processor creation
func TestFactory_CreateTraces_ConfigValidationError(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create config with invalid update frequency to trigger validation error
	cfg := &Config{
		ForestSize:              10,
		SubsampleSize:           32,
		ContaminationRate:       0.1,
		Threshold:               0.5,
		MinSamples:              1,
		Mode:                    "enrich",
		ScoreAttribute:          "anomaly.score",
		ClassificationAttribute: "anomaly.is_anomaly",
		TrainingWindow:          "1h",
		UpdateFrequency:         "invalid-duration", // This will cause validation to fail
		Performance:             PerformanceConfig{BatchSize: 64},
		Features: FeatureConfig{
			Traces: []string{"duration"},
		},
	}

	_, err := factory.CreateTraces(t.Context(), settings, cfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestFactory_CreateMetrics_ConfigValidationError(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create config with invalid update frequency to trigger validation error
	cfg := &Config{
		ForestSize:              10,
		SubsampleSize:           32,
		ContaminationRate:       0.1,
		Threshold:               0.5,
		MinSamples:              1,
		Mode:                    "enrich",
		ScoreAttribute:          "anomaly.score",
		ClassificationAttribute: "anomaly.is_anomaly",
		TrainingWindow:          "1h",
		UpdateFrequency:         "invalid-duration", // This will cause validation to fail
		Performance:             PerformanceConfig{BatchSize: 64},
		Features: FeatureConfig{
			Metrics: []string{"value"},
		},
	}

	_, err := factory.CreateMetrics(t.Context(), settings, cfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestFactory_CreateLogs_ConfigValidationError(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create config with invalid update frequency to trigger validation error
	cfg := &Config{
		ForestSize:              10,
		SubsampleSize:           32,
		ContaminationRate:       0.1,
		Threshold:               0.5,
		MinSamples:              1,
		Mode:                    "enrich",
		ScoreAttribute:          "anomaly.score",
		ClassificationAttribute: "anomaly.is_anomaly",
		TrainingWindow:          "1h",
		UpdateFrequency:         "invalid-duration", // This will cause validation to fail
		Performance:             PerformanceConfig{BatchSize: 64},
		Features: FeatureConfig{
			Logs: []string{"severity_number"},
		},
	}

	_, err := factory.CreateLogs(t.Context(), settings, cfg, next)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestTracesProcessor_ConsumeTraces(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	p, err := factory.CreateTraces(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))

	// Test ConsumeTraces
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	err = p.ConsumeTraces(t.Context(), traces)
	assert.NoError(t, err)

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestMetricsProcessor_ConsumeMetrics(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	p, err := factory.CreateMetrics(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))

	// Test ConsumeMetrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test-metric")
	dps := m.SetEmptyGauge().DataPoints()
	dp := dps.AppendEmpty()
	dp.SetDoubleValue(42.0)

	err = p.ConsumeMetrics(t.Context(), metrics)
	assert.NoError(t, err)

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestLogsProcessor_ConsumeLogs(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	p, err := factory.CreateLogs(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))

	// Test ConsumeLogs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("test log message")

	err = p.ConsumeLogs(t.Context(), logs)
	assert.NoError(t, err)

	require.NoError(t, p.Shutdown(t.Context()))
}

func TestProcessorCapabilities(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Test traces processor capabilities
	tp, err := factory.CreateTraces(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	caps := tp.Capabilities()
	assert.True(t, caps.MutatesData, "Traces processor should mutate data")

	// Test metrics processor capabilities
	mp, err := factory.CreateMetrics(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	caps = mp.Capabilities()
	assert.True(t, caps.MutatesData, "Metrics processor should mutate data")

	// Test logs processor capabilities
	lp, err := factory.CreateLogs(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	caps = lp.Capabilities()
	assert.True(t, caps.MutatesData, "Logs processor should mutate data")
}

func TestProcessorConsumerErrors(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	// Test with error consumer for traces
	errorConsumer := consumertest.NewErr(assert.AnError)
	tp, err := factory.CreateTraces(t.Context(), settings, rawCfg, errorConsumer)
	require.NoError(t, err)
	require.NoError(t, tp.Start(t.Context(), componenttest.NewNopHost()))

	traces := ptrace.NewTraces()
	err = tp.ConsumeTraces(t.Context(), traces)
	assert.Error(t, err)

	require.NoError(t, tp.Shutdown(t.Context()))

	// Test with error consumer for metrics
	mp, err := factory.CreateMetrics(t.Context(), settings, rawCfg, errorConsumer)
	require.NoError(t, err)
	require.NoError(t, mp.Start(t.Context(), componenttest.NewNopHost()))

	metrics := pmetric.NewMetrics()
	err = mp.ConsumeMetrics(t.Context(), metrics)
	assert.Error(t, err)

	require.NoError(t, mp.Shutdown(t.Context()))

	// Test with error consumer for logs
	lp, err := factory.CreateLogs(t.Context(), settings, rawCfg, errorConsumer)
	require.NoError(t, err)
	require.NoError(t, lp.Start(t.Context(), componenttest.NewNopHost()))

	logs := plog.NewLogs()
	err = lp.ConsumeLogs(t.Context(), logs)
	assert.Error(t, err)

	require.NoError(t, lp.Shutdown(t.Context()))
}

// Additional tests to reach 100% coverage
func TestProcessorErrorPropagation(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))
	next := consumertest.NewNop()

	// Create traces processor and test error scenarios
	tp, err := factory.CreateTraces(t.Context(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NoError(t, tp.Start(t.Context(), componenttest.NewNopHost()))

	// Test with context cancellation
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	traces := ptrace.NewTraces()
	err = tp.ConsumeTraces(ctx, traces)
	assert.Error(t, err)

	require.NoError(t, tp.Shutdown(t.Context()))
}

func TestFactoryStability(t *testing.T) {
	// Verify factory stability level and type constants
	assert.Equal(t, component.StabilityLevelAlpha, stability)
	assert.Equal(t, "isolationforest", typeStr)
}
