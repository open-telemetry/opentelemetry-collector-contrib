// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
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

func generateTestExponentialHistogramMetricWithLongBuckets(name string) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	exponentialHistogramMetric := metric.SetEmptyExponentialHistogram()

	exponentialHistogramDatapoint := exponentialHistogramMetric.DataPoints().AppendEmpty()
	exponentialHistogramDatapoint.SetCount(3662)
	exponentialHistogramDatapoint.SetSum(1000)
	exponentialHistogramDatapoint.SetMin(-9e+17)
	exponentialHistogramDatapoint.SetMax(9e+17)
	exponentialHistogramDatapoint.SetZeroCount(2)
	posBucketCounts := make([]uint64, 60)
	for i := range posBucketCounts {
		posBucketCounts[i] = uint64(i + 1)
	}
	exponentialHistogramDatapoint.Positive().BucketCounts().FromRaw(posBucketCounts)
	negBucketCounts := make([]uint64, 60)
	for i := range negBucketCounts {
		negBucketCounts[i] = uint64(i + 1)
	}
	exponentialHistogramDatapoint.Negative().BucketCounts().FromRaw(negBucketCounts)
	exponentialHistogramDatapoint.Attributes().PutStr("label1", "value1")
	return otelMetrics
}

func generateTestExponentialHistogramMetricWithSpecifiedNumberOfBuckets(name string, bucketLength int) pmetric.Metrics {
	halfBucketLength := bucketLength / 2
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Seconds")
	exponentialHistogramMetric := metric.SetEmptyExponentialHistogram()

	exponentialHistogramDatapoint := exponentialHistogramMetric.DataPoints().AppendEmpty()
	exponentialHistogramDatapoint.SetCount(250550)
	exponentialHistogramDatapoint.SetSum(10000)
	exponentialHistogramDatapoint.SetMin(-9e+20)
	exponentialHistogramDatapoint.SetMax(9e+20)
	exponentialHistogramDatapoint.SetZeroCount(50)
	posBucketCounts := make([]uint64, halfBucketLength)
	for i := range posBucketCounts {
		posBucketCounts[i] = uint64(i + 1)
	}
	exponentialHistogramDatapoint.Positive().BucketCounts().FromRaw(posBucketCounts)
	negBucketCounts := make([]uint64, halfBucketLength)
	for i := range negBucketCounts {
		negBucketCounts[i] = uint64(i + 1)
	}
	exponentialHistogramDatapoint.Negative().BucketCounts().FromRaw(negBucketCounts)
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
				value:  &cWMetricHistogram{Values: []float64{6, 3, 1.5, 0, -1.5, -3, -6}, Counts: []float64{3, 2, 1, 4, 1, 2, 3}, Count: 16},
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
				value:  &cWMetricHistogram{Values: []float64{10, 2.5, 0.625, 0, -0.625, -2.5, -10}, Counts: []float64{3, 2, 1, 4, 1, 2, 3}, Count: 16},
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

func TestCalculateDeltaDatapoints_ExponentialHistogramDataPointSliceWithSplitDataPoints(t *testing.T) {
	dmd := generateDeltaMetricMetadata(false, "foo", false)

	testCases := []struct {
		name               string
		histogramDPS       pmetric.ExponentialHistogramDataPointSlice
		expectedDatapoints []dataPoint
	}{
		{
			name: "Exponential histogram with more than 100 buckets, including positive, negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3662))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14,
							2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12,
							1.649267441664e+12, 8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10,
							1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
							5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576,
							12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072,
							-6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07,
							-5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09,
							-1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11,
						},
						Counts: []float64{
							60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7,
							6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33,
							34, 35, 36, 37, 38, 39,
						},
						Sum: 1000, Count: 2612, Min: -5.49755813888e+11, Max: 9e+17,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							-8.24633720832e+11, -1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
							-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15, -3.377699720527872e+15,
							-6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						},
						Counts: []float64{40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60},
						Sum:    0, Count: 1050, Min: -9e+17, Max: -5.49755813888e+11,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
		{
			name: "Exponential histogram with more than 100 buckets, including positive and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 120)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				histogramDP.SetSum(10000)
				histogramDP.SetMin(0)
				histogramDP.SetMax(9e+36)
				histogramDP.SetCount(uint64(7262))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34,
							3.1153781151208966e+34, 1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33,
							9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31,
							3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30, 1.901475900342344e+30,
							9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
							2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
							9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
							2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
							9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
							2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
							8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
							2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
							2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
							8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
							2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
							4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
							6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
							5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06,
						},
						Counts: []float64{
							120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99,
							98, 97, 96, 95, 94, 93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68,
							67, 66, 65, 64, 63, 62, 61, 60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37,
							36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21,
						},
						Sum: 10000, Count: 7050, Min: 1.048576e+06, Max: 9e+36,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{786432, 393216, 196608, 98304, 49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, 0},
						Counts: []float64{20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2},
						Sum:    0, Count: 212, Min: 0, Max: 1.048576e+06,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
		{
			name: "Exponential histogram with more than 100 buckets, including negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				negBucketCounts := make([]uint64, 120)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetZeroCount(2)
				histogramDP.SetSum(10000)
				histogramDP.SetMin(-9e+36)
				histogramDP.SetMax(0)
				histogramDP.SetCount(uint64(7262))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072, -6144, -12288, -24576,
							-49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07,
							-2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08,
							-1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10,
							-5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
							-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13,
							-5.2776558133248e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16,
							-2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18,
							-6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19,
							-1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
							-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22,
							-2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
							-4.5334718235548594e+23, -9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24,
							-7.253554917687775e+24, -1.450710983537555e+25, -2.90142196707511e+25, -5.80284393415022e+25,
							-1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
							-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28,
							-2.9710560942849127e+28, -5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29,
						},
						Counts: []float64{
							2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
							25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46,
							47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72,
							73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
						},
						Sum: 10000, Count: 4952, Min: -6.338253001141147e+29, Max: 0,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							-9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30, -7.605903601369376e+30,
							-1.5211807202738753e+31, -3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32,
							-2.4338891524382005e+32, -4.867778304876401e+32, -9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33,
							-7.788445287802241e+33, -1.5576890575604483e+34, -3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35,
							-2.4923024920967173e+35, -4.9846049841934345e+35, -9.969209968386869e+35,
						},
						Counts: []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120},
						Sum:    0, Count: 2310, Min: -9e+36, Max: -6.338253001141147e+29,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
		{
			name: "Exponential histogram with more than 100 buckets, including positive and negative buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3660))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15, 8.44424930131968e+14, 4.22212465065984e+14,
							2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12,
							1.649267441664e+12, 8.24633720832e+11, 4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10,
							1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08,
							5.0331648e+07, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576,
							12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072,
							-6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07,
							-5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09,
							-1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
						},
						Counts: []float64{
							60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28, 27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7,
							6, 5, 4, 3, 2, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33,
							34, 35, 36, 37, 38, 39, 40,
						},
						Sum: 1000, Count: 2650, Min: -1.099511627776e+12, Max: 9e+17,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
							-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15, -3.377699720527872e+15,
							-6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						},
						Counts: []float64{41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60},
						Sum:    0, Count: 1010, Min: -9e+17, Max: -1.099511627776e+12,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
		{
			name: "Exponential histogram with exact 200 buckets, including positive, negative buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 100)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				negBucketCounts := make([]uint64, 100)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(100000)
				histogramDP.SetMin(-9e+36)
				histogramDP.SetMax(9e+36)
				histogramDP.SetCount(uint64(3662))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
							2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
							9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
							2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
							9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
							2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
							8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
							2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
							2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
							8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
							2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
							4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09,
							3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07,
							2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06, 786432, 393216, 196608, 98304, 49152, 24576, 12288,
							6144, 3072, 1536, 768, 384, 192, 96, 48, 24, 12, 6, 3, 1.5,
						},
						Counts: []float64{
							100, 99, 98, 97, 96, 95, 94,
							93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61,
							60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28,
							27, 26, 25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1,
						},
						Sum: 100000, Count: 5050, Min: 1, Max: 9e+36,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							-1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536, -3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216,
							-786432, -1.572864e+06, -3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08,
							-2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10,
							-2.5769803776e+10, -5.1539607552e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11,
							-1.649267441664e+12, -3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
							-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14, -1.688849860263936e+15,
							-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16,
							-5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17, -4.323455642275676e+17, -8.646911284551352e+17,
							-1.7293822569102705e+18, -3.458764513820541e+18, -6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19,
							-5.5340232221128655e+19, -1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
							-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22, -2.833419889721787e+22,
							-5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23, -4.5334718235548594e+23, -9.066943647109719e+23,
							-1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25, -2.90142196707511e+25,
							-5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26, -9.284550294640352e+26,
							-1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28, -2.9710560942849127e+28,
							-5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29, -9.50737950171172e+29,
						},
						Counts: []float64{
							1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35,
							36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68,
							69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100,
						},
						Sum: 0, Count: 5050, Min: -9e+36, Max: -1,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
		{
			name: "Exponential histogram with more than 200 buckets, including positive, negative and zero buckets",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 120)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 120)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i + 1)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(100000)
				histogramDP.SetMin(-9e+36)
				histogramDP.SetMax(9e+36)
				histogramDP.SetCount(uint64(3662))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							9.969209968386869e+35, 4.9846049841934345e+35, 2.4923024920967173e+35, 1.2461512460483586e+35, 6.230756230241793e+34,
							3.1153781151208966e+34, 1.5576890575604483e+34, 7.788445287802241e+33, 3.894222643901121e+33, 1.9471113219505604e+33,
							9.735556609752802e+32, 4.867778304876401e+32, 2.4338891524382005e+32, 1.2169445762191002e+32, 6.084722881095501e+31,
							3.0423614405477506e+31, 1.5211807202738753e+31, 7.605903601369376e+30, 3.802951800684688e+30, 1.901475900342344e+30,
							9.50737950171172e+29, 4.75368975085586e+29, 2.37684487542793e+29, 1.188422437713965e+29, 5.942112188569825e+28,
							2.9710560942849127e+28, 1.4855280471424563e+28, 7.427640235712282e+27, 3.713820117856141e+27, 1.8569100589280704e+27,
							9.284550294640352e+26, 4.642275147320176e+26, 2.321137573660088e+26, 1.160568786830044e+26, 5.80284393415022e+25,
							2.90142196707511e+25, 1.450710983537555e+25, 7.253554917687775e+24, 3.6267774588438875e+24, 1.8133887294219438e+24,
							9.066943647109719e+23, 4.5334718235548594e+23, 2.2667359117774297e+23, 1.1333679558887149e+23, 5.666839779443574e+22,
							2.833419889721787e+22, 1.4167099448608936e+22, 7.083549724304468e+21, 3.541774862152234e+21, 1.770887431076117e+21,
							8.854437155380585e+20, 4.4272185776902924e+20, 2.2136092888451462e+20, 1.1068046444225731e+20, 5.5340232221128655e+19,
							2.7670116110564327e+19, 1.3835058055282164e+19, 6.917529027641082e+18, 3.458764513820541e+18, 1.7293822569102705e+18,
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 5.404319552844595e+16,
							2.7021597764222976e+16, 1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 1.688849860263936e+15,
							8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14, 1.05553116266496e+14, 5.2776558133248e+13,
							2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 1.649267441664e+12, 8.24633720832e+11,
							4.12316860416e+11, 2.06158430208e+11, 1.03079215104e+11, 5.1539607552e+10, 2.5769803776e+10, 1.2884901888e+10,
							6.442450944e+09, 3.221225472e+09, 1.610612736e+09, 8.05306368e+08, 4.02653184e+08, 2.01326592e+08, 1.00663296e+08, 5.0331648e+07,
							2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 1.572864e+06,
						},
						Counts: []float64{
							120, 119, 118, 117, 116, 115, 114, 113, 112, 111, 110, 109, 108, 107, 106, 105, 104, 103, 102, 101, 100, 99, 98, 97, 96, 95, 94,
							93, 92, 91, 90, 89, 88, 87, 86, 85, 84, 83, 82, 81, 80, 79, 78, 77, 76, 75, 74, 73, 72, 71, 70, 69, 68, 67, 66, 65, 64, 63, 62, 61,
							60, 59, 58, 57, 56, 55, 54, 53, 52, 51, 50, 49, 48, 47, 46, 45, 44, 43, 42, 41, 40, 39, 38, 37, 36, 35, 34, 33, 32, 31, 30, 29, 28,
							27, 26, 25, 24, 23, 22, 21,
						},
						Sum: 100000, Count: 7050, Min: 1048576, Max: 9e+36,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							786432, 393216, 196608, 98304, 49152, 24576, 12288, 6144, 3072, 1536, 768, 384, 192, 96, 48, 24,
							12, 6, 3, 1.5, 0, -1.5, -3, -6, -12, -24, -48, -96, -192, -384, -768, -1536,
							-3072, -6144, -12288, -24576, -49152, -98304, -196608, -393216, -786432, -1.572864e+06, -3.145728e+06, -6.291456e+06,
							-1.2582912e+07, -2.5165824e+07, -5.0331648e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08,
							-1.610612736e+09, -3.221225472e+09, -6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -5.1539607552e+10,
							-1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -1.649267441664e+12,
							-3.298534883328e+12, -6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -5.2776558133248e+13,
							-1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-1.688849860263936e+15, -3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16,
							-2.7021597764222976e+16, -5.404319552844595e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17, -1.7293822569102705e+18, -3.458764513820541e+18,
							-6.917529027641082e+18, -1.3835058055282164e+19, -2.7670116110564327e+19, -5.5340232221128655e+19,
							-1.1068046444225731e+20, -2.2136092888451462e+20, -4.4272185776902924e+20, -8.854437155380585e+20,
							-1.770887431076117e+21, -3.541774862152234e+21, -7.083549724304468e+21, -1.4167099448608936e+22,
							-2.833419889721787e+22, -5.666839779443574e+22, -1.1333679558887149e+23, -2.2667359117774297e+23,
							-4.5334718235548594e+23,
						},
						Counts: []float64{
							20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
							11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
							37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62,
							63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
						},
						Sum: 0, Count: 3372, Min: -6.044629098073146e+23, Max: 1048576,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							-9.066943647109719e+23, -1.8133887294219438e+24, -3.6267774588438875e+24, -7.253554917687775e+24, -1.450710983537555e+25,
							-2.90142196707511e+25, -5.80284393415022e+25, -1.160568786830044e+26, -2.321137573660088e+26, -4.642275147320176e+26,
							-9.284550294640352e+26, -1.8569100589280704e+27, -3.713820117856141e+27, -7.427640235712282e+27, -1.4855280471424563e+28,
							-2.9710560942849127e+28, -5.942112188569825e+28, -1.188422437713965e+29, -2.37684487542793e+29, -4.75368975085586e+29,
							-9.50737950171172e+29, -1.901475900342344e+30, -3.802951800684688e+30, -7.605903601369376e+30, -1.5211807202738753e+31,
							-3.0423614405477506e+31, -6.084722881095501e+31, -1.2169445762191002e+32, -2.4338891524382005e+32, -4.867778304876401e+32,
							-9.735556609752802e+32, -1.9471113219505604e+33, -3.894222643901121e+33, -7.788445287802241e+33, -1.5576890575604483e+34,
							-3.1153781151208966e+34, -6.230756230241793e+34, -1.2461512460483586e+35, -2.4923024920967173e+35, -4.9846049841934345e+35,
							-9.969209968386869e+35,
						},
						Counts: []float64{
							80, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
							110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120,
						},
						Sum: 0, Count: 4100, Min: -9e+36, Max: -6.044629098073146e+23,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
		{
			name: "Exponential histogram with more than 100 buckets, including positive, negative and zero buckets with zero counts",
			histogramDPS: func() pmetric.ExponentialHistogramDataPointSlice {
				histogramDPS := pmetric.NewExponentialHistogramDataPointSlice()
				histogramDP := histogramDPS.AppendEmpty()
				posBucketCounts := make([]uint64, 60)
				for i := range posBucketCounts {
					posBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Positive().BucketCounts().FromRaw(posBucketCounts)
				histogramDP.SetZeroCount(2)
				negBucketCounts := make([]uint64, 60)
				for i := range negBucketCounts {
					negBucketCounts[i] = uint64(i % 5)
				}
				histogramDP.Negative().BucketCounts().FromRaw(negBucketCounts)
				histogramDP.SetSum(1000)
				histogramDP.SetMin(-9e+17)
				histogramDP.SetMax(9e+17)
				histogramDP.SetCount(uint64(3662))
				histogramDP.Attributes().PutStr("label1", "value1")
				return histogramDPS
			}(),
			expectedDatapoints: []dataPoint{
				{
					name: "foo",
					value: &cWMetricHistogram{
						Values: []float64{
							8.646911284551352e+17, 4.323455642275676e+17, 2.161727821137838e+17, 1.080863910568919e+17, 2.7021597764222976e+16,
							1.3510798882111488e+16, 6.755399441055744e+15, 3.377699720527872e+15, 8.44424930131968e+14, 4.22212465065984e+14, 2.11106232532992e+14,
							1.05553116266496e+14, 2.6388279066624e+13, 1.3194139533312e+13, 6.597069766656e+12, 3.298534883328e+12, 8.24633720832e+11, 4.12316860416e+11,
							2.06158430208e+11, 1.03079215104e+11, 2.5769803776e+10, 1.2884901888e+10, 6.442450944e+09, 3.221225472e+09, 8.05306368e+08, 4.02653184e+08,
							2.01326592e+08, 1.00663296e+08, 2.5165824e+07, 1.2582912e+07, 6.291456e+06, 3.145728e+06, 786432, 393216, 196608, 98304, 24576, 12288, 6144, 3072,
							768, 384, 192, 96, 24, 12, 6, 3, 0, -3, -6, -12, -24, -96, -192, -384, -768, -3072, -6144, -12288, -24576, -98304, -196608, -393216, -786432,
							-3.145728e+06, -6.291456e+06, -1.2582912e+07, -2.5165824e+07, -1.00663296e+08, -2.01326592e+08, -4.02653184e+08, -8.05306368e+08, -3.221225472e+09,
							-6.442450944e+09, -1.2884901888e+10, -2.5769803776e+10, -1.03079215104e+11, -2.06158430208e+11, -4.12316860416e+11, -8.24633720832e+11, -3.298534883328e+12,
							-6.597069766656e+12, -1.3194139533312e+13, -2.6388279066624e+13, -1.05553116266496e+14, -2.11106232532992e+14, -4.22212465065984e+14, -8.44424930131968e+14,
							-3.377699720527872e+15, -6.755399441055744e+15, -1.3510798882111488e+16, -2.7021597764222976e+16, -1.080863910568919e+17, -2.161727821137838e+17,
							-4.323455642275676e+17, -8.646911284551352e+17,
						},
						Counts: []float64{
							4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3, 2, 1, 4, 3,
							2, 1, 2, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4,
						},
						Sum: 1000, Count: 242, Min: -9e+17, Max: 9e+17,
					},
					labels: map[string]string{oTellibDimensionKey: instrLibName, "label1": "value1"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(_ *testing.T) {
			exponentialHistogramDatapointSlice := exponentialHistogramDataPointSlice{dmd, tc.histogramDPS}
			emfCalcs := setupEmfCalculators()
			defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
			dps, retained := exponentialHistogramDatapointSlice.CalculateDeltaDatapoints(0, instrLibName, false, emfCalcs)

			assert.True(t, retained)
			assert.Equal(t, 1, exponentialHistogramDatapointSlice.Len())
			assert.Len(t, dps, len(tc.expectedDatapoints))
			for i, expectedDP := range tc.expectedDatapoints {
				assert.Equal(t, expectedDP, dps[i], "datapoint mismatch at index %d", i)
			}
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

	// With instrumentation library name
	labels = createLabels(labelsMap, "cloudwatch-otel")
	expectedLabels[oTellibDimensionKey] = "cloudwatch-otel"
	assert.Equal(t, expectedLabels, labels)
}

func TestGetDataPoints(t *testing.T) {
	logger := zap.NewNop()

	normalDeltaMetricMetadata := generateDeltaMetricMetadata(false, "foo", false)
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
			expectedDatapointSlice: numberDataPointSlice{normalDeltaMetricMetadata, pmetric.NumberDataPointSlice{}},
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
			expectedDatapointSlice: summaryDataPointSlice{normalDeltaMetricMetadata, pmetric.SummaryDataPointSlice{}},
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
		metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", metric.Type(), 0)

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
		metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", pmetric.MetricTypeEmpty, 0)
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

func benchmarkGetAndCalculateDeltaDataPoints(b *testing.B, bucketLength int) {
	generateMetrics := []pmetric.Metrics{
		generateTestGaugeMetric("int-gauge", intValueType),
		generateTestGaugeMetric("int-gauge", doubleValueType),
		generateTestHistogramMetric("histogram"),
		generateTestExponentialHistogramMetric("exponential-histogram"),
		generateTestExponentialHistogramMetricWithSpecifiedNumberOfBuckets(
			"exponential-histogram-buckets-"+strconv.Itoa(bucketLength), bucketLength),
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
			metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", metrics.At(i).Type(), 0)
			dps := getDataPoints(metrics.At(i), metadata, zap.NewNop())

			for i := 0; i < dps.Len(); i++ {
				dps.CalculateDeltaDatapoints(i, "", false, emfCalcs)
			}
		}
	}
}

func BenchmarkGetAndCalculateDeltaDataPointsInclude100Buckets(b *testing.B) {
	benchmarkGetAndCalculateDeltaDataPoints(b, 100)
}

func BenchmarkGetAndCalculateDeltaDataPointsInclude200Buckets(b *testing.B) {
	benchmarkGetAndCalculateDeltaDataPoints(b, 200)
}

func BenchmarkGetAndCalculateDeltaDataPointsInclude300Buckets(b *testing.B) {
	benchmarkGetAndCalculateDeltaDataPoints(b, 300)
}

func BenchmarkGetAndCalculateDeltaDataPointsInclude500Buckets(b *testing.B) {
	benchmarkGetAndCalculateDeltaDataPoints(b, 500)
}
