// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

type mockExporter struct {
	rms []*metricdata.ResourceMetrics
}

func (m *mockExporter) Temporality(_ sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.DeltaTemporality
}

func (m *mockExporter) Aggregation(_ sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.AggregationDefault{}
}

func (m *mockExporter) Export(_ context.Context, metrics *metricdata.ResourceMetrics) error {
	m.rms = append(m.rms, metrics)
	return nil
}

func (m *mockExporter) ForceFlush(_ context.Context) error {
	return nil
}

func (m *mockExporter) Shutdown(_ context.Context) error {
	return nil
}

func TestFixedNumberOfMetrics(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumMetrics: 5,
		MetricType: metricTypeSum,
	}

	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.rms, 5)
}

func TestRateOfMetrics(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
		MetricType: metricTypeSum,
	}

	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// test
	require.NoError(t, Run(cfg, expFunc, zap.NewNop()))

	// verify
	// the minimum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(m.rms) >= 6, "there should have been more than 6 metrics, had %d", len(m.rms))
	// // the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(m.rms) <= 20, "there should have been less than 20 metrics, had %d", len(m.rms))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
		MetricType: metricTypeSum,
	}

	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	assert.True(t, len(m.rms) > 100, "there should have been more than 100 metrics, had %d", len(m.rms))
}
