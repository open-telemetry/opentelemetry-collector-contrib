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
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	aws "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
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

func generateTestIntSum(name string) []*metricspb.Metric {
	var metrics []*metricspb.Metric
	for i := 0; i < 2; i++ {
		metrics = append(metrics, &metricspb.Metric{
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
								Int64Value: int64(i),
							},
						},
					},
				},
			},
		})
	}
	return metrics
}

func generateTestDoubleSum(name string) []*metricspb.Metric {
	var metrics []*metricspb.Metric
	for i := 0; i < 2; i++ {
		metrics = append(metrics, &metricspb.Metric{
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
								DoubleValue: float64(i) * 0.1,
							},
						},
					},
				},
			},
		})
	}
	return metrics
}

func generateTestHistogram(name string) *metricspb.Metric {
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

func generateTestSummary(name string) []*metricspb.Metric {
	var metrics []*metricspb.Metric
	for i := 0; i < 2; i++ {
		metrics = append(metrics, &metricspb.Metric{
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
										Value: float64(i * 15.0),
									},
									Count: &wrappers.Int64Value{
										Value: int64(i * 5),
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
		})
	}
	return metrics
}

func setupDataPointCache() {
	deltaMetricCalculator = aws.NewFloat64DeltaCalculator()
	summaryMetricCalculator = aws.NewMetricCalculator(calculateSummaryDelta)
}

func TestIntDataPointSliceAt(t *testing.T) {
	setupDataPointCache()

	instrLibName := "cloudwatch-otel"
	labels := map[string]pdata.AttributeValue{"label": pdata.NewAttributeValueString("value")}

	testDeltaCases := []struct {
		testName        string
		adjustToDelta   bool
		value           interface{}
		calculatedValue interface{}
	}{
		{
			"w/ 1st delta calculation",
			true,
			int64(-17),
			float64(0),
		},
		{
			"w/ 2nd delta calculation",
			true,
			int64(1),
			float64(18),
		},
		{
			"w/o delta calculation",
			false,
			int64(10),
			float64(10),
		},
	}

	for i, tc := range testDeltaCases {
		t.Run(tc.testName, func(t *testing.T) {
			testDPS := pdata.NewNumberDataPointSlice()
			testDP := testDPS.AppendEmpty()
			testDP.SetIntVal(tc.value.(int64))
			testDP.Attributes().InitFromMap(labels)

			dps := numberDataPointSlice{
				instrLibName,
				deltaMetricMetadata{
					tc.adjustToDelta,
					"foo",
					0,
					"namespace",
					"log-group",
					"log-stream",
				},
				testDPS,
			}

			expectedDP := dataPoint{
				value: tc.calculatedValue,
				labels: map[string]string{
					oTellibDimensionKey: instrLibName,
					"label":             "value",
				},
			}

			assert.Equal(t, 1, dps.Len())
			dp, retained := dps.At(0)
			assert.Equal(t, i > 0, retained)
			if retained {
				assert.Equal(t, expectedDP.labels, dp.labels)
				assert.InDelta(t, expectedDP.value.(float64), dp.value.(float64), 0.02)
			}
		})
	}
}

func TestDoubleDataPointSliceAt(t *testing.T) {
	setupDataPointCache()

	instrLibName := "cloudwatch-otel"
	labels := map[string]pdata.AttributeValue{"label1": pdata.NewAttributeValueString("value1")}

	testDeltaCases := []struct {
		testName        string
		adjustToDelta   bool
		value           interface{}
		calculatedValue interface{}
	}{
		{
			"w/ 1st delta calculation",
			true,
			0.4,
			0.4,
		},
		{
			"w/ 2nd delta calculation",
			true,
			0.8,
			0.4,
		},
		{
			"w/o delta calculation",
			false,
			0.5,
			0.5,
		},
	}

	for i, tc := range testDeltaCases {
		t.Run(tc.testName, func(t *testing.T) {
			testDPS := pdata.NewNumberDataPointSlice()
			testDP := testDPS.AppendEmpty()
			testDP.SetDoubleVal(tc.value.(float64))
			testDP.Attributes().InitFromMap(labels)

			dps := numberDataPointSlice{
				instrLibName,
				deltaMetricMetadata{
					tc.adjustToDelta,
					"foo",
					0,
					"namespace",
					"log-group",
					"log-stream",
				},
				testDPS,
			}

			assert.Equal(t, 1, dps.Len())
			dp, retained := dps.At(0)
			assert.Equal(t, i > 0, retained)
			if retained {
				assert.InDelta(t, tc.calculatedValue.(float64), dp.value.(float64), 0.002)
			}
		})
	}
}

func TestHistogramDataPointSliceAt(t *testing.T) {
	instrLibName := "cloudwatch-otel"
	labels := map[string]pdata.AttributeValue{"label1": pdata.NewAttributeValueString("value1")}

	testDPS := pdata.NewHistogramDataPointSlice()
	testDP := testDPS.AppendEmpty()
	testDP.SetCount(uint64(17))
	testDP.SetSum(17.13)
	testDP.SetBucketCounts([]uint64{1, 2, 3})
	testDP.SetExplicitBounds([]float64{1, 2, 3})
	testDP.Attributes().InitFromMap(labels)

	dps := histogramDataPointSlice{
		instrLibName,
		testDPS,
	}

	expectedDP := dataPoint{
		value: &cWMetricStats{
			Sum:   17.13,
			Count: 17,
		},
		labels: map[string]string{
			oTellibDimensionKey: instrLibName,
			"label1":            "value1",
		},
	}

	assert.Equal(t, 1, dps.Len())
	dp, _ := dps.At(0)
	assert.Equal(t, expectedDP, dp)
}

func TestSummaryDataPointSliceAt(t *testing.T) {
	setupDataPointCache()

	instrLibName := "cloudwatch-otel"
	labels := map[string]pdata.AttributeValue{"label1": pdata.NewAttributeValueString("value1")}
	metadataTimeStamp := time.Now().UnixNano() / int64(time.Millisecond)

	testCases := []struct {
		testName           string
		inputSumCount      []interface{}
		calculatedSumCount []interface{}
	}{
		{
			"1st summary count calculation",
			[]interface{}{17.3, uint64(17)},
			[]interface{}{float64(0), uint64(0)},
		},
		{
			"2nd summary count calculation",
			[]interface{}{float64(100), uint64(25)},
			[]interface{}{82.7, uint64(8)},
		},
		{
			"3rd summary count calculation",
			[]interface{}{float64(120), uint64(26)},
			[]interface{}{float64(20), uint64(1)},
		},
	}

	for i, tt := range testCases {
		t.Run(tt.testName, func(t *testing.T) {
			testDPS := pdata.NewSummaryDataPointSlice()
			testDP := testDPS.AppendEmpty()
			testDP.SetSum(tt.inputSumCount[0].(float64))
			testDP.SetCount(tt.inputSumCount[1].(uint64))

			testDP.QuantileValues().EnsureCapacity(2)
			testQuantileValue := testDP.QuantileValues().AppendEmpty()
			testQuantileValue.SetQuantile(0)
			testQuantileValue.SetValue(float64(1))
			testQuantileValue = testDP.QuantileValues().AppendEmpty()
			testQuantileValue.SetQuantile(100)
			testQuantileValue.SetValue(float64(5))
			testDP.Attributes().InitFromMap(labels)

			dps := summaryDataPointSlice{
				instrLibName,
				deltaMetricMetadata{
					true,
					"foo",
					metadataTimeStamp,
					"namespace",
					"log-group",
					"log-stream",
				},
				testDPS,
			}

			expectedDP := dataPoint{
				value: &cWMetricStats{
					Max:   5,
					Min:   1,
					Sum:   tt.calculatedSumCount[0].(float64),
					Count: tt.calculatedSumCount[1].(uint64),
				},
				labels: map[string]string{
					oTellibDimensionKey: instrLibName,
					"label1":            "value1",
				},
			}

			assert.Equal(t, 1, dps.Len())
			dp, retained := dps.At(0)
			assert.Equal(t, i > 0, retained)
			if retained {
				expectedMetricStats := expectedDP.value.(*cWMetricStats)
				actualMetricsStats := dp.value.(*cWMetricStats)
				assert.Equal(t, expectedDP.labels, dp.labels)
				assert.Equal(t, expectedMetricStats.Max, actualMetricsStats.Max)
				assert.Equal(t, expectedMetricStats.Min, actualMetricsStats.Min)
				assert.InDelta(t, expectedMetricStats.Count, actualMetricsStats.Count, 0.1)
				assert.InDelta(t, expectedMetricStats.Sum, actualMetricsStats.Sum, 0.02)
			}
		})
	}
}

func TestCreateLabels(t *testing.T) {
	expectedLabels := map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	}
	labelsMap := pdata.NewAttributeMapFromMap(map[string]pdata.AttributeValue{
		"a": pdata.NewAttributeValueString("A"),
		"b": pdata.NewAttributeValueString("B"),
		"c": pdata.NewAttributeValueString("C"),
	})

	labels := createLabels(labelsMap, noInstrumentationLibraryName)
	assert.Equal(t, expectedLabels, labels)

	// With isntrumentation library name
	labels = createLabels(labelsMap, "cloudwatch-otel")
	expectedLabels[oTellibDimensionKey] = "cloudwatch-otel"
	assert.Equal(t, expectedLabels, labels)
}

