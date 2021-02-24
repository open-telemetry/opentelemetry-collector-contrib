// Copyright 2020, OpenTelemetry Authors
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
	"reflect"
	"strings"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func generateTestIntGauge(name string) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: name,
			Type: metricspb.MetricDescriptor_GAUGE_INT64,
			Unit: "Count",
			LabelKeys: []*metricspb.LabelKey{
				{Key: "label1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "value1", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_Int64Value{
							Int64Value: 1,
						},
					},
				},
			},
		},
	}
}

func generateTestDoubleGauge(name string) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: name,
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
			Unit: "Count",
			LabelKeys: []*metricspb.LabelKey{
				{Key: "label1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "value1", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_DoubleValue{
							DoubleValue: 0.1,
						},
					},
				},
			},
		},
	}
}

func generateTestIntSum(name string) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: name,
			Type: metricspb.MetricDescriptor_CUMULATIVE_INT64,
			Unit: "Count",
			LabelKeys: []*metricspb.LabelKey{
				{Key: "label1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "value1", HasValue: true},
					{Value: "value2", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_Int64Value{
							Int64Value: 1,
						},
					},
				},
			},
		},
	}
}

func generateTestDoubleSum(name string) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: name,
			Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
			Unit: "Count",
			LabelKeys: []*metricspb.LabelKey{
				{Key: "label1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "value1", HasValue: true},
					{Value: "value2", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_DoubleValue{
							DoubleValue: 0.1,
						},
					},
				},
			},
		},
	}
}

