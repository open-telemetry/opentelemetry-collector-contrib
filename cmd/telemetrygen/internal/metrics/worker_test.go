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
	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumMetrics: 5,
		MetricType: metricTypeSum,
	}

	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, exp.rms, 5)
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
	exp := &mockExporter{}

	// test
	require.NoError(t, Run(cfg, exp, zap.NewNop()))

	// verify
	// the minimum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(exp.rms) >= 6, "there should have been more than 6 metrics, had %d", len(exp.rms))
	// the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.True(t, len(exp.rms) <= 20, "there should have been less than 20 metrics, had %d", len(exp.rms))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
		MetricType: metricTypeSum,
	}
	exp := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	assert.True(t, len(exp.rms) > 100, "there should have been more than 100 metrics, had %d", len(exp.rms))
}

func TestNoTelemetryAttrs(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithNoAttributes(qty)
	exp := &mockExporter{}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, exp.rms, qty)

	rms := exp.rms
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, attr.Len(), 0, "it shouldn't have attrs here")
	}
}

func TestSingleTelemetryAttr(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithOneAttribute(qty)
	exp := &mockExporter{}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, exp.rms, qty)

	rms := exp.rms
	for i := 0; i < qty; i++ {
		ms := rms[i].ScopeMetrics[0].Metrics[0]
		// @note update when telemetrygen allow other metric types
		attr := ms.Data.(metricdata.Gauge[int64]).DataPoints[0].Attributes
		assert.Equal(t, attr.Len(), 1, "it must have a single attribute here")
		actualValue, _ := attr.Value(telemetryAttrKeyOne)
		assert.Equal(t, actualValue.AsString(), telemetryAttrValueOne, "it should be "+telemetryAttrValueOne)
	}
}

func TestMultipleTelemetryAttr(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithMultipleAttributes(qty)
	exp := &mockExporter{}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, Run(cfg, exp, logger))

	time.Sleep(1 * time.Second)

	// asserts
	require.Len(t, exp.rms, qty)

	rms := exp.rms
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

func configWithNoAttributes(qty int) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: nil,
		},
		NumMetrics: qty,
	}
}

func configWithOneAttribute(qty int) *Config {
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne},
		},
		NumMetrics: qty,
	}
}

func configWithMultipleAttributes(qty int) *Config {
	kvs := common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne, telemetryAttrKeyTwo: telemetryAttrValueTwo}
	return &Config{
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: kvs,
		},
		NumMetrics: qty,
	}
}
