// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"testing"
	"time"

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
	return config.Supervisor{
		Server: config.OpAMPServer{
			Endpoint: "ws://localhost:1234",
			TLS:      configtls.ClientConfig{Insecure: true},
		},
		Storage: config.Storage{
			Directory: t.TempDir(),
		},
		Agent: config.Agent{
			Executable:              execPath,
			OrphanDetectionInterval: time.Second,
			ConfigApplyTimeout:      time.Second,
			BootstrapTimeout:        time.Second,
		},
	}
}

func TestSupervisorMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(t.Context()) }()

	cfg := newTestSupervisorConfig(t)
	supervisor, err := NewSupervisor(zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	supervisor.telemetrySettings.MeterProvider = mp
	metrics, err := supervisorTelemetry.NewMetrics(mp)
	require.NoError(t, err)
	supervisor.metrics = metrics

	supervisor.metrics.SetCollectorHealthStatus(t.Context(), true)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &rm)
	require.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)
	sm := rm.ScopeMetrics[0]
	require.Len(t, sm.Metrics, 1)

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

	supervisor.Shutdown()
}

func TestSupervisorMetricsLifecycle(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer func() { _ = mp.Shutdown(t.Context()) }()

	cfg := newTestSupervisorConfig(t)
	supervisor, err := NewSupervisor(zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	supervisor.telemetrySettings.MeterProvider = mp
	metrics, err := supervisorTelemetry.NewMetrics(mp)
	require.NoError(t, err)
	supervisor.metrics = metrics

	supervisor.metrics.SetCollectorHealthStatus(t.Context(), true)
	supervisor.metrics.SetCollectorHealthStatus(t.Context(), false)

	var rm metricdata.ResourceMetrics
	err = reader.Collect(t.Context(), &rm)
	require.NoError(t, err)
	require.Len(t, rm.ScopeMetrics, 1)
	sm := rm.ScopeMetrics[0]
	require.Len(t, sm.Metrics, 1)

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

	supervisor.Shutdown()
}
