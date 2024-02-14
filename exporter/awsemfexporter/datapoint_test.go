// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	aws "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

type metricValueType string

const (
	intValueType    metricValueType = "int"
	doubleValueType metricValueType = "double"
)

const instrLibName = "cloudwatch-otel"

func generateTestGaugeMetric(name string, valueType metricValueType) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Count")
	gaugeMetric := metric.SetEmptyGauge()
	gaugeDatapoint := gaugeMetric.DataPoints().AppendEmpty()
	gaugeDatapoint.Attributes().PutStr("label1", "value1")

	switch valueType {
	case doubleValueType:
		gaugeDatapoint.SetDoubleValue(0.1)
	default:
		gaugeDatapoint.SetIntValue(1)
	}
	return otelMetrics
}

func generateTestGaugeMetricNaN(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Count")
	gaugeMetric := metric.SetEmptyGauge()
	gaugeDatapoint := gaugeMetric.DataPoints().AppendEmpty()
	gaugeDatapoint.Attributes().PutStr("label1", "value1")

	gaugeDatapoint.SetDoubleValue(math.NaN())

	return otelMetrics
}

func generateTestGaugeMetricInf(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Count")
	gaugeMetric := metric.SetEmptyGauge()
	gaugeDatapoint := gaugeMetric.DataPoints().AppendEmpty()
	gaugeDatapoint.Attributes().PutStr("label1", "value1")

	gaugeDatapoint.SetDoubleValue(math.Inf(0))

	return otelMetrics
}

func generateTestSumMetric(name string, valueType metricValueType) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	for i := 0; i < 2; i++ {
		metric := metrics.AppendEmpty()
		metric.SetName(name)
		metric.SetUnit("Count")
		sumMetric := metric.SetEmptySum()

		sumMetric.SetIsMonotonic(true)
		sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		sumDatapoint := sumMetric.DataPoints().AppendEmpty()
		sumDatapoint.Attributes().PutStr("label1", "value1")
		switch valueType {
		case doubleValueType:
			sumDatapoint.SetDoubleValue(float64(i) * 0.1)
		default:
			sumDatapoint.SetIntValue(int64(i))
		}
	}

	return otelMetrics
}

func generateTestHistogramMetric(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	histogramMetric := metric.SetEmptyHistogram()
	histogramDatapoint := histogramMetric.DataPoints().AppendEmpty()
	histogramDatapoint.BucketCounts().FromRaw([]uint64{5, 6, 7})
	histogramDatapoint.ExplicitBounds().FromRaw([]float64{0, 10})
	histogramDatapoint.Attributes().PutStr("label1", "value1")
	histogramDatapoint.SetCount(18)
	histogramDatapoint.SetSum(35.0)
	return otelMetrics
}

func generateTestHistogramMetricWithNaNs(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	histogramMetric := metric.SetEmptyHistogram()
	histogramDatapoint := histogramMetric.DataPoints().AppendEmpty()
	histogramDatapoint.BucketCounts().FromRaw([]uint64{5, 6, 7})
	histogramDatapoint.ExplicitBounds().FromRaw([]float64{0, math.NaN()})
	histogramDatapoint.Attributes().PutStr("label1", "value1")
	histogramDatapoint.SetCount(18)
	histogramDatapoint.SetSum(math.NaN())
	return otelMetrics
}

func generateTestHistogramMetricWithInfs(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	histogramMetric := metric.SetEmptyHistogram()
	histogramDatapoint := histogramMetric.DataPoints().AppendEmpty()
	histogramDatapoint.BucketCounts().FromRaw([]uint64{5, 6, 7})
	histogramDatapoint.ExplicitBounds().FromRaw([]float64{0, math.Inf(0)})
	histogramDatapoint.Attributes().PutStr("label1", "value1")
	histogramDatapoint.SetCount(18)
	histogramDatapoint.SetSum(math.Inf(0))
	return otelMetrics
}

func generateTestExponentialHistogramMetric(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	exponentialHistogramMetric := metric.SetEmptyExponentialHistogram()

	exponentialHistogramDatapoint := exponentialHistogramMetric.DataPoints().AppendEmpty()
	exponentialHistogramDatapoint.SetCount(4)
	exponentialHistogramDatapoint.SetSum(0)
	exponentialHistogramDatapoint.SetMin(-4)
	exponentialHistogramDatapoint.SetMax(4)
	exponentialHistogramDatapoint.SetZeroCount(0)
	exponentialHistogramDatapoint.SetScale(1)
	exponentialHistogramDatapoint.Positive().SetOffset(1)
	exponentialHistogramDatapoint.Positive().BucketCounts().FromRaw([]uint64{
		1, 0, 1,
	})
	exponentialHistogramDatapoint.Negative().SetOffset(1)
	exponentialHistogramDatapoint.Negative().BucketCounts().FromRaw([]uint64{
		1, 0, 1,
	})
	exponentialHistogramDatapoint.Attributes().PutStr("label1", "value1")
	return otelMetrics
}

