// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

func setupDataPointCache() {
	deltaMetricCalculator = aws.NewFloat64DeltaCalculator()
	summaryMetricCalculator = aws.NewMetricCalculator(calculateSummaryDelta)
}

func TestCalculateDeltaDatapoints_NumberDataPointSlice(t *testing.T) {
	for _, retainInitialValueOfDeltaMetric := range []bool{true, false} {
		setupDataPointCache()

		testCases := []struct {
			name              string
			adjustToDelta     bool
			metricName        string
			metricValue       interface{}
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

				deltaMetricMetadata := generateDeltaMetricMetadata(tc.adjustToDelta, tc.metricName, retainInitialValueOfDeltaMetric)
				numberDatapointSlice := numberDataPointSlice{deltaMetricMetadata, numberDPS}

				// When calculate the delta datapoints for number datapoint
				dps, retained := numberDatapointSlice.CalculateDeltaDatapoints(0, instrLibName, false)

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
	deltaMetricMetadata := generateDeltaMetricMetadata(false, "foo", false)

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
			histogramDatapointSlice := histogramDataPointSlice{deltaMetricMetadata, tc.histogramDPS}

			// When calculate the delta datapoints for histograms
			dps, retained := histogramDatapointSlice.CalculateDeltaDatapoints(0, instrLibName, false)

			// Then receiving the following datapoint with an expected length
			assert.True(t, retained)
			assert.Equal(t, 1, histogramDatapointSlice.Len())
			assert.Equal(t, tc.expectedDatapoint, dps[0])
		})
	}

}

func TestCalculateDeltaDatapoints_SummaryDataPointSlice(t *testing.T) {
	for _, retainInitialValueOfDeltaMetric := range []bool{true, false} {
		deltaMetricMetadata := generateDeltaMetricMetadata(true, "foo", retainInitialValueOfDeltaMetric)

		testCases := []struct {
			name               string
			summaryMetricValue map[string]interface{}
			expectedDatapoint  []dataPoint
			expectedRetained   bool
		}{
			{
				name:               fmt.Sprintf("Detailed summary with 1st delta sum count calculation retainInitialValueOfDeltaMetric=%t", retainInitialValueOfDeltaMetric),
				summaryMetricValue: map[string]interface{}{"sum": float64(17.3), "count": uint64(17), "firstQuantile": float64(1), "secondQuantile": float64(5)},
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
				summaryMetricValue: map[string]interface{}{"sum": float64(100), "count": uint64(25), "firstQuantile": float64(1), "secondQuantile": float64(5)},
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
				summaryMetricValue: map[string]interface{}{"sum": float64(120), "count": uint64(26), "firstQuantile": float64(1), "secondQuantile": float64(5)},
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

				summaryDatapointSlice := summaryDataPointSlice{deltaMetricMetadata, summaryDPS}

				// When calculate the delta datapoints for sum and count in summary
				dps, retained := summaryDatapointSlice.CalculateDeltaDatapoints(0, "", true)

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

func TestCreateLabels(t *testing.T) {
	expectedLabels := map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	}
	labelsMap := pcommon.NewMap()
	assert.NoError(t, labelsMap.FromRaw(map[string]interface{}{
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
		expectedAttributes     map[string]interface{}
	}{
		{
			name:                   "Int gauge",
			isPrometheusMetrics:    false,
			metric:                 generateTestGaugeMetric("foo", intValueType),
			expectedDatapointSlice: numberDataPointSlice{normalDeltraMetricMetadata, pmetric.NumberDataPointSlice{}},
			expectedAttributes:     map[string]interface{}{"label1": "value1"},
		},
		{
			name:                   "Double sum",
			isPrometheusMetrics:    false,
			metric:                 generateTestSumMetric("foo", doubleValueType),
			expectedDatapointSlice: numberDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.NumberDataPointSlice{}},
			expectedAttributes:     map[string]interface{}{"label1": "value1"},
		},
		{
			name:                   "Histogram",
			isPrometheusMetrics:    false,
			metric:                 generateTestHistogramMetric("foo"),
			expectedDatapointSlice: histogramDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.HistogramDataPointSlice{}},
			expectedAttributes:     map[string]interface{}{"label1": "value1"},
		},
		{
			name:                   "Summary from SDK",
			isPrometheusMetrics:    false,
			metric:                 generateTestSummaryMetric("foo"),
			expectedDatapointSlice: summaryDataPointSlice{normalDeltraMetricMetadata, pmetric.SummaryDataPointSlice{}},
			expectedAttributes:     map[string]interface{}{"label1": "value1"},
		},
		{
			name:                   "Summary from Prometheus",
			isPrometheusMetrics:    true,
			metric:                 generateTestSummaryMetric("foo"),
			expectedDatapointSlice: summaryDataPointSlice{cumulativeDeltaMetricMetadata, pmetric.SummaryDataPointSlice{}},
			expectedAttributes:     map[string]interface{}{"label1": "value1"},
		},
	}

	for _, tc := range testCases {
		rm := tc.metric.ResourceMetrics().At(0)
		metrics := rm.ScopeMetrics().At(0).Metrics()
		metric := metrics.At(metrics.Len() - 1)
		metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", metric.Type())

		t.Run(tc.name, func(t *testing.T) {
			setupDataPointCache()

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
		generateTestSumMetric("int-sum", intValueType),
		generateTestSumMetric("double-sum", doubleValueType),
		generateTestSummaryMetric("summary"),
	}

	finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)
	rms := finalOtelMetrics.ResourceMetrics()
	metrics := rms.At(0).ScopeMetrics().At(0).Metrics()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < metrics.Len(); i++ {
			metadata := generateTestMetricMetadata("namespace", time.Now().UnixNano()/int64(time.Millisecond), "log-group", "log-stream", "cloudwatch-otel", metrics.At(i).Type())
			dps := getDataPoints(metrics.At(i), metadata, zap.NewNop())

			for i := 0; i < dps.Len(); i++ {
				dps.CalculateDeltaDatapoints(i, "", false)
			}
		}
	}
}