func TestGetDataPoints(t *testing.T) {
	metadata := cWMetricMetadata{
		groupedMetricMetadata: groupedMetricMetadata{
			namespace:   "namespace",
			timestampMs: time.Now().UnixNano() / int64(time.Millisecond),
			logGroup:    "log-group",
			logStream:   "log-stream",
		},
		instrumentationLibraryName: "cloudwatch-otel",
	}

	dmm := deltaMetricMetadata{
		false,
		"foo",
		metadata.timestampMs,
		"namespace",
		"log-group",
		"log-stream",
	}
	cumulativeDmm := deltaMetricMetadata{
		true,
		"foo",
		metadata.timestampMs,
		"namespace",
		"log-group",
		"log-stream",
	}
	testCases := []struct {
		testName            string
		isPrometheusMetrics bool
		metric              *metricspb.Metric
		expectedDataPoints  dataPoints
	}{
		{
			"Int gauge",
			false,
			generateTestIntGauge("foo"),
			numberDataPointSlice{
				metadata.instrumentationLibraryName,
				dmm,
				pdata.NumberDataPointSlice{},
			},
		},
		{
			"Double gauge",
			false,
			generateTestDoubleGauge("foo"),
			numberDataPointSlice{
				metadata.instrumentationLibraryName,
				dmm,
				pdata.NumberDataPointSlice{},
			},
		},
		{
			"Int sum",
			false,
			generateTestIntSum("foo")[1],
			numberDataPointSlice{
				metadata.instrumentationLibraryName,
				cumulativeDmm,
				pdata.NumberDataPointSlice{},
			},
		},
		{
			"Double sum",
			false,
			generateTestDoubleSum("foo")[1],
			numberDataPointSlice{
				metadata.instrumentationLibraryName,
				cumulativeDmm,
				pdata.NumberDataPointSlice{},
			},
		},
		{
			"Double histogram",
			false,
			generateTestHistogram("foo"),
			histogramDataPointSlice{
				metadata.instrumentationLibraryName,
				pdata.HistogramDataPointSlice{},
			},
		},
		{
			"Summary from SDK",
			false,
			generateTestSummary("foo")[1],
			summaryDataPointSlice{
				metadata.instrumentationLibraryName,
				dmm,
				pdata.SummaryDataPointSlice{},
			},
		},
		{
			"Summary from Prometheus",
			true,
			generateTestSummary("foo")[1],
			summaryDataPointSlice{
				metadata.instrumentationLibraryName,
				cumulativeDmm,
				pdata.SummaryDataPointSlice{},
			},
		},
	}

	for _, tc := range testCases {
		ocMetrics := []*metricspb.Metric{tc.metric}

		// Retrieve *pdata.Metric
		rm := internaldata.OCToMetrics(nil, nil, ocMetrics).ResourceMetrics().At(0)
		metric := rm.InstrumentationLibraryMetrics().At(0).Metrics().At(0)

		logger := zap.NewNop()

		expectedAttributes := pdata.NewAttributeMapFromMap(map[string]pdata.AttributeValue{"label1": pdata.NewAttributeValueString("value1")})

		t.Run(tc.testName, func(t *testing.T) {
			setupDataPointCache()

			if tc.isPrometheusMetrics {
				metadata.receiver = prometheusReceiver
			} else {
				metadata.receiver = ""
			}
			dps := getDataPoints(&metric, metadata, logger)
			assert.NotNil(t, dps)
			assert.Equal(t, reflect.TypeOf(tc.expectedDataPoints), reflect.TypeOf(dps))
			switch convertedDPS := dps.(type) {
			case numberDataPointSlice:
				expectedDPS := tc.expectedDataPoints.(numberDataPointSlice)
				assert.Equal(t, metadata.instrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, expectedDPS.deltaMetricMetadata, convertedDPS.deltaMetricMetadata)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.NumberDataPointSlice.At(0)
				switch dp.Type() {
				case pdata.MetricValueTypeDouble:
					assert.Equal(t, 0.1, dp.DoubleVal())
				case pdata.MetricValueTypeInt:
					assert.Equal(t, int64(1), dp.IntVal())
				}
				assert.Equal(t, expectedAttributes, dp.Attributes())
			case histogramDataPointSlice:
				assert.Equal(t, metadata.instrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.HistogramDataPointSlice.At(0)
				assert.Equal(t, 35.0, dp.Sum())
				assert.Equal(t, uint64(18), dp.Count())
				assert.Equal(t, []float64{0, 10}, dp.ExplicitBounds())
				assert.Equal(t, expectedAttributes, dp.Attributes())
			case summaryDataPointSlice:
				expectedDPS := tc.expectedDataPoints.(summaryDataPointSlice)
				assert.Equal(t, metadata.instrumentationLibraryName, convertedDPS.instrumentationLibraryName)
				assert.Equal(t, expectedDPS.deltaMetricMetadata, convertedDPS.deltaMetricMetadata)
				assert.Equal(t, 1, convertedDPS.Len())
				dp := convertedDPS.SummaryDataPointSlice.At(0)
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
		metric.SetDataType(pdata.MetricDataTypeNone)

		obs, logs := observer.New(zap.WarnLevel)
		logger := zap.New(obs)

		dps := getDataPoints(&metric, metadata, logger)
		assert.Nil(t, dps)

		// Test output warning logs
		expectedLogs := []observer.LoggedEntry{
			{
				Entry: zapcore.Entry{Level: zap.WarnLevel, Message: "Unhandled metric data type."},
				Context: []zapcore.Field{
					zap.String("DataType", "None"),
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
	ocMetrics := []*metricspb.Metric{
		generateTestIntGauge("int-gauge"),
		generateTestDoubleGauge("double-gauge"),
		generateTestHistogram("double-histogram"),
	}
	ocMetrics = append(ocMetrics, generateTestIntSum("int-sum")...)
	ocMetrics = append(ocMetrics, generateTestDoubleSum("double-sum")...)
	ocMetrics = append(ocMetrics, generateTestSummary("summary")...)
	rms := internaldata.OCToMetrics(nil, nil, ocMetrics).ResourceMetrics()
	metrics := rms.At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	numMetrics := metrics.Len()

	metadata := cWMetricMetadata{
		groupedMetricMetadata: groupedMetricMetadata{
			namespace:   "Namespace",
			timestampMs: int64(1596151098037),
			logGroup:    "log-group",
			logStream:   "log-stream",
		},
		instrumentationLibraryName: "cloudwatch-otel",
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

func TestIntDataPointSlice_At(t *testing.T) {
	type fields struct {
		instrumentationLibraryName string
		deltaMetricMetadata        deltaMetricMetadata
		NumberDataPointSlice       pdata.NumberDataPointSlice
	}
	type args struct {
		i int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   dataPoint
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dps := numberDataPointSlice{
				instrumentationLibraryName: tt.fields.instrumentationLibraryName,
				deltaMetricMetadata:        tt.fields.deltaMetricMetadata,
				NumberDataPointSlice:       tt.fields.NumberDataPointSlice,
			}
			if got, _ := dps.At(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("At() = %v, want %v", got, tt.want)
			}
		})
	}
}