func generateTestExponentialHistogramMetricWithNaNs(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	exponentialHistogramMetric := metric.SetEmptyExponentialHistogram()

	exponentialHistogramDatapoint := exponentialHistogramMetric.DataPoints().AppendEmpty()
	exponentialHistogramDatapoint.SetCount(4)
	exponentialHistogramDatapoint.SetSum(math.NaN())
	exponentialHistogramDatapoint.SetMin(math.NaN())
	exponentialHistogramDatapoint.SetMax(math.NaN())
	exponentialHistogramDatapoint.SetZeroCount(0)
	exponentialHistogramDatapoint.SetScale(1)
	exponentialHistogramDatapoint.Positive().SetOffset(1)
	exponentialHistogramDatapoint.Positive().BucketCounts().FromRaw([]uint64{
		1, 0, 1,
	})
	exponentialHistogramDatapoint.Negative().SetOffset(1)
	exponentialHistogramDatapoint.Negative().BucketCounts().FromRaw([]uint64{
		1, 0, 1,
	})
	exponentialHistogramDatapoint.Attributes().PutStr("label1", "value1")
	return otelMetrics
}

func generateTestExponentialHistogramMetricWithInfs(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	exponentialHistogramMetric := metric.SetEmptyExponentialHistogram()

	exponentialHistogramDatapoint := exponentialHistogramMetric.DataPoints().AppendEmpty()
	exponentialHistogramDatapoint.SetCount(4)
	exponentialHistogramDatapoint.SetSum(math.Inf(0))
	exponentialHistogramDatapoint.SetMin(math.Inf(0))
	exponentialHistogramDatapoint.SetMax(math.Inf(0))
	exponentialHistogramDatapoint.SetZeroCount(0)
	exponentialHistogramDatapoint.SetScale(1)
	exponentialHistogramDatapoint.Positive().SetOffset(1)
	exponentialHistogramDatapoint.Positive().BucketCounts().FromRaw([]uint64{
		1, 0, 1,
	})
	exponentialHistogramDatapoint.Negative().SetOffset(1)
	exponentialHistogramDatapoint.Negative().BucketCounts().FromRaw([]uint64{
		1, 0, 1,
	})
	exponentialHistogramDatapoint.Attributes().PutStr("label1", "value1")
	return otelMetrics
}

func generateTestSummaryMetric(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()

	for i := 0; i < 2; i++ {
		metric := metrics.AppendEmpty()
		metric.SetName(name)
		metric.SetUnit("Seconds")
		summaryMetric := metric.SetEmptySummary()
		summaryDatapoint := summaryMetric.DataPoints().AppendEmpty()
		summaryDatapoint.Attributes().PutStr("label1", "value1")
		summaryDatapoint.SetCount(uint64(5 * i))
		summaryDatapoint.SetSum(float64(15.0 * i))
		firstQuantile := summaryDatapoint.QuantileValues().AppendEmpty()
		firstQuantile.SetQuantile(0.0)
		firstQuantile.SetValue(1)
		secondQuantile := summaryDatapoint.QuantileValues().AppendEmpty()
		secondQuantile.SetQuantile(100.0)
		secondQuantile.SetValue(5)
	}

	return otelMetrics
}

func generateTestSummaryMetricWithNaN(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()

	for i := 0; i < 2; i++ {
		metric := metrics.AppendEmpty()
		metric.SetName(name)
		metric.SetUnit("Seconds")
		summaryMetric := metric.SetEmptySummary()
		summaryDatapoint := summaryMetric.DataPoints().AppendEmpty()
		summaryDatapoint.Attributes().PutStr("label1", "value1")
		summaryDatapoint.SetCount(uint64(5 * i))
		summaryDatapoint.SetSum(math.NaN())
		firstQuantile := summaryDatapoint.QuantileValues().AppendEmpty()
		firstQuantile.SetQuantile(math.NaN())
		firstQuantile.SetValue(math.NaN())
		secondQuantile := summaryDatapoint.QuantileValues().AppendEmpty()
		secondQuantile.SetQuantile(math.NaN())
		secondQuantile.SetValue(math.NaN())
	}

	return otelMetrics
}

func generateTestSummaryMetricWithInf(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()

	for i := 0; i < 2; i++ {
		metric := metrics.AppendEmpty()
		metric.SetName(name)
		metric.SetUnit("Seconds")
		summaryMetric := metric.SetEmptySummary()
		summaryDatapoint := summaryMetric.DataPoints().AppendEmpty()
		summaryDatapoint.Attributes().PutStr("label1", "value1")
		summaryDatapoint.SetCount(uint64(5 * i))
		summaryDatapoint.SetSum(math.Inf(0))
		firstQuantile := summaryDatapoint.QuantileValues().AppendEmpty()
		firstQuantile.SetQuantile(math.Inf(0))
		firstQuantile.SetValue(math.Inf(0))
		secondQuantile := summaryDatapoint.QuantileValues().AppendEmpty()
		secondQuantile.SetQuantile(math.Inf(0))
		secondQuantile.SetValue(math.Inf(0))
	}

	return otelMetrics
}

func generateOtelTestMetrics(generatedOtelMetrics ...pmetric.Metrics) pmetric.Metrics {
	finalOtelMetrics := pmetric.NewMetrics()
	rs := finalOtelMetrics.ResourceMetrics().AppendEmpty()
	finalMetrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	for _, generatedOtelMetric := range generatedOtelMetrics {
		generatedMetrics := generatedOtelMetric.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		for i := 0; i < generatedMetrics.Len(); i++ {
			generatedMetric := generatedMetrics.At(i)
			finalMetric := finalMetrics.AppendEmpty()
			generatedMetric.CopyTo(finalMetric)
		}
	}
	return finalOtelMetrics
}
func generateDeltaMetricMetadata(adjustToDelta bool, metricName string, retainInitialValueForDelta bool) deltaMetricMetadata {
	return deltaMetricMetadata{
		adjustToDelta:              adjustToDelta,
		metricName:                 metricName,
		logGroup:                   "log-group",
		logStream:                  "log-stream",
		namespace:                  "namespace",
		retainInitialValueForDelta: retainInitialValueForDelta,
	}
}

