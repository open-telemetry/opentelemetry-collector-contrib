// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	supervisorTelemetry "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
)

func newTestAgentExecutable(t *testing.T) string {
	t.Helper()
	tempFile, err := os.CreateTemp(t.TempDir(), "test_binary")
	require.NoError(t, err)
	tempFile.Close()
	return tempFile.Name()
}

func newTestSupervisorConfig(t *testing.T) config.Supervisor {
	execPath := newTestAgentExecutable(t)
	cfg := config.DefaultSupervisor()
	cfg.Server = config.OpAMPServer{
		Endpoint: "ws://localhost:1234",
		TLS:      configtls.ClientConfig{Insecure: true},
	}
	cfg.Storage = config.Storage{Directory: t.TempDir()}
	cfg.Agent = config.Agent{
		Executable:              execPath,
		OrphanDetectionInterval: time.Second,
		ConfigApplyTimeout:      time.Second,
		BootstrapTimeout:        time.Second,
	}
	return cfg
}

func newTestSupervisorWithStartupFallbackConfig(t *testing.T, metrics *supervisorTelemetry.Metrics) *Supervisor {
	t.Helper()

	fallbackConfigPath := filepath.Join(t.TempDir(), "fallback_config.yaml")
	require.NoError(t, os.WriteFile(fallbackConfigPath, []byte(`
receivers:
  nop: null

exporters:
  nop: null

service:
  pipelines:
    logs:
      receivers: [nop]
      exporters: [nop]
`), 0o600))

	supervisor := &Supervisor{
		runCtx:       t.Context(),
		pidProvider:  staticPIDProvider(1234),
		metrics:      metrics,
		cfgState:     &atomic.Value{},
		featureGates: map[string]struct{}{},
		hasNewConfig: make(chan struct{}, 1),
		persistentState: &persistentState{
			InstanceID: uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb"),
		},
		agentDescription:               &atomic.Value{},
		agentConfigOwnTelemetrySection: &atomic.Value{},
		telemetrySettings:              newNopTelemetrySettings(),
		config: config.Supervisor{
			Storage: config.Storage{
				Directory: t.TempDir(),
			},
			Agent: config.Agent{
				OrphanDetectionInterval: time.Second,
				StartupFallbackConfigs:  []string{fallbackConfigPath},
			},
		},
	}

	supervisor.agentDescription.Store(&protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			{
				Key: "service.name",
				Value: &protobufs.AnyValue{
					Value: &protobufs.AnyValue_StringValue{
						StringValue: "otelcol",
					},
				},
			},
		},
	})
	require.NoError(t, supervisor.createTemplates())

	return supervisor
}

func requireFallbackMetricValue(t *testing.T, reader *metric.ManualReader, value int64) {
	t.Helper()

	var rm metricdata.ResourceMetrics
	err := reader.Collect(t.Context(), &rm)
	require.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)
	sm := rm.ScopeMetrics[0]
	require.Len(t, sm.Metrics, 2)

	var fallbackMetric metricdata.Metrics
	for _, m := range sm.Metrics {
		if m.Name == supervisorTelemetry.CollectorFallbackStatusMetric {
			fallbackMetric = m
			break
		}
	}

	require.NotEmpty(t, fallbackMetric)
	metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
		DataPoints:  []metricdata.DataPoint[int64]{{Value: value}},
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: false,
	}, fallbackMetric.Data, metricdatatest.IgnoreTimestamp())
}

func TestSupervisorMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(t.Context()) }()

	cfg := newTestSupervisorConfig(t)
	supervisor, err := NewSupervisor(t.Context(), zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	supervisor.runCtx, supervisor.runCtxCancel = context.WithCancel(t.Context())

	supervisor.telemetrySettings.MeterProvider = mp
	metrics, err := supervisorTelemetry.NewMetrics(mp)
	require.NoError(t, err)
	supervisor.metrics = metrics

	supervisor.metrics.SetCollectorHealthStatus(t.Context(), true)
	supervisor.metrics.SetCollectorFallbackStatus(t.Context(), true)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &rm)
	require.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)
	sm := rm.ScopeMetrics[0]
	require.Len(t, sm.Metrics, 2)

	findMetric := func(name string) metricdata.Metrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
		return metricdata.Metrics{}
	}

	healthMetric := findMetric(supervisorTelemetry.CollectorHealthStatusMetric)
	require.NotEmpty(t, healthMetric)
	metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
		DataPoints:  []metricdata.DataPoint[int64]{{Value: 1}},
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: false,
	}, healthMetric.Data, metricdatatest.IgnoreTimestamp())

	fallbackMetric := findMetric(supervisorTelemetry.CollectorFallbackStatusMetric)
	require.NotEmpty(t, fallbackMetric)
	metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
		DataPoints:  []metricdata.DataPoint[int64]{{Value: 1}},
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: false,
	}, fallbackMetric.Data, metricdatatest.IgnoreTimestamp())

	supervisor.Shutdown()
}

func TestSupervisorMetricsLifecycle(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(t.Context()) }()

	cfg := newTestSupervisorConfig(t)
	supervisor, err := NewSupervisor(t.Context(), zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	supervisor.runCtx, supervisor.runCtxCancel = context.WithCancel(t.Context())

	supervisor.telemetrySettings.MeterProvider = mp
	metrics, err := supervisorTelemetry.NewMetrics(mp)
	require.NoError(t, err)
	supervisor.metrics = metrics

	supervisor.metrics.SetCollectorHealthStatus(t.Context(), true)
	supervisor.metrics.SetCollectorHealthStatus(t.Context(), false)
	supervisor.metrics.SetCollectorFallbackStatus(t.Context(), true)
	supervisor.metrics.SetCollectorFallbackStatus(t.Context(), false)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &rm)
	require.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)
	sm := rm.ScopeMetrics[0]
	require.Len(t, sm.Metrics, 2)

	findMetric := func(name string) metricdata.Metrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
		return metricdata.Metrics{}
	}

	healthMetric := findMetric(supervisorTelemetry.CollectorHealthStatusMetric)
	require.NotEmpty(t, healthMetric)
	metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
		DataPoints:  []metricdata.DataPoint[int64]{{Value: 0}},
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: false,
	}, healthMetric.Data, metricdatatest.IgnoreTimestamp())

	fallbackMetric := findMetric(supervisorTelemetry.CollectorFallbackStatusMetric)
	require.NotEmpty(t, fallbackMetric)
	metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
		DataPoints:  []metricdata.DataPoint[int64]{{Value: 0}},
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: false,
	}, fallbackMetric.Data, metricdatatest.IgnoreTimestamp())

	supervisor.Shutdown()
}

func TestSupervisorInitialFallbackConfigSetsFallbackMetric(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(t.Context()) }()

	metrics, err := supervisorTelemetry.NewMetrics(mp)
	require.NoError(t, err)

	supervisor := newTestSupervisorWithStartupFallbackConfig(t, metrics)

	require.NoError(t, supervisor.loadAndWriteInitialMergedConfig())

	requireFallbackMetricValue(t, reader, 1)
}

func TestSupervisorInitialFallbackConfigClearsFallbackMetricOnConnect(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(t.Context()) }()

	metrics, err := supervisorTelemetry.NewMetrics(mp)
	require.NoError(t, err)

	supervisor := newTestSupervisorWithStartupFallbackConfig(t, metrics)
	require.NoError(t, supervisor.loadAndWriteInitialMergedConfig())
	requireFallbackMetricValue(t, reader, 1)

	supervisor.onConnect(t.Context())

	requireFallbackMetricValue(t, reader, 0)
}
