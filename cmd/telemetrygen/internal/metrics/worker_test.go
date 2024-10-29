// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

const (
	telemetryAttrKeyOne   = "key1"
	telemetryAttrKeyTwo   = "key2"
	telemetryAttrValueOne = "value1"
	telemetryAttrValueTwo = "value2"
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
	// arrange
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

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))
	time.Sleep(1 * time.Second)

	// assert
	require.Len(t, m.rms, 5)
}

func TestRateOfMetrics(t *testing.T) {
	// arrange
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

	// act
	require.NoError(t, Run(cfg, expFunc, zap.NewNop()))

	// assert
	// the minimum acceptable number of metrics for the rate of 10/sec for half a second
	assert.GreaterOrEqual(t, len(m.rms), 6, "there should have been more than 6 metrics, had %d", len(m.rms))
	// the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.LessOrEqual(t, len(m.rms), 20, "there should have been less than 20 metrics, had %d", len(m.rms))
}

func TestUnthrottled(t *testing.T) {
	// arrange
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

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	// assert
	assert.Greater(t, len(m.rms), 100, "there should have been more than 100 metrics, had %d", len(m.rms))
}

func TestSumNoTelemetryAttrs(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithNoAttributes(metricTypeSum, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, m.rms, qty)

	rms := m.rms
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		assert.Equal(t, "test", ms.Name)
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Sum[int64]).DataPoints[0].Attributes
		assert.Equal(t, 0, attr.Len(), "it shouldn't have attrs here")
	}
}

func TestGaugeNoTelemetryAttrs(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithNoAttributes(metricTypeGauge, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, m.rms, qty)

	rms := m.rms
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		assert.Equal(t, "test", ms.Name)
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, 0, attr.Len(), "it shouldn't have attrs here")
	}
}

func TestSumSingleTelemetryAttr(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithOneAttribute(metricTypeSum, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, m.rms, qty)

	rms := m.rms
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		assert.Equal(t, "test", ms.Name)
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Sum[int64]).DataPoints[0].Attributes
		assert.Equal(t, 1, attr.Len(), "it must have a single attribute here")
		actualValue, _ := attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, telemetryAttrValueOne, actualValue.AsString(), "it should be "+telemetryAttrValueOne)
	}
}

func TestGaugeSingleTelemetryAttr(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithOneAttribute(metricTypeGauge, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, m.rms, qty)

	rms := m.rms
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		assert.Equal(t, "test", ms.Name)
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, 1, attr.Len(), "it must have a single attribute here")
		actualValue, _ := attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, telemetryAttrValueOne, actualValue.AsString(), "it should be "+telemetryAttrValueOne)
	}
}

func TestSumMultipleTelemetryAttr(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithMultipleAttributes(metricTypeSum, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, m.rms, qty)

	rms := m.rms
	var actualValue attribute.Value
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Sum[int64]).DataPoints[0].Attributes
		assert.Equal(t, 2, attr.Len(), "it must have multiple attributes here")
		actualValue, _ = attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, telemetryAttrValueOne, actualValue.AsString(), "it should be "+telemetryAttrValueOne)
		actualValue, _ = attr.Value(telemetryAttrKeyTwo)
		assert.Equal(t, telemetryAttrValueTwo, actualValue.AsString(), "it should be "+telemetryAttrValueTwo)
	}
}

func TestGaugeMultipleTelemetryAttr(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithMultipleAttributes(metricTypeGauge, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, m.rms, qty)

	rms := m.rms
	var actualValue attribute.Value
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, 2, attr.Len(), "it must have multiple attributes here")
		actualValue, _ = attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, telemetryAttrValueOne, actualValue.AsString(), "it should be "+telemetryAttrValueOne)
		actualValue, _ = attr.Value(telemetryAttrKeyTwo)
		assert.Equal(t, telemetryAttrValueTwo, actualValue.AsString(), "it should be "+telemetryAttrValueTwo)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		wantErrMessage string
	}{
		{
			name: "No duration or NumMetrics",
			cfg: &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				MetricType: metricTypeSum,
				TraceID:    "123",
			},
			wantErrMessage: "either `metrics` or `duration` must be greater than 0",
		},
		{
			name: "TraceID invalid",
			cfg: &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics: 5,
				MetricType: metricTypeSum,
				TraceID:    "123",
			},
			wantErrMessage: "TraceID must be a 32 character hex string, like: 'ae87dadd90e9935a4bc9660628efd569'",
		},
		{
			name: "SpanID invalid",
			cfg: &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics: 5,
				MetricType: metricTypeSum,
				TraceID:    "ae87dadd90e9935a4bc9660628efd569",
				SpanID:     "123",
			},
			wantErrMessage: "SpanID must be a 16 character hex string, like: '5828fa4960140870'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockExporter{}
			expFunc := func() (sdkmetric.Exporter, error) {
				return m, nil
			}
			logger, _ := zap.NewDevelopment()
			require.EqualError(t, Run(tt.cfg, expFunc, logger), tt.wantErrMessage)
		})
	}
}

func configWithNoAttributes(metric metricType, qty int) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: nil,
		},
		NumMetrics: qty,
		MetricName: "test",
		MetricType: metric,
	}
}

func configWithOneAttribute(metric metricType, qty int) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne},
		},
		NumMetrics: qty,
		MetricName: "test",
		MetricType: metric,
	}
}

func configWithMultipleAttributes(metric metricType, qty int) *Config {
	kvs := common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne, telemetryAttrKeyTwo: telemetryAttrValueTwo}
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: kvs,
		},
		NumMetrics: qty,
		MetricType: metric,
	}
}