func setupEmfCalculators() *emfCalculators {
	return &emfCalculators{
		summary: aws.NewMetricCalculator(calculateSummaryDelta),
		delta:   aws.NewFloat64DeltaCalculator(),
	}
}

func shutdownEmfCalculators(c *emfCalculators) error {
	var errs error
	errs = multierr.Append(errs, c.delta.Shutdown())
	return multierr.Append(errs, c.summary.Shutdown())

}

func TestIsStaleNaNInf_NumberDataPointSlice(t *testing.T) {
	testCases := []struct {
		name           string
		metricName     string
		metricValue    any
		expectedAssert assert.BoolAssertionFunc
		setFlagsFunc   func(point pmetric.NumberDataPoint) pmetric.NumberDataPoint
	}{
		{
			name:           "nan",
			metricValue:    math.NaN(),
			metricName:     "NaN",
			expectedAssert: assert.True,
		},
		{
			name:           "inf",
			metricValue:    math.Inf(0),
			metricName:     "Inf",
			expectedAssert: assert.True,
		},
		{
			name:           "valid float",
			metricValue:    0.4,
			metricName:     "floaty mc-float-face",
			expectedAssert: assert.False,
		},
		{
			name:           "data point flag set",
			metricValue:    0.4,
			metricName:     "floaty mc-float-face part two",
			expectedAssert: assert.True,
			setFlagsFunc: func(point pmetric.NumberDataPoint) pmetric.NumberDataPoint {
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				return point
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			// Given the number datapoint (including Sum and Gauge OTEL metric type) with data type as int or double
			numberDPS := pmetric.NewNumberDataPointSlice()

			numberDP := numberDPS.AppendEmpty()
			if tc.setFlagsFunc != nil {
				tc.setFlagsFunc(numberDP)
			}

			switch v := tc.metricValue.(type) {
			case int64:
				numberDP.SetIntValue(v)
			case float64:
				numberDP.SetDoubleValue(v)
			}

			numberDatapointSlice := numberDataPointSlice{deltaMetricMetadata{}, numberDPS}
			isStaleNanInf, _ := numberDatapointSlice.IsStaleNaNInf(0)
			tc.expectedAssert(t, isStaleNanInf)
		})
	}
}

