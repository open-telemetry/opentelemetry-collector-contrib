// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

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

func checkMetricTemporality(t *testing.T, ms metricdata.Metrics, metricType MetricType, expectedAggregationTemporality metricdata.Temporality) {
	switch metricType {
	case MetricTypeSum:
		sumData, ok := ms.Data.(metricdata.Sum[int64])
		require.True(t, ok, "expected Sum data type")
		assert.Equal(t, expectedAggregationTemporality, sumData.Temporality)
	case MetricTypeHistogram:
		histogramData, ok := ms.Data.(metricdata.Histogram[int64])
		require.True(t, ok, "expected Histogram data type")
		assert.Equal(t, expectedAggregationTemporality, histogramData.Temporality)
	default:
		t.Fatalf("unsupported metric type: %v", metricType)
	}
}

func TestFixedNumberOfMetrics(t *testing.T) {
	// arrange
	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumMetrics: 5,
		MetricType: MetricTypeSum,
	}
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))
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
		MetricType: MetricTypeSum,
	}
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	require.NoError(t, run(cfg, expFunc, zap.NewNop()))

	// assert
	// the minimum acceptable number of metrics for the rate of 10/sec for half a second
	assert.GreaterOrEqual(t, len(m.rms), 6, "there should have been more than 6 metrics, had %d", len(m.rms))
	// the maximum acceptable number of metrics for the rate of 10/sec for half a second
	assert.LessOrEqual(t, len(m.rms), 20, "there should have been less than 20 metrics, had %d", len(m.rms))
}

func TestMetricsWithTemporality(t *testing.T) {
	tests := []struct {
		name                           string
		metricType                     MetricType
		aggregationTemporality         AggregationTemporality
		expectedAggregationTemporality metricdata.Temporality
	}{
		{
			name:                           "Sum: delta temporality",
			metricType:                     MetricTypeSum,
			aggregationTemporality:         AggregationTemporality(metricdata.DeltaTemporality),
			expectedAggregationTemporality: metricdata.DeltaTemporality,
		},
		{
			name:                           "Sum: cumulative temporality",
			metricType:                     MetricTypeSum,
			aggregationTemporality:         AggregationTemporality(metricdata.CumulativeTemporality),
			expectedAggregationTemporality: metricdata.CumulativeTemporality,
		},
		{
			name:                           "Histogram: delta temporality",
			metricType:                     MetricTypeHistogram,
			aggregationTemporality:         AggregationTemporality(metricdata.DeltaTemporality),
			expectedAggregationTemporality: metricdata.DeltaTemporality,
		},
		{
			name:                           "Histogram: cumulative temporality",
			metricType:                     MetricTypeHistogram,
			aggregationTemporality:         AggregationTemporality(metricdata.CumulativeTemporality),
			expectedAggregationTemporality: metricdata.CumulativeTemporality,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			cfg := &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics:             1,
				MetricName:             "test",
				MetricType:             tt.metricType,
				AggregationTemporality: tt.aggregationTemporality,
			}
			m := &mockExporter{}
			expFunc := func() (sdkmetric.Exporter, error) {
				return m, nil
			}

			// act
			logger, _ := zap.NewDevelopment()
			require.NoError(t, run(cfg, expFunc, logger))

			time.Sleep(1 * time.Second)

			// assert
			require.Len(t, m.rms, 1)
			ms := m.rms[0].ScopeMetrics[0].Metrics[0]
			assert.Equal(t, "test", ms.Name)

			checkMetricTemporality(t, ms, tt.metricType, tt.expectedAggregationTemporality)
		})
	}
}

func TestUnthrottled(t *testing.T) {
	// arrange
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
		MetricType: MetricTypeSum,
	}
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	// assert
	assert.Greater(t, len(m.rms), 100, "there should have been more than 100 metrics, had %d", len(m.rms))
}

func TestSumNoTelemetryAttrs(t *testing.T) {
	// arrange
	qty := 2
	cfg := configWithNoAttributes(MetricTypeSum, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

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
	cfg := configWithNoAttributes(MetricTypeGauge, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

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
	cfg := configWithOneAttribute(MetricTypeSum, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

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
	cfg := configWithOneAttribute(MetricTypeGauge, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

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
	cfg := configWithMultipleAttributes(MetricTypeSum, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

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
	cfg := configWithMultipleAttributes(MetricTypeGauge, qty)
	m := &mockExporter{}
	expFunc := func() (sdkmetric.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

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
				MetricType: MetricTypeSum,
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
				MetricType: MetricTypeSum,
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
				MetricType: MetricTypeSum,
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
			require.EqualError(t, run(tt.cfg, expFunc, logger), tt.wantErrMessage)
		})
	}
}

func configWithNoAttributes(metric MetricType, qty int) *Config {
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

func configWithOneAttribute(metric MetricType, qty int) *Config {
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

func configWithMultipleAttributes(metric MetricType, qty int) *Config {
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

func TestTemporalityStartTimes(t *testing.T) {
	tests := []struct {
		name        string
		temporality AggregationTemporality
		checkTimes  func(t *testing.T, firstTime, secondTime time.Time)
	}{
		{
			name:        "Cumulative temporality keeps same start timestamp",
			temporality: AggregationTemporality(metricdata.CumulativeTemporality),
			checkTimes: func(t *testing.T, firstTime, secondTime time.Time) {
				assert.Equal(t, firstTime, secondTime,
					"cumulative metrics should have same start time")
			},
		},
		{
			name:        "Delta temporality has different start timestamps",
			temporality: AggregationTemporality(metricdata.DeltaTemporality),
			checkTimes: func(t *testing.T, firstTime, secondTime time.Time) {
				assert.True(t, secondTime.After(firstTime),
					"delta metrics should have increasing start times")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockExporter{}
			running := &atomic.Bool{}
			running.Store(true)

			wg := &sync.WaitGroup{}
			wg.Add(1)

			w := worker{
				metricName:             "test_metric",
				metricType:             MetricTypeSum,
				aggregationTemporality: tt.temporality,
				numMetrics:             2,
				running:                running,
				limitPerSecond:         rate.Inf,
				logger:                 zap.NewNop(),
				wg:                     wg,
			}

			w.simulateMetrics(resource.Default(), m, nil)

			wg.Wait()

			require.GreaterOrEqual(t, len(m.rms), 2, "should have at least 2 metric points")

			firstMetric := m.rms[0].ScopeMetrics[0].Metrics[0]
			secondMetric := m.rms[1].ScopeMetrics[0].Metrics[0]

			firstStartTime := firstMetric.Data.(metricdata.Sum[int64]).DataPoints[0].StartTime
			secondStartTime := secondMetric.Data.(metricdata.Sum[int64]).DataPoints[0].StartTime

			tt.checkTimes(t, firstStartTime, secondStartTime)
		})
	}
}
