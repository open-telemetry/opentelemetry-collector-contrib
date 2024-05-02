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
	assert.True(t, len(m.rms) >= 6, "there should have been more than 6 metrics, had %d", len(m.rms))
	// the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(m.rms) <= 20, "there should have been less than 20 metrics, had %d", len(m.rms))
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
	assert.True(t, len(m.rms) > 100, "there should have been more than 100 metrics, had %d", len(m.rms))
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
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Sum[int64]).DataPoints[0].Attributes
		assert.Equal(t, attr.Len(), 0, "it shouldn't have attrs here")
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
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, attr.Len(), 0, "it shouldn't have attrs here")
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
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Sum[int64]).DataPoints[0].Attributes
		assert.Equal(t, attr.Len(), 1, "it must have a single attribute here")
		actualValue, _ := attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, actualValue.AsString(), telemetryAttrValueOne, "it should be "+telemetryAttrValueOne)
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
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, attr.Len(), 1, "it must have a single attribute here")
		actualValue, _ := attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, actualValue.AsString(), telemetryAttrValueOne, "it should be "+telemetryAttrValueOne)
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

func configWithNoAttributes(metric metricType, qty int) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: nil,
		},
		NumMetrics: qty,
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