func TestCalculateDeltaDatapoints_NumberDataPointSlice(t *testing.T) {
	emfCalcs := setupEmfCalculators()
	defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
	for _, retainInitialValueOfDeltaMetric := range []bool{true, false} {

		testCases := []struct {
			name              string
			adjustToDelta     bool
			metricName        string
			metricValue       any
			expectedDatapoint dataPoint
			expectedRetained  bool
		}{
			{
				name:          fmt.Sprintf("Float data type with 1st delta calculation retainInitialValueOfDeltaMetric=%t", retainInitialValueOfDeltaMetric),
				adjustToDelta: true,
				metricValue:   0.4,
				metricName:    "double",
				expectedDatapoint: dataPoint{
					name:   "double",
					value:  0.4,
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				expectedRetained: retainInitialValueOfDeltaMetric,
			},
			{
				name:          "Float data type with 2nd delta calculation",
				adjustToDelta: true,
				metricName:    "double",
				metricValue:   0.8,
				expectedDatapoint: dataPoint{
					name:   "double",
					value:  0.4,
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				expectedRetained: true,
			},
			{
				name:          "Double data type without delta calculation",
				adjustToDelta: false,
				metricName:    "double",
				metricValue:   0.5,
				expectedDatapoint: dataPoint{
					name:   "double",
					value:  0.5,
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				expectedRetained: true,
			},

			{
				name:          "Int data type with 1st delta calculation",
				adjustToDelta: true,
				metricName:    "int",
				metricValue:   int64(-17),
				expectedDatapoint: dataPoint{
					name:   "int",
					value:  float64(-17),
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				expectedRetained: retainInitialValueOfDeltaMetric,
			},
			{
				name:          "Int data type with 2nd delta calculation",
				adjustToDelta: true,
				metricName:    "int",
				metricValue:   int64(1),
				expectedDatapoint: dataPoint{
					name:   "int",
					value:  float64(18),
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				expectedRetained: true,
			},
			{
				name:          "Int data type without delta calculation",
				adjustToDelta: false,
				metricName:    "int",
				metricValue:   int64(10),
				expectedDatapoint: dataPoint{
					name:   "int",
					value:  float64(10),
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				expectedRetained: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {

				// Given the number datapoint (including Sum and Gauge OTEL metric type) with data type as int or double
				numberDPS := pmetric.NewNumberDataPointSlice()
				numberDP := numberDPS.AppendEmpty()

				numberDP.Attributes().PutStr("label1", "value1")

				switch v := tc.metricValue.(type) {
				case int64:
					numberDP.SetIntValue(v)
				case float64:
					numberDP.SetDoubleValue(v)
				}

				dmd := generateDeltaMetricMetadata(tc.adjustToDelta, tc.metricName, retainInitialValueOfDeltaMetric)
				numberDatapointSlice := numberDataPointSlice{dmd, numberDPS}

				// When calculate the delta datapoints for number datapoint
				dps, retained := numberDatapointSlice.CalculateDeltaDatapoints(0, instrLibName, false, emfCalcs)

				assert.Equal(t, 1, numberDatapointSlice.Len())
				assert.Equal(t, tc.expectedRetained, retained)
				if retained {
					assert.Equal(t, tc.expectedDatapoint.name, dps[0].name)
					assert.Equal(t, tc.expectedDatapoint.labels, dps[0].labels)
					// Asserting the delta is within 0.002 with the following datapoint
					assert.InDelta(t, tc.expectedDatapoint.value, dps[0].value, 0.002)
				}
			})
		}
	}
}

func TestCalculateDeltaDatapoints_HistogramDataPointSlice(t *testing.T) {
	dmd := generateDeltaMetricMetadata(false, "foo", false)

	testCases := []struct {
		name              string
		histogramDPS      pmetric.HistogramDataPointSlice
		expectedDatapoint dataPoint
	}{
		{
			name: "Histogram with min and max",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.SetMin(10)
				histogramDP.SetMax(30)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricStats{Sum: 17.13, Count: 17, Min: 10, Max: 30},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
			},
		},
		{
			name: "Histogram without min and max",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS

			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricStats{Sum: 17.13, Count: 17, Min: 0, Max: 0},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
			},
		},
		{
			name: "Histogram with buckets and bounds",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.BucketCounts().FromRaw([]uint64{1, 2, 3})
				histogramDP.ExplicitBounds().FromRaw([]float64{1, 2, 3})
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricStats{Sum: 17.13, Count: 17, Min: 0, Max: 0},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			// Given the histogram datapoints
			histogramDatapointSlice := histogramDataPointSlice{dmd, tc.histogramDPS}
			emfCalcs := setupEmfCalculators()
			defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
			// When calculate the delta datapoints for histograms
			dps, retained := histogramDatapointSlice.CalculateDeltaDatapoints(0, instrLibName, false, emfCalcs)

			// Then receiving the following datapoint with an expected length
			assert.True(t, retained)
			assert.Equal(t, 1, histogramDatapointSlice.Len())
			assert.Equal(t, tc.expectedDatapoint, dps[0])

		})
	}

}

func TestIsStaleNaNInf_HistogramDataPointSlice(t *testing.T) {

	testCases := []struct {
		name           string
		histogramDPS   pmetric.HistogramDataPointSlice
		boolAssertFunc assert.BoolAssertionFunc
		setFlagsFunc   func(point pmetric.HistogramDataPoint) pmetric.HistogramDataPoint
	}{
		{
			name: "Histogram with all NaNs",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.NaN())
				histogramDP.SetMin(math.NaN())
				histogramDP.SetMax(math.NaN())
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with NaN Sum",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.NaN())
				histogramDP.SetMin(1234)
				histogramDP.SetMax(1234)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with NaN Min",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(123)
				histogramDP.SetMin(math.NaN())
				histogramDP.SetMax(123)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with nan Max",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(123)
				histogramDP.SetMin(123)
				histogramDP.SetMax(math.NaN())
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with min and max",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.SetMin(10)
				histogramDP.SetMax(30)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.False,
		},
		{
			name: "Histogram with no NaNs or Inf",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.SetMin(10)
				histogramDP.SetMax(30)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
			setFlagsFunc: func(point pmetric.HistogramDataPoint) pmetric.HistogramDataPoint {
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				return point
			},
		},
		{
			name: "Histogram with all Infs",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.Inf(0))
				histogramDP.SetMin(math.Inf(0))
				histogramDP.SetMax(math.Inf(0))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with Inf Sum",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.Inf(0))
				histogramDP.SetMin(1234)
				histogramDP.SetMax(1234)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with Inf Min",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(123)
				histogramDP.SetMin(math.Inf(0))
				histogramDP.SetMax(123)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Histogram with Inf Max",
			histogramDPS: func() pmetric.HistogramDataPointSlice {
				histogramDPS := pmetric.NewHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(123)
				histogramDP.SetMin(123)
				histogramDP.SetMax(math.Inf(0))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			// Given the histogram datapoints
			histogramDatapointSlice := histogramDataPointSlice{deltaMetricMetadata{}, tc.histogramDPS}
			if tc.setFlagsFunc != nil {
				tc.setFlagsFunc(histogramDatapointSlice.At(0))
			}
			isStaleNanInf, _ := histogramDatapointSlice.IsStaleNaNInf(0)
			tc.boolAssertFunc(t, isStaleNanInf)
		})
	}

}

func TestCalculateDeltaDatapoints_ExponentialHistogramDataPointSlice(t *testing.T) {
	dmd := generateDeltaMetricMetadata(false, "foo", false)

	testCases := []struct {
		name              string
		histogramDPS      pmetric.ExponentialHistogramDataPointSlice
		expectedDatapoint dataPoint
	}{
		{
			name: "Exponential histogram with min and max",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.SetMin(10)
				histogramDP.SetMax(30)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricHistogram{Values: []float64{}, Counts: []float64{}, Sum: 17.13, Count: 17, Min: 10, Max: 30},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
			},
		},
		{
			name: "Exponential histogram without min and max",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS

			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricHistogram{Values: []float64{}, Counts: []float64{}, Sum: 17.13, Count: 17, Min: 0, Max: 0},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
			},
		},
		{
			name: "Exponential histogram with buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3})
				histogramDP.SetZeroCount(4)
				histogramDP.Negative().BucketCounts().FromRaw([]uint64{1, 2, 3})
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricHistogram{Values: []float64{1.5, 3, 6, 0, -1.5, -3, -6}, Counts: []float64{1, 2, 3, 4, 1, 2, 3}},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
			},
		},
		{
			name: "Exponential histogram with different scale/offset/labels",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetScale(-1)
				histogramDP.Positive().SetOffset(-1)
				histogramDP.Positive().BucketCounts().FromRaw([]uint64{1, 2, 3})
				histogramDP.SetZeroCount(4)
				histogramDP.Negative().SetOffset(-1)
				histogramDP.Negative().BucketCounts().FromRaw([]uint64{1, 2, 3})
				histogramDP.Attributes().PutStr("label1", "value1")
				histogramDP.Attributes().PutStr("label2", "value2")
				return histogramDPS
			}(),
			expectedDatapoint: dataPoint{
				name:   "foo",
				value:  &cWMetricHistogram{Values: []float64{0.625, 2.5, 10, 0, -0.625, -2.5, -10}, Counts: []float64{1, 2, 3, 4, 1, 2, 3}},
				labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1", "label2": "value2"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			// Given the histogram datapoints
			exponentialHistogramDatapointSlice := exponentialHistogramDataPointSlice{dmd, tc.histogramDPS}
			emfCalcs := setupEmfCalculators()
			defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
			// When calculate the delta datapoints for histograms
			dps, retained := exponentialHistogramDatapointSlice.CalculateDeltaDatapoints(0, instrLibName, false, emfCalcs)

			// Then receiving the following datapoint with an expected length
			assert.True(t, retained)
			assert.Equal(t, 1, exponentialHistogramDatapointSlice.Len())
			assert.Equal(t, tc.expectedDatapoint, dps[0])
		})
	}

}