func generateTestDoubleHistogram(name string) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: name,
			Type: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
			Unit: "Seconds",
			LabelKeys: []*metricspb.LabelKey{
				{Key: "label1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "value1", HasValue: true},
					{Value: "value2", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_DistributionValue{
							DistributionValue: &metricspb.DistributionValue{
								Sum:   35.0,
								Count: 18,
								BucketOptions: &metricspb.DistributionValue_BucketOptions{
									Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
										Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
											Bounds: []float64{0, 10},
										},
									},
								},
								Buckets: []*metricspb.DistributionValue_Bucket{
									{
										Count: 5,
									},
									{
										Count: 6,
									},
									{
										Count: 7,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func generateTestSummary(name string) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: name,
			Type: metricspb.MetricDescriptor_SUMMARY,
			Unit: "Seconds",
			LabelKeys: []*metricspb.LabelKey{
				{Key: "label1"},
			},
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: []*metricspb.LabelValue{
					{Value: "value1", HasValue: true},
				},
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_SummaryValue{
							SummaryValue: &metricspb.SummaryValue{
								Sum: &wrappers.DoubleValue{
									Value: 15.0,
								},
								Count: &wrappers.Int64Value{
									Value: 5,
								},
								Snapshot: &metricspb.SummaryValue_Snapshot{
									Count: &wrappers.Int64Value{
										Value: 5,
									},
									Sum: &wrappers.DoubleValue{
										Value: 15.0,
									},
									PercentileValues: []*metricspb.SummaryValue_Snapshot_ValueAtPercentile{
										{
											Percentile: 0.0,
											Value:      1,
										},
										{
											Percentile: 100.0,
											Value:      5,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestIntDataPointSliceAt(t *testing.T) {
	instrLibName := "cloudwatch-otel"
	labels := map[string]string{"label1": "value1"}
	rateKeys := rateKeyParams{
		namespaceKey:  "namespace",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
	}

	testCases := []struct {
		testName           string
		needsCalculateRate bool
		value              interface{}
		calculatedValue    interface{}
	}{
		{
			"no rate calculation",
			false,
			int64(-17),
			float64(-17),
		},
		{
			"w/ 1st rate calculation",
			true,
			int64(1),
			float64(0),
		},
		{
			"w/ 2nd rate calculation",
			true,
			int64(2),
			float64(1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			timestamp := time.Now().UnixNano() / int64(time.Millisecond)
			testDPS := pdata.NewIntDataPointSlice()
			testDPS.Resize(1)
			testDP := testDPS.At(0)
			testDP.SetValue(tc.value.(int64))
			testDP.LabelsMap().InitFromMap(labels)

			dps := IntDataPointSlice{
				instrLibName,
				rateCalculationMetadata{
					tc.needsCalculateRate,
					rateKeys,
					timestamp,
				},
				testDPS,
			}

			expectedDP := DataPoint{
				Value: tc.calculatedValue,
				Labels: map[string]string{
					oTellibDimensionKey: instrLibName,
					"label1":            "value1",
				},
			}

			assert.Equal(t, 1, dps.Len())
			dp := dps.At(0)
			if strings.Contains(tc.testName, "2nd rate") {
				assert.True(t, (expectedDP.Value.(float64)-dp.Value.(float64)) < 0.01)
			} else {
				assert.Equal(t, expectedDP, dp)
			}
			// sleep 1s for verifying the cumulative metric delta rate
			time.Sleep(1000 * time.Millisecond)
		})
	}
}

func TestDoubleDataPointSliceAt(t *testing.T) {
	instrLibName := "cloudwatch-otel"
	labels := map[string]string{"label1": "value1"}
	rateKeys := rateKeyParams{
		namespaceKey:  "namespace",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
	}

	testCases := []struct {
		testName           string
		needsCalculateRate bool
		value              interface{}
		calculatedValue    interface{}
	}{
		{
			"no rate calculation",
			false,
			float64(0.3),
			float64(0.3),
		},
		{
			"w/ 1st rate calculation",
			true,
			float64(0.4),
			float64(0.0),
		},
		{
			"w/ 2nd rate calculation",
			true,
			float64(0.5),
			float64(0.1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			timestamp := time.Now().UnixNano() / int64(time.Millisecond)
			testDPS := pdata.NewDoubleDataPointSlice()
			testDPS.Resize(1)
			testDP := testDPS.At(0)
			testDP.SetValue(tc.value.(float64))
			testDP.LabelsMap().InitFromMap(labels)

			dps := DoubleDataPointSlice{
				instrLibName,
				rateCalculationMetadata{
					tc.needsCalculateRate,
					rateKeys,
					timestamp,
				},
				testDPS,
			}

			expectedDP := DataPoint{
				Value: tc.calculatedValue,
				Labels: map[string]string{
					oTellibDimensionKey: instrLibName,
					"label1":            "value1",
				},
			}

			assert.Equal(t, 1, dps.Len())
			dp := dps.At(0)
			if strings.Contains(tc.testName, "2nd rate") {
				assert.True(t, (expectedDP.Value.(float64)-dp.Value.(float64)) < 0.002)
			} else {
				assert.Equal(t, expectedDP, dp)
			}
			// sleep 10ms for verifying the cumulative metric delta rate
			time.Sleep(1000 * time.Millisecond)
		})
	}
}

func TestDoubleHistogramDataPointSliceAt(t *testing.T) {
	instrLibName := "cloudwatch-otel"
	labels := map[string]string{"label1": "value1"}

	testDPS := pdata.NewDoubleHistogramDataPointSlice()
	testDPS.Resize(1)
	testDP := testDPS.At(0)
	testDP.SetCount(uint64(17))
	testDP.SetSum(float64(17.13))
	testDP.SetBucketCounts([]uint64{1, 2, 3})
	testDP.SetExplicitBounds([]float64{1, 2, 3})
	testDP.LabelsMap().InitFromMap(labels)

	dps := DoubleHistogramDataPointSlice{
		instrLibName,
		testDPS,
	}

	expectedDP := DataPoint{
		Value: &CWMetricStats{
			Sum:   17.13,
			Count: 17,
		},
		Labels: map[string]string{
			oTellibDimensionKey: instrLibName,
			"label1":            "value1",
		},
	}

	assert.Equal(t, 1, dps.Len())
	dp := dps.At(0)
	assert.Equal(t, expectedDP, dp)
}

func TestDoubleSummaryDataPointSliceAt(t *testing.T) {
	instrLibName := "cloudwatch-otel"
	labels := map[string]string{"label1": "value1"}

	testDPS := pdata.NewDoubleSummaryDataPointSlice()
	testDPS.Resize(1)
	testDP := testDPS.At(0)
	testDP.SetCount(uint64(17))
	testDP.SetSum(float64(17.13))
	testDP.QuantileValues().Resize(2)
	testQuantileValue := testDP.QuantileValues().At(0)
	testQuantileValue.SetQuantile(0)
	testQuantileValue.SetValue(float64(1))
	testQuantileValue = testDP.QuantileValues().At(1)
	testQuantileValue.SetQuantile(100)
	testQuantileValue.SetValue(float64(5))
	testDP.LabelsMap().InitFromMap(labels)

	dps := DoubleSummaryDataPointSlice{
		instrLibName,
		testDPS,
	}

	expectedDP := DataPoint{
		Value: &CWMetricStats{
			Max:   5,
			Min:   1,
			Count: 17,
			Sum:   17.13,
		},
		Labels: map[string]string{
			oTellibDimensionKey: instrLibName,
			"label1":            "value1",
		},
	}

	assert.Equal(t, 1, dps.Len())
	dp := dps.At(0)
	assert.Equal(t, expectedDP, dp)
}

func TestCreateLabels(t *testing.T) {
	expectedLabels := map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	}
	labelsMap := pdata.NewStringMap().InitFromMap(expectedLabels)

	labels := createLabels(labelsMap, noInstrumentationLibraryName)
	assert.Equal(t, expectedLabels, labels)

	// With isntrumentation library name
	labels = createLabels(labelsMap, "cloudwatch-otel")
	expectedLabels[oTellibDimensionKey] = "cloudwatch-otel"
	assert.Equal(t, expectedLabels, labels)
}

func TestCalculateRate(t *testing.T) {
	intRateKey := "foo"
	doubleRateKey := "bar"
	time1 := time.Now().UnixNano() / int64(time.Millisecond)
	time2 := time.Unix(0, time1*int64(time.Millisecond)).Add(time.Second*10).UnixNano() / int64(time.Millisecond)
	time3 := time.Unix(0, time2*int64(time.Millisecond)).Add(time.Second*10).UnixNano() / int64(time.Millisecond)

	intVal1 := float64(0)
	intVal2 := float64(10)
	intVal3 := float64(200)
	doubleVal1 := 0.0
	doubleVal2 := 5.0
	doubleVal3 := 15.1

	rate := calculateRate(intRateKey, intVal1, time1)
	assert.Equal(t, float64(0), rate)
	rate = calculateRate(doubleRateKey, doubleVal1, time1)
	assert.Equal(t, float64(0), rate)

	rate = calculateRate(intRateKey, intVal2, time2)
	assert.Equal(t, float64(1), rate)
	rate = calculateRate(doubleRateKey, doubleVal2, time2)
	assert.Equal(t, 0.5, rate)

	// Test change of data type
	rate = calculateRate(intRateKey, doubleVal3, time3)
	assert.Equal(t, float64(0.51), rate)
	rate = calculateRate(doubleRateKey, intVal3, time3)
	assert.Equal(t, float64(19.5), rate)
}

func TestGetDataPoints(t *testing.T) {
	metadata := CWMetricMetadata{
		Namespace:                  "Namespace",
		TimestampMs:                time.Now().UnixNano() / int64(time.Millisecond),
		LogGroup:                   "log-group",
		LogStream:                  "log-stream",
		InstrumentationLibraryName: "cloudwatch-otel",
	}

	testCases := []struct {
		testName           string
		metric             *metricspb.Metric
		expectedDataPoints DataPoints
	}{
		{
			"Int gauge",
			generateTestIntGauge("foo"),
			IntDataPointSlice{
				metadata.InstrumentationLibraryName,
				rateCalculationMetadata{
					false,
					rateKeyParams{
						namespaceKey:  metadata.Namespace,
						metricNameKey: "foo",
						logGroupKey:   metadata.LogGroup,
						logStreamKey:  metadata.LogStream,
					},
					metadata.TimestampMs,
				},
				pdata.IntDataPointSlice{},
			},
		},
		{
			"Double gauge",
			generateTestDoubleGauge("foo"),
			DoubleDataPointSlice{
				metadata.InstrumentationLibraryName,
				rateCalculationMetadata{
					false,
					rateKeyParams{
						namespaceKey:  metadata.Namespace,
						metricNameKey: "foo",
						logGroupKey:   metadata.LogGroup,
						logStreamKey:  metadata.LogStream,
					},
					metadata.TimestampMs,
				},
				pdata.DoubleDataPointSlice{},
			},
		},
		{
			"Int sum",
			generateTestIntSum("foo"),
			IntDataPointSlice{
				metadata.InstrumentationLibraryName,
				rateCalculationMetadata{
					true,
					rateKeyParams{
						namespaceKey:  metadata.Namespace,
						metricNameKey: "foo",
						logGroupKey:   metadata.LogGroup,
						logStreamKey:  metadata.LogStream,
					},
					metadata.TimestampMs,
				},
				pdata.IntDataPointSlice{},
			},
		},
		{
			"Double sum",
			generateTestDoubleSum("foo"),
			DoubleDataPointSlice{
				metadata.InstrumentationLibraryName,
				rateCalculationMetadata{
					true,
					rateKeyParams{
						namespaceKey:  metadata.Namespace,
						metricNameKey: "foo",
						logGroupKey:   metadata.LogGroup,
						logStreamKey:  metadata.LogStream,
					},
					metadata.TimestampMs,
				},
				pdata.DoubleDataPointSlice{},
			},
		},
		{
			"Double histogram",
			generateTestDoubleHistogram("foo"),
			DoubleHistogramDataPointSlice{
				metadata.InstrumentationLibraryName,
				pdata.DoubleHistogramDataPointSlice{},
			},
		},
		{
			"Summary",
			generateTestSummary("foo"),
			DoubleSummaryDataPointSlice{
				metadata.InstrumentationLibraryName,
				pdata.DoubleSummaryDataPointSlice{},
			},
		},
	}

	for _, tc := range testCases {
		oc := consumerdata.MetricsData{
			Metrics: []*metricspb.Metric{tc.metric},
		}

		// Retrieve *pdata.Metric
		rm := internaldata.OCToMetrics(oc).ResourceMetrics().At(0)
		metric := rm.InstrumentationLibraryMetrics().At(0).Metrics().At(0)

		logger := zap.NewNop()

		expectedLabels := pdata.NewStringMap().InitFromMap(map[string]string{"label1": "value1"})

		t.Run(tc.testName, func(t *testing.T) {
			dps := getDataPoints(&metric, metadata, logger)
			assert.NotNil(t, dps)
			assert.Equal(t, reflect.TypeOf(tc.expectedDataPoints), reflect.TypeOf(dps))
			switch convertedDPS := dps.(type) {
			case IntDataPointSlice:
				expectedDPS := tc.expectedDataPoints.(IntDataPointSlice)
				assert.Equal(t, metadata.InstrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, expectedDPS.rateCalculationMetadata, convertedDPS.rateCalculationMetadata)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.IntDataPointSlice.At(0)
				assert.Equal(t, int64(1), dp.Value())
				assert.Equal(t, expectedLabels, dp.LabelsMap())
			case DoubleDataPointSlice:
				expectedDPS := tc.expectedDataPoints.(DoubleDataPointSlice)
				assert.Equal(t, metadata.InstrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, expectedDPS.rateCalculationMetadata, convertedDPS.rateCalculationMetadata)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.DoubleDataPointSlice.At(0)
				assert.Equal(t, 0.1, dp.Value())
				assert.Equal(t, expectedLabels, dp.LabelsMap())
			case DoubleHistogramDataPointSlice:
				assert.Equal(t, metadata.InstrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.DoubleHistogramDataPointSlice.At(0)
				assert.Equal(t, 35.0, dp.Sum())
				assert.Equal(t, uint64(18), dp.Count())
				assert.Equal(t, []float64{0, 10}, dp.ExplicitBounds())
				assert.Equal(t, expectedLabels, dp.LabelsMap())
			case DoubleSummaryDataPointSlice:
				assert.Equal(t, metadata.InstrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.DoubleSummaryDataPointSlice.At(0)
				assert.Equal(t, 15.0, dp.Sum())
				assert.Equal(t, uint64(5), dp.Count())
				assert.Equal(t, 2, dp.QuantileValues().Len())
				assert.Equal(t, float64(1), dp.QuantileValues().At(0).Value())
				assert.Equal(t, float64(5), dp.QuantileValues().At(1).Value())
			}
		})
	}

	t.Run("Unhandled metric type", func(t *testing.T) {
		metric := pdata.NewMetric()
		metric.SetName("foo")
		metric.SetUnit("Count")
		metric.SetDataType(pdata.MetricDataTypeIntHistogram)

		obs, logs := observer.New(zap.WarnLevel)
		logger := zap.New(obs)

		dps := getDataPoints(&metric, metadata, logger)
		assert.Nil(t, dps)

		// Test output warning logs
		expectedLogs := []observer.LoggedEntry{
			{
				Entry: zapcore.Entry{Level: zap.WarnLevel, Message: "Unhandled metric data type."},
				Context: []zapcore.Field{
					zap.String("DataType", "IntHistogram"),
					zap.String("Name", "foo"),
					zap.String("Unit", "Count"),
				},
			},
		}
		assert.Equal(t, 1, logs.Len())
		assert.Equal(t, expectedLogs, logs.AllUntimed())
	})

	t.Run("Nil metric", func(t *testing.T) {
		dps := getDataPoints(nil, metadata, zap.NewNop())
		assert.Nil(t, dps)
	})
}

func BenchmarkGetDataPoints(b *testing.B) {
	oc := consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			generateTestIntGauge("int-gauge"),
			generateTestDoubleGauge("double-gauge"),
			generateTestIntSum("int-sum"),
			generateTestDoubleSum("double-sum"),
			generateTestDoubleHistogram("double-histogram"),
			generateTestSummary("summary"),
		},
	}
	rms := internaldata.OCToMetrics(oc).ResourceMetrics()
	metrics := rms.At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	numMetrics := metrics.Len()

	metadata := CWMetricMetadata{
		Namespace:                  "Namespace",
		TimestampMs:                int64(1596151098037),
		LogGroup:                   "log-group",
		LogStream:                  "log-stream",
		InstrumentationLibraryName: "cloudwatch-otel",
	}

	logger := zap.NewNop()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < numMetrics; i++ {
			metric := metrics.At(i)
			getDataPoints(&metric, metadata, logger)
		}
	}
}

func TestGetSortedLabelsEquals(t *testing.T) {
	labelMap1 := make(map[string]string)
	labelMap1["k1"] = "v1"
	labelMap1["k2"] = "v2"

	labelMap2 := make(map[string]string)
	labelMap2["k2"] = "v2"
	labelMap2["k1"] = "v1"

	sortedLabels1 := getSortedLabels(labelMap1)
	sortedLabels2 := getSortedLabels(labelMap2)

	rateKeyParams1 := rateKeyParams{
		namespaceKey:  "namespace",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
		labels:        sortedLabels1,
	}
	rateKeyParams2 := rateKeyParams{
		namespaceKey:  "namespace",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
		labels:        sortedLabels2,
	}
	assert.Equal(t, rateKeyParams1, rateKeyParams2)
}

func TestGetSortedLabelsNotEqual(t *testing.T) {
	labelMap1 := make(map[string]string)
	labelMap1["k1"] = "v1"
	labelMap1["k2"] = "v2"

	labelMap2 := make(map[string]string)
	labelMap2["k2"] = "v2"
	labelMap2["k1"] = "v3"

	sortedLabels1 := getSortedLabels(labelMap1)
	sortedLabels2 := getSortedLabels(labelMap2)

	rateKeyParams1 := rateKeyParams{
		namespaceKey:  "namespace",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
		labels:        sortedLabels1,
	}
	rateKeyParams2 := rateKeyParams{
		namespaceKey:  "namespace",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
		labels:        sortedLabels2,
	}
	assert.NotEqual(t, rateKeyParams1, rateKeyParams2)
}

func TestGetSortedLabelsNotEqualOnPram(t *testing.T) {
	labelMap1 := make(map[string]string)
	labelMap1["k1"] = "v1"
	labelMap1["k2"] = "v2"

	labelMap2 := make(map[string]string)
	labelMap2["k2"] = "v2"
	labelMap2["k1"] = "v1"

	sortedLabels1 := getSortedLabels(labelMap1)
	sortedLabels2 := getSortedLabels(labelMap2)

	rateKeyParams1 := rateKeyParams{
		namespaceKey:  "namespaceA",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
		labels:        sortedLabels1,
	}
	rateKeyParams2 := rateKeyParams{
		namespaceKey:  "namespaceB",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
		labels:        sortedLabels2,
	}
	assert.NotEqual(t, rateKeyParams1, rateKeyParams2)
}

func TestGetSortedLabelsNotEqualOnEmptyLabel(t *testing.T) {
	rateKeyParams1 := rateKeyParams{
		namespaceKey:  "namespaceA",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
	}
	rateKeyParams2 := rateKeyParams{
		namespaceKey:  "namespaceA",
		metricNameKey: "foo",
		logGroupKey:   "log-group",
		logStreamKey:  "log-stream",
	}
	assert.Equal(t, rateKeyParams1, rateKeyParams2)
}