func TestIsStaleNaNInf_ExponentialHistogramDataPointSlice(t *testing.T) {

	testCases := []struct {
		name           string
		histogramDPS   pmetric.ExponentialHistogramDataPointSlice
		boolAssertFunc assert.BoolAssertionFunc
		setFlagsFunc   func(point pmetric.ExponentialHistogramDataPoint) pmetric.ExponentialHistogramDataPoint
	}{
		{
			name: "Exponential histogram with non NaNs or Infs",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.SetMin(10)
				histogramDP.SetMax(30)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.False,
		},
		{
			name: "Exponential histogram with all possible NaN",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.NaN())
				histogramDP.SetMin(math.NaN())
				histogramDP.SetMax(math.NaN())
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with NaN Sum",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.NaN())
				histogramDP.SetMin(1245)
				histogramDP.SetMax(1234556)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with NaN Min",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(1255)
				histogramDP.SetMin(math.NaN())
				histogramDP.SetMax(12545)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with NaN Max",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(512444)
				histogramDP.SetMin(123)
				histogramDP.SetMax(math.NaN())
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with NaNs",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.NaN())
				histogramDP.SetMin(math.NaN())
				histogramDP.SetMax(math.NaN())
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with set flag func",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(17.13)
				histogramDP.SetMin(10)
				histogramDP.SetMax(30)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
			setFlagsFunc: func(point pmetric.ExponentialHistogramDataPoint) pmetric.ExponentialHistogramDataPoint {
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				return point
			},
		},
		{
			name: "Exponential histogram with all possible Inf",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.Inf(0))
				histogramDP.SetMin(math.Inf(0))
				histogramDP.SetMax(math.Inf(0))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with Inf Sum",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.Inf(0))
				histogramDP.SetMin(1245)
				histogramDP.SetMax(1234556)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with Inf Min",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(1255)
				histogramDP.SetMin(math.Inf(0))
				histogramDP.SetMax(12545)
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with Inf Max",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(512444)
				histogramDP.SetMin(123)
				histogramDP.SetMax(math.Inf(0))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
		{
			name: "Exponential histogram with Infs",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				histogramDP.SetCount(uint64(17))
				histogramDP.SetSum(math.Inf(0))
				histogramDP.SetMin(math.Inf(0))
				histogramDP.SetMax(math.Inf(0))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			boolAssertFunc: assert.True,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			// Given the histogram datapoints
			exponentialHistogramDatapointSlice := exponentialHistogramDataPointSlice{deltaMetricMetadata{}, tc.histogramDPS}
			if tc.setFlagsFunc != nil {
				tc.setFlagsFunc(exponentialHistogramDatapointSlice.At(0))
			}
			isStaleNaNInf, _ := exponentialHistogramDatapointSlice.IsStaleNaNInf(0)
			// When calculate the delta datapoints for histograms
			tc.boolAssertFunc(t, isStaleNaNInf)
		})
	}

}

func TestCalculateDeltaDatapoints_SummaryDataPointSlice(t *testing.T) {
	emfCalcs := setupEmfCalculators()
	defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
	for _, retainInitialValueOfDeltaMetric := range []bool{true, false} {
		dmd := generateDeltaMetricMetadata(true, "foo", retainInitialValueOfDeltaMetric)

		testCases := []struct {
			name               string
			summaryMetricValue map[string]any
			expectedDatapoint  []dataPoint
			expectedRetained   bool
		}{
			{
				name:               fmt.Sprintf("Detailed summary with 1st delta sum count calculation retainInitialValueOfDeltaMetric=%t", retainInitialValueOfDeltaMetric),
				summaryMetricValue: map[string]any{"sum": float64(17.3), "count": uint64(17), "firstQuantile": float64(1), "secondQuantile": float64(5)},
				expectedDatapoint: []dataPoint{
					{name: fmt.Sprint("foo", summarySumSuffix), value: float64(17.3), labels: map[string]string{"label1": "value1"}},
					{name: fmt.Sprint("foo", summaryCountSuffix), value: uint64(17), labels: map[string]string{"label1": "value1"}},
					{name: "foo", value: float64(1), labels: map[string]string{"label1": "value1", "quantile": "0"}},
					{name: "foo", value: float64(5), labels: map[string]string{"label1": "value1", "quantile": "100"}},
				},
				expectedRetained: retainInitialValueOfDeltaMetric,
			},
			{
				name:               "Detailed summary with 2nd delta sum count calculation",
				summaryMetricValue: map[string]any{"sum": float64(100), "count": uint64(25), "firstQuantile": float64(1), "secondQuantile": float64(5)},
				expectedDatapoint: []dataPoint{
					{name: fmt.Sprint("foo", summarySumSuffix), value: float64(82.7), labels: map[string]string{"label1": "value1"}},
					{name: fmt.Sprint("foo", summaryCountSuffix), value: uint64(8), labels: map[string]string{"label1": "value1"}},
					{name: "foo", value: float64(1), labels: map[string]string{"label1": "value1", "quantile": "0"}},
					{name: "foo", value: float64(5), labels: map[string]string{"label1": "value1", "quantile": "100"}},
				},
				expectedRetained: true,
			},
			{
				name:               "Detailed summary with 3rd delta sum count calculation",
				summaryMetricValue: map[string]any{"sum": float64(120), "count": uint64(26), "firstQuantile": float64(1), "secondQuantile": float64(5)},
				expectedDatapoint: []dataPoint{
					{name: fmt.Sprint("foo", summarySumSuffix), value: float64(20), labels: map[string]string{"label1": "value1"}},
					{name: fmt.Sprint("foo", summaryCountSuffix), value: uint64(1), labels: map[string]string{"label1": "value1"}},
					{name: "foo", value: float64(1), labels: map[string]string{"label1": "value1", "quantile": "0"}},
					{name: "foo", value: float64(5), labels: map[string]string{"label1": "value1", "quantile": "100"}},
				},
				expectedRetained: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Given the summary datapoints with quantile 0, quantile 100, sum and count
				summaryDPS := pmetric.NewSummaryDataPointSlice()
				summaryDP := summaryDPS.AppendEmpty()
				summaryDP.SetSum(tc.summaryMetricValue["sum"].(float64))
				summaryDP.SetCount(tc.summaryMetricValue["count"].(uint64))
				summaryDP.Attributes().PutStr("label1", "value1")

				summaryDP.QuantileValues().EnsureCapacity(2)
				firstQuantileValue := summaryDP.QuantileValues().AppendEmpty()
				firstQuantileValue.SetQuantile(0)
				firstQuantileValue.SetValue(tc.summaryMetricValue["firstQuantile"].(float64))
				secondQuantileValue := summaryDP.QuantileValues().AppendEmpty()
				secondQuantileValue.SetQuantile(100)
				secondQuantileValue.SetValue(tc.summaryMetricValue["secondQuantile"].(float64))

				summaryDatapointSlice := summaryDataPointSlice{dmd, summaryDPS}

				// When calculate the delta datapoints for sum and count in summary
				dps, retained := summaryDatapointSlice.CalculateDeltaDatapoints(0, "", true, emfCalcs)

				// Then receiving the following datapoint with an expected length
				assert.Equal(t, tc.expectedRetained, retained)
				assert.Equal(t, 1, summaryDatapointSlice.Len())
				if retained {
					assert.Len(t, dps, 4)
					for i, dp := range dps {
						assert.Equal(t, tc.expectedDatapoint[i].labels, dp.labels)
						assert.InDelta(t, tc.expectedDatapoint[i].value, dp.value, 0.002)
					}

				}
			})
		}
	}
}

func TestIsStaleNaNInf_SummaryDataPointSlice(t *testing.T) {
	type qMetricObject struct {
		value    float64
		quantile float64
	}
	type quantileTestObj struct {
		sum      float64
		count    uint64
		qMetrics []qMetricObject
	}
	testCases := []struct {
		name               string
		summaryMetricValue quantileTestObj
		expectedBoolAssert assert.BoolAssertionFunc
		setFlagsFunc       func(point pmetric.SummaryDataPoint) pmetric.SummaryDataPoint
	}{
		{
			name: "summary with no nan or inf values",
			summaryMetricValue: quantileTestObj{
				sum:   17.3,
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    5,
						quantile: 2.0,
					},
				},
			},
			expectedBoolAssert: assert.False,
		},
		{
			name: "Summary with nan sum",
			summaryMetricValue: quantileTestObj{
				sum:   math.NaN(),
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    5,
						quantile: 2.0,
					},
				},
			},
			expectedBoolAssert: assert.True,
		},
		{
			name: "Summary with no recorded value flag set to true",
			summaryMetricValue: quantileTestObj{
				sum:   1245.65,
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    5,
						quantile: 2.0,
					},
				},
			},
			expectedBoolAssert: assert.True,
			setFlagsFunc: func(point pmetric.SummaryDataPoint) pmetric.SummaryDataPoint {
				point.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				return point
			},
		},
		{
			name: "Summary with nan quantile value",
			summaryMetricValue: quantileTestObj{
				sum:   1245.65,
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    math.NaN(),
						quantile: 2.0,
					},
				},
			},
			expectedBoolAssert: assert.True,
		},
		{
			name: "Summary with nan quantile",
			summaryMetricValue: quantileTestObj{
				sum:   1245.65,
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    7.8,
						quantile: math.NaN(),
					},
				},
			},
			expectedBoolAssert: assert.True,
		},
		{
			name: "Summary with Inf sum",
			summaryMetricValue: quantileTestObj{
				sum:   math.Inf(0),
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    5,
						quantile: 2.0,
					},
				},
			},
			expectedBoolAssert: assert.True,
		},
		{
			name: "Summary with inf quantile value",
			summaryMetricValue: quantileTestObj{
				sum:   1245.65,
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    math.Inf(0),
						quantile: 2.0,
					},
				},
			},
			expectedBoolAssert: assert.True,
		},
		{
			name: "Summary with inf quantile",
			summaryMetricValue: quantileTestObj{
				sum:   1245.65,
				count: 17,
				qMetrics: []qMetricObject{
					{
						value:    1,
						quantile: 0.5,
					},
					{
						value:    7.8,
						quantile: math.Inf(0),
					},
				},
			},
			expectedBoolAssert: assert.True,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summaryDPS := pmetric.NewSummaryDataPointSlice()
			summaryDP := summaryDPS.AppendEmpty()
			summaryDP.SetSum(tc.summaryMetricValue.sum)
			summaryDP.SetCount(tc.summaryMetricValue.count)
			summaryDP.Attributes().PutStr("label1", "value1")

			summaryDP.QuantileValues().EnsureCapacity(len(tc.summaryMetricValue.qMetrics))
			for _, qMetric := range tc.summaryMetricValue.qMetrics {
				newQ := summaryDP.QuantileValues().AppendEmpty()
				newQ.SetValue(qMetric.value)
				newQ.SetQuantile(qMetric.quantile)
			}

			summaryDatapointSlice := summaryDataPointSlice{deltaMetricMetadata{}, summaryDPS}
			if tc.setFlagsFunc != nil {
				tc.setFlagsFunc(summaryDatapointSlice.At(0))
			}
			isStaleNaNInf, _ := summaryDatapointSlice.IsStaleNaNInf(0)
			tc.expectedBoolAssert(t, isStaleNaNInf)
		})
	}

}

func TestCreateLabels(t *testing.T) {
	expectedLabels := map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	}
	labelsMap := pcommon.NewMap()
	assert.NoError(t, labelsMap.FromRaw(map[string]any{
		"a": "A",
		"b": "B",
		"c": "C",
	}))

	labels := createLabels(labelsMap, "")
	assert.Equal(t, expectedLabels, labels)

	// With isntrumentation library name
	labels = createLabels(labelsMap, "cloudwatch-otel")
	expectedLabels[oTellibDimensionKey] = "cloudwatch-otel"
	assert.Equal(t, expectedLabels, labels)
}

func TestGetDataPoints(t *testing.T) {
	logger := zap.NewNop()

	normalDeltraMetricMetadata := generateDeltaMetricMetadata(false, "foo", false)
	cumulativeDeltaMetricMetadata := generateDeltaMetricMetadata(true, "foo", false)

	testCases := []struct {
		name                   string
		isPrometheusMetrics    bool
		metric                 pmetric.Metrics
		expectedDatapointSlice dataPoints
		expectedAttributes     map[string]any
	}{
		{
			name:                   "Int gauge",
			isPrometheusMetrics:    false,
			metric:                 generateTestGaugeMetric("foo", intValueType),
			expectedDatapointSlice: numberDataPointSlice{normalDeltraMetricMetadata, pmetric.NumberDataPointSlice{}},
			expectedAttributes:     map[string]any{"label1": "value1"},
		},
		{
			name:                   "Double sum",
			isPrometheusMetrics:    false,
			metric:                 generateTestSumMetric("foo", doubleValueType),
			expectedDatapointSlice: numberDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.NumberDataPointSlice{}},
			expectedAttributes:     map[string]any{"label1": "value1"},
		},
		{
			name:                   "Histogram",
			isPrometheusMetrics:    false,
			metric:                 generateTestHistogramMetric("foo"),
			expectedDatapointSlice: histogramDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.HistogramDataPointSlice{}},
			expectedAttributes:     map[string]any{"label1": "value1"},
		},
		{
			name:                   "ExponentialHistogram",
			isPrometheusMetrics:    false,
			metric:                 generateTestExponentialHistogramMetric("foo"),
			expectedDatapointSlice: exponentialHistogramDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.ExponentialHistogramDataPointSlice{}},
			expectedAttributes:     map[string]any{"label1": "value1"},
		},
		{
			name:                   "Summary from SDK",
			isPrometheusMetrics:    false,
			metric:                 generateTestSummaryMetric("foo"),
			expectedDatapointSlice: summaryDataPointSlice{normalDeltraMetricMetadata, pmetric.SummaryDataPointSlice{}},
			expectedAttributes:     map[string]any{"label1": "value1"},
		},
		{
			name:                   "Summary from Prometheus",
			isPrometheusMetrics:    true,
			metric:                 generateTestSummaryMetric("foo"),
			expectedDatapointSlice: summaryDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.SummaryDataPointSlice{}},
			expectedAttributes:     map[string]any{"label1": "value1"},
		},
	}

	for _, tc := range testCases {
		rm := tc.metric.ResourceMetrics().At(0)
		metrics := rm.ScopeMetrics().At(0).Metrics()
		metric := metrics.At(metrics.Len() - 1)
		metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", metric.Type())

		t.Run(tc.name, func(t *testing.T) {

			if tc.isPrometheusMetrics {
				metadata.receiver = prometheusReceiver
			} else {
				metadata.receiver = ""
			}

			dps := getDataPoints(metric, metadata, logger)
			assert.NotNil(t, dps)
			assert.Equal(t, reflect.TypeOf(tc.expectedDatapointSlice), reflect.TypeOf(dps))
			switch convertedDPS := dps.(type) {
			case numberDataPointSlice:
				expectedDPS := tc.expectedDatapointSlice.(numberDataPointSlice)
				assert.Equal(t, expectedDPS.deltaMetricMetadata, convertedDPS.deltaMetricMetadata)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.NumberDataPointSlice.At(0)
				switch dp.ValueType() {
				case pmetric.NumberDataPointValueTypeDouble:
					assert.Equal(t, 0.1, dp.DoubleValue())
				case pmetric.NumberDataPointValueTypeInt:
					assert.Equal(t, int64(1), dp.IntValue())
				}
				assert.Equal(t, tc.expectedAttributes, dp.Attributes().AsRaw())
			case histogramDataPointSlice:
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.HistogramDataPointSlice.At(0)
				assert.Equal(t, 35.0, dp.Sum())
				assert.Equal(t, uint64(18), dp.Count())
				assert.Equal(t, []float64{0, 10}, dp.ExplicitBounds().AsRaw())
				assert.Equal(t, tc.expectedAttributes, dp.Attributes().AsRaw())
			case exponentialHistogramDataPointSlice:
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.ExponentialHistogramDataPointSlice.At(0)
				assert.Equal(t, float64(0), dp.Sum())
				assert.Equal(t, uint64(4), dp.Count())
				assert.Equal(t, []uint64{1, 0, 1}, dp.Positive().BucketCounts().AsRaw())
				assert.Equal(t, []uint64{1, 0, 1}, dp.Negative().BucketCounts().AsRaw())
				assert.Equal(t, uint64(0), dp.ZeroCount())
				assert.Equal(t, tc.expectedAttributes, dp.Attributes().AsRaw())
			case summaryDataPointSlice:
				expectedDPS := tc.expectedDatapointSlice.(summaryDataPointSlice)
				assert.Equal(t, expectedDPS.deltaMetricMetadata, convertedDPS.deltaMetricMetadata)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.SummaryDataPointSlice.At(0)
				assert.Equal(t, 15.0, dp.Sum())
				assert.Equal(t, uint64(5), dp.Count())
				assert.Equal(t, 2, dp.QuantileValues().Len())
				assert.Equal(t, float64(1), dp.QuantileValues().At(0).Value())
				assert.Equal(t, float64(5), dp.QuantileValues().At(1).Value())
				assert.Equal(t, tc.expectedAttributes, dp.Attributes().AsRaw())
			}
		})
	}

	t.Run("Unhandled metric type", func(t *testing.T) {
		metric := pmetric.NewMetric()
		metric.SetName("foo")
		metric.SetUnit("Count")
		metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", pmetric.MetricTypeEmpty)
		obs, logs := observer.New(zap.WarnLevel)
		logger := zap.New(obs)

		dps := getDataPoints(metric, metadata, logger)
		assert.Nil(t, dps)

		// Test output warning logs
		expectedLogs := []observer.LoggedEntry{
			{
				Entry: zapcore.Entry{Level: zap.WarnLevel, Message: "Unhandled metric data type."},
				Context: []zapcore.Field{
					zap.String("DataType", "Empty"),
					zap.String("Name", "foo"),
					zap.String("Unit", "Count"),
				},
			},
		}
		assert.Equal(t, 1, logs.Len())
		assert.Equal(t, expectedLogs, logs.AllUntimed())
	})
}

func BenchmarkGetAndCalculateDeltaDataPoints(b *testing.B) {
	generateMetrics := []pmetric.Metrics{
		generateTestGaugeMetric("int-gauge", intValueType),
		generateTestGaugeMetric("int-gauge", doubleValueType),
		generateTestHistogramMetric("histogram"),
		generateTestExponentialHistogramMetric("exponential-histogram"),
		generateTestSumMetric("int-sum", intValueType),
		generateTestSumMetric("double-sum", doubleValueType),
		generateTestSummaryMetric("summary"),
	}

	finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)
	rms := finalOtelMetrics.ResourceMetrics()
	metrics := rms.At(0).ScopeMetrics().At(0).Metrics()
	emfCalcs := setupEmfCalculators()
	defer require.NoError(b, shutdownEmfCalculators(emfCalcs))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < metrics.Len(); i++ {
			metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", metrics.At(i).Type())
			dps := getDataPoints(metrics.At(i), metadata, zap.NewNop())

			for i := 0; i < dps.Len(); i++ {
				dps.CalculateDeltaDatapoints(i, "", false, emfCalcs)
			}
		}
	}
}
