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
	"io/ioutil"
	"sort"
	"strings"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/collector/translator/internaldata"
)

func TestTranslateOtToCWMetricWithInstrLibrary(t *testing.T) {

	md := createMetricTestData()
	rm := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.At(0)
	ilm.InstrumentationLibrary().InitEmpty()
	ilm.InstrumentationLibrary().SetName("cloudwatch-lib")
	cwm, totalDroppedMetrics := TranslateOtToCWMetric(&rm, ZeroAndSingleDimensionRollup, "")
	assert.Equal(t, 1, totalDroppedMetrics)
	assert.NotNil(t, cwm)
	assert.Equal(t, 5, len(cwm))
	assert.Equal(t, 1, len(cwm[0].Measurements))

	met := cwm[0]

	assert.Equal(t, met.Fields["spanCounter"], 0)
	assert.Equal(t, "myServiceNS/myServiceName", met.Measurements[0].Namespace)
	assert.Equal(t, 4, len(met.Measurements[0].Dimensions))
	dimensionSetOne := met.Measurements[0].Dimensions[0]
	sort.Strings(dimensionSetOne)
	assert.Equal(t, []string{OTellibDimensionKey, "isItAnError", "spanName"}, dimensionSetOne)
	assert.Equal(t, 1, len(met.Measurements[0].Metrics))
	assert.Equal(t, "spanCounter", met.Measurements[0].Metrics[0]["Name"])
	assert.Equal(t, "Count", met.Measurements[0].Metrics[0]["Unit"])

	dimensionSetTwo := met.Measurements[0].Dimensions[1]
	assert.Equal(t, []string{OTellibDimensionKey}, dimensionSetTwo)

	dimensionSetThree := met.Measurements[0].Dimensions[2]
	sort.Strings(dimensionSetThree)
	assert.Equal(t, []string{OTellibDimensionKey, "spanName"}, dimensionSetThree)

	dimensionSetFour := met.Measurements[0].Dimensions[3]
	sort.Strings(dimensionSetFour)
	assert.Equal(t, []string{OTellibDimensionKey, "isItAnError"}, dimensionSetFour)
}

func TestTranslateOtToCWMetricWithoutInstrLibrary(t *testing.T) {

	md := createMetricTestData()
	rm := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	cwm, totalDroppedMetrics := TranslateOtToCWMetric(&rm, ZeroAndSingleDimensionRollup, "")
	assert.Equal(t, 1, totalDroppedMetrics)
	assert.NotNil(t, cwm)
	assert.Equal(t, 5, len(cwm))
	assert.Equal(t, 1, len(cwm[0].Measurements))

	met := cwm[0]
	assert.NotContains(t, met.Fields, OTellibDimensionKey)
	assert.Equal(t, met.Fields["spanCounter"], 0)

	assert.Equal(t, "myServiceNS/myServiceName", met.Measurements[0].Namespace)
	assert.Equal(t, 4, len(met.Measurements[0].Dimensions))
	dimensionSetOne := met.Measurements[0].Dimensions[0]
	sort.Strings(dimensionSetOne)
	assert.Equal(t, []string{"isItAnError", "spanName"}, dimensionSetOne)
	assert.Equal(t, 1, len(met.Measurements[0].Metrics))
	assert.Equal(t, "spanCounter", met.Measurements[0].Metrics[0]["Name"])
	assert.Equal(t, "Count", met.Measurements[0].Metrics[0]["Unit"])

	// zero dimension metric
	dimensionSetTwo := met.Measurements[0].Dimensions[1]
	assert.Equal(t, []string{}, dimensionSetTwo)

	dimensionSetThree := met.Measurements[0].Dimensions[2]
	sort.Strings(dimensionSetTwo)
	assert.Equal(t, []string{"spanName"}, dimensionSetThree)

	dimensionSetFour := met.Measurements[0].Dimensions[3]
	sort.Strings(dimensionSetFour)
	assert.Equal(t, []string{"isItAnError"}, dimensionSetFour)
}

func TestTranslateOtToCWMetricWithNameSpace(t *testing.T) {
	md := consumerdata.MetricsData{
		Node: &commonpb.Node{
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				conventions.AttributeServiceName: "myServiceName",
			},
		},
		Metrics: []*metricspb.Metric{},
	}
	rm := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	cwm, totalDroppedMetrics := TranslateOtToCWMetric(&rm, ZeroAndSingleDimensionRollup, "")
	assert.Equal(t, 0, totalDroppedMetrics)
	assert.Nil(t, cwm)
	assert.Equal(t, 0, len(cwm))
	md = consumerdata.MetricsData{
		Node: &commonpb.Node{
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				conventions.AttributeServiceNamespace: "myServiceNS",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
							{Value: "false", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 1,
								},
							},
						},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
				},
				Timeseries: []*metricspb.TimeSeries{},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanGaugeCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_GAUGE_INT64,
				},
				Timeseries: []*metricspb.TimeSeries{},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanGaugeDoubleCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_GAUGE_DOUBLE,
				},
				Timeseries: []*metricspb.TimeSeries{},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanDoubleCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
				},
				Timeseries: []*metricspb.TimeSeries{},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanDoubleCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
				},
				Timeseries: []*metricspb.TimeSeries{},
			},
		},
	}
	rm = internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	cwm, totalDroppedMetrics = TranslateOtToCWMetric(&rm, ZeroAndSingleDimensionRollup, "")
	assert.Equal(t, 0, totalDroppedMetrics)
	assert.NotNil(t, cwm)
	assert.Equal(t, 1, len(cwm))

	met := cwm[0]
	assert.Equal(t, "myServiceNS", met.Measurements[0].Namespace)
}

func TestTranslateCWMetricToEMF(t *testing.T) {
	cwMeasurement := CwMeasurement{
		Namespace:  "test-emf",
		Dimensions: [][]string{{"OTelLib"}, {"OTelLib", "spanName"}},
		Metrics: []map[string]string{{
			"Name": "spanCounter",
			"Unit": "Count",
		}},
	}
	timestamp := int64(1596151098037)
	fields := make(map[string]interface{})
	fields["OTelLib"] = "cloudwatch-otel"
	fields["spanName"] = "test"
	fields["spanCounter"] = 0

	met := &CWMetrics{
		Timestamp:    timestamp,
		Fields:       fields,
		Measurements: []CwMeasurement{cwMeasurement},
	}
	inputLogEvent := TranslateCWMetricToEMF([]*CWMetrics{met})

	assert.Equal(t, readFromFile("testdata/testTranslateCWMetricToEMF.json"), *inputLogEvent[0].InputLogEvent.Message, "Expect to be equal")
}

func TestGetCWMetrics(t *testing.T) {
	namespace := "Namespace"
	OTelLib := "OTelLib"
	instrumentationLibName := "InstrLibName"

	testCases := []struct {
		testName string
		metric   *metricspb.Metric
		expected []*CWMetrics
	}{
		{
			"Int gauge",
			&metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "foo",
					Type: metricspb.MetricDescriptor_GAUGE_INT64,
					Unit: "Count",
					LabelKeys: []*metricspb.LabelKey{
						{Key: "label1"},
						{Key: "label2"},
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
					{
						LabelValues: []*metricspb.LabelValue{
							{HasValue: false},
							{Value: "value2", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Value: &metricspb.Point_Int64Value{
									Int64Value: 3,
								},
							},
						},
					},
				},
			},
			[]*CWMetrics{
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label1", "label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    int64(1),
						"label1": "value1",
						"label2": "value2",
					},
				},
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    int64(3),
						"label2": "value2",
					},
				},
			},
		},
		{
			"Double gauge",
			&metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "foo",
					Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
					Unit: "Count",
					LabelKeys: []*metricspb.LabelKey{
						{Key: "label1"},
						{Key: "label2"},
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
					{
						LabelValues: []*metricspb.LabelValue{
							{HasValue: false},
							{Value: "value2", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 0.3,
								},
							},
						},
					},
				},
			},
			[]*CWMetrics{
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label1", "label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    0.1,
						"label1": "value1",
						"label2": "value2",
					},
				},
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    0.3,
						"label2": "value2",
					},
				},
			},
		},
		{
			"Int sum",
			&metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "foo",
					Type: metricspb.MetricDescriptor_CUMULATIVE_INT64,
					Unit: "Count",
					LabelKeys: []*metricspb.LabelKey{
						{Key: "label1"},
						{Key: "label2"},
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
					{
						LabelValues: []*metricspb.LabelValue{
							{HasValue: false},
							{Value: "value2", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Value: &metricspb.Point_Int64Value{
									Int64Value: 3,
								},
							},
						},
					},
				},
			},
			[]*CWMetrics{
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label1", "label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    0,
						"label1": "value1",
						"label2": "value2",
					},
				},
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    0,
						"label2": "value2",
					},
				},
			},
		},
		{
			"Double sum",
			&metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "foo",
					Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
					Unit: "Count",
					LabelKeys: []*metricspb.LabelKey{
						{Key: "label1"},
						{Key: "label2"},
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
					{
						LabelValues: []*metricspb.LabelValue{
							{HasValue: false},
							{Value: "value2", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 0.3,
								},
							},
						},
					},
				},
			},
			[]*CWMetrics{
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label1", "label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    0,
						"label1": "value1",
						"label2": "value2",
					},
				},
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Count"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib:  instrumentationLibName,
						"foo":    0,
						"label2": "value2",
					},
				},
			},
		},
		{
			"Double histogram",
			&metricspb.Metric{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name: "foo",
					Type: metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
					Unit: "Seconds",
					LabelKeys: []*metricspb.LabelKey{
						{Key: "label1"},
						{Key: "label2"},
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
										Sum:   15.0,
										Count: 5,
										BucketOptions: &metricspb.DistributionValue_BucketOptions{
											Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
												Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
													Bounds: []float64{0, 10},
												},
											},
										},
										Buckets: []*metricspb.DistributionValue_Bucket{
											{
												Count: 0,
											},
											{
												Count: 4,
											},
											{
												Count: 1,
											},
										},
									},
								},
							},
						},
					},
					{
						LabelValues: []*metricspb.LabelValue{
							{HasValue: false},
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
			},
			[]*CWMetrics{
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label1", "label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Seconds"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib: instrumentationLibName,
						"foo": &CWMetricStats{
							Min:   0,
							Max:   10,
							Sum:   15.0,
							Count: 5,
						},
						"label1": "value1",
						"label2": "value2",
					},
				},
				{
					Measurements: []CwMeasurement{
						{
							Namespace: namespace,
							Dimensions: [][]string{
								{"label2", OTelLib},
							},
							Metrics: []map[string]string{
								{"Name": "foo", "Unit": "Seconds"},
							},
						},
					},
					Fields: map[string]interface{}{
						OTelLib: instrumentationLibName,
						"foo": &CWMetricStats{
							Min:   0,
							Max:   10,
							Sum:   35.0,
							Count: 18,
						},
						"label2": "value2",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			oc := consumerdata.MetricsData{
				Node: &commonpb.Node{},
				Resource: &resourcepb.Resource{
					Labels: map[string]string{
						conventions.AttributeServiceName:      "myServiceName",
						conventions.AttributeServiceNamespace: "myServiceNS",
					},
				},
				Metrics: []*metricspb.Metric{tc.metric},
			}

			// Retrieve *pdata.Metric
			rms := internaldata.OCToMetrics(oc).ResourceMetrics()
			assert.Equal(t, 1, rms.Len())
			ilms := rms.At(0).InstrumentationLibraryMetrics()
			assert.Equal(t, 1, ilms.Len())
			metrics := ilms.At(0).Metrics()
			assert.Equal(t, 1, metrics.Len())
			metric := metrics.At(0)

			cwMetrics := getCWMetrics(&metric, namespace, instrumentationLibName, "")
			assert.Equal(t, len(tc.expected), len(cwMetrics))

			for i, expected := range tc.expected {
				cwMetric := cwMetrics[i]
				assert.Equal(t, len(expected.Measurements), len(cwMetric.Measurements))
				assert.Equal(t, expected.Measurements, cwMetric.Measurements)
				assert.Equal(t, len(expected.Fields), len(cwMetric.Fields))
				assert.Equal(t, expected.Fields, cwMetric.Fields)
			}
		})
	}
}

func TestBuildCWMetric(t *testing.T) {
	namespace := "Namespace"
	instrLibName := "InstrLibName"
	OTelLib := "OTelLib"
	metricSlice := []map[string]string{
		map[string]string{
			"Name": "foo",
			"Unit": "",
		},
	}
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetName("foo")

	t.Run("Int gauge", func(t *testing.T) {
		metric.SetDataType(pdata.MetricDataTypeIntGauge)
		dp := pdata.NewIntDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(map[string]string{
			"label1": "value1",
		})
		dp.SetValue(int64(-17))

		cwMetric := buildCWMetric(dp, &metric, namespace, metricSlice, instrLibName, "")

		assert.NotNil(t, cwMetric)
		assert.Equal(t, 1, len(cwMetric.Measurements))
		expectedMeasurement := CwMeasurement{
			Namespace: namespace,
			Dimensions: [][]string{{"label1", OTelLib}},
			Metrics: metricSlice,
		}
		assert.Equal(t, expectedMeasurement, cwMetric.Measurements[0])
		expectedFields := map[string]interface{}{
			OTelLib:  instrLibName,
			"foo":    int64(-17),
			"label1": "value1",
		}
		assert.Equal(t, expectedFields, cwMetric.Fields)
	})

	t.Run("Double gauge", func(t *testing.T) {
		metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
		dp := pdata.NewDoubleDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(map[string]string{
			"label1": "value1",
		})
		dp.SetValue(0.3)

		cwMetric := buildCWMetric(dp, &metric, namespace, metricSlice, instrLibName, "")

		assert.NotNil(t, cwMetric)
		assert.Equal(t, 1, len(cwMetric.Measurements))
		expectedMeasurement := CwMeasurement{
			Namespace: namespace,
			Dimensions: [][]string{{"label1", OTelLib}},
			Metrics: metricSlice,
		}
		assert.Equal(t, expectedMeasurement, cwMetric.Measurements[0])
		expectedFields := map[string]interface{}{
			OTelLib:  instrLibName,
			"foo":    0.3,
			"label1": "value1",
		}
		assert.Equal(t, expectedFields, cwMetric.Fields)
	})

	t.Run("Int sum", func(t *testing.T) {
		metric.SetDataType(pdata.MetricDataTypeIntSum)
		metric.IntSum().InitEmpty()
		metric.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		dp := pdata.NewIntDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(map[string]string{
			"label1": "value1",
		})
		dp.SetValue(int64(-17))

		cwMetric := buildCWMetric(dp, &metric, namespace, metricSlice, instrLibName, "")

		assert.NotNil(t, cwMetric)
		assert.Equal(t, 1, len(cwMetric.Measurements))
		expectedMeasurement := CwMeasurement{
			Namespace: namespace,
			Dimensions: [][]string{{"label1", OTelLib}},
			Metrics: metricSlice,
		}
		assert.Equal(t, expectedMeasurement, cwMetric.Measurements[0])
		expectedFields := map[string]interface{}{
			OTelLib:  instrLibName,
			"foo":    0,
			"label1": "value1",
		}
		assert.Equal(t, expectedFields, cwMetric.Fields)
	})

	t.Run("Double sum", func(t *testing.T) {
		metric.SetDataType(pdata.MetricDataTypeDoubleSum)
		metric.DoubleSum().InitEmpty()
		metric.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		dp := pdata.NewDoubleDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(map[string]string{
			"label1": "value1",
		})
		dp.SetValue(0.3)

		cwMetric := buildCWMetric(dp, &metric, namespace, metricSlice, instrLibName, "")

		assert.NotNil(t, cwMetric)
		assert.Equal(t, 1, len(cwMetric.Measurements))
		expectedMeasurement := CwMeasurement{
			Namespace: namespace,
			Dimensions: [][]string{{"label1", OTelLib}},
			Metrics: metricSlice,
		}
		assert.Equal(t, expectedMeasurement, cwMetric.Measurements[0])
		expectedFields := map[string]interface{}{
			OTelLib:  instrLibName,
			"foo":    0,
			"label1": "value1",
		}
		assert.Equal(t, expectedFields, cwMetric.Fields)
	})

	t.Run("Double histogram", func(t *testing.T) {
		metric.SetDataType(pdata.MetricDataTypeDoubleHistogram)
		dp := pdata.NewDoubleHistogramDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(map[string]string{
			"label1": "value1",
		})
		dp.SetCount(uint64(17))
		dp.SetSum(float64(17.13))
		dp.SetBucketCounts([]uint64{1, 2, 3})
		dp.SetExplicitBounds([]float64{1, 2, 3})

		cwMetric := buildCWMetric(dp, &metric, namespace, metricSlice, instrLibName, "")

		assert.NotNil(t, cwMetric)
		assert.Equal(t, 1, len(cwMetric.Measurements))
		expectedMeasurement := CwMeasurement{
			Namespace: namespace,
			Dimensions: [][]string{{"label1", OTelLib}},
			Metrics: metricSlice,
		}
		assert.Equal(t, expectedMeasurement, cwMetric.Measurements[0])
		expectedFields := map[string]interface{}{
			OTelLib:  instrLibName,
			"foo":    &CWMetricStats{
				Min:   1,
				Max:   3,
				Sum:   17.13,
				Count: 17,
			},
			"label1": "value1",
		}
		assert.Equal(t, expectedFields, cwMetric.Fields)
	})
	
	t.Run("Invalid datapoint type", func(t *testing.T) {
		metric.SetDataType(pdata.MetricDataTypeIntGauge)
		dp := pdata.NewIntHistogramDataPoint()
		dp.InitEmpty()

		cwMetric := buildCWMetric(dp, &metric, namespace, metricSlice, instrLibName, "")
		assert.Nil(t, cwMetric)
	})
}

func TestCreateDimensions(t *testing.T) {
	OTelLib := "OTelLib"
	testCases := []struct {
		testName string
		labels   map[string]string
		dims     [][]string
	}{
		{
			"single label",
			map[string]string{"a": "foo"},
			[][]string{
				{"a", OTelLib},
				{OTelLib},
			},
		},
		{
			"multiple labels",
			map[string]string{"a": "foo", "b": "bar"},
			[][]string{
				{"a", "b", OTelLib},
				{OTelLib},
				{OTelLib, "a"},
				{OTelLib, "b"},
			},
		},
		{
			"no labels",
			map[string]string{},
			[][]string{
				{OTelLib},
			},
		},
	}

	sliceSorter := func(slice [][]string) func(a, b int) bool {
		stringified := make([]string, len(slice))
		for i, v := range slice {
			stringified[i] = strings.Join(v, ",")
		}
		return func(i, j int) bool {
			return stringified[i] > stringified[j]
		}
	}

	for _, tc := range testCases {
		dp := pdata.NewIntDataPoint()
		dp.InitEmpty()
		dp.LabelsMap().InitFromMap(tc.labels)
		dimensions, fields := createDimensions(dp, OTelLib, ZeroAndSingleDimensionRollup)

		// Sort slice for equality check
		sort.Slice(tc.dims, sliceSorter(tc.dims))
		sort.Slice(dimensions, sliceSorter(dimensions))

		assert.Equal(t, tc.dims, dimensions)

		expectedFields := make(map[string]interface{})
		for k, v := range tc.labels {
			expectedFields[k] = v
		}
		expectedFields[OTellibDimensionKey] = OTelLib

		assert.Equal(t, expectedFields, fields)
	}

}

func TestCalculateRate(t *testing.T) {
	prevValue := int64(0)
	curValue := int64(10)
	fields := make(map[string]interface{})
	fields["OTelLib"] = "cloudwatch-otel"
	fields["spanName"] = "test"
	fields["spanCounter"] = prevValue
	fields["type"] = "Int64"
	prevTime := time.Now().UnixNano() / int64(time.Millisecond)
	curTime := time.Unix(0, prevTime*int64(time.Millisecond)).Add(time.Second*10).UnixNano() / int64(time.Millisecond)
	rate := calculateRate(fields, prevValue, prevTime)
	assert.Equal(t, 0, rate)
	rate = calculateRate(fields, curValue, curTime)
	assert.Equal(t, int64(1), rate)

	prevDoubleValue := 0.0
	curDoubleValue := 5.0
	fields["type"] = "Float64"
	rate = calculateRate(fields, prevDoubleValue, prevTime)
	assert.Equal(t, 0, rate)
	rate = calculateRate(fields, curDoubleValue, curTime)
	assert.Equal(t, 0.5, rate)
}

func readFromFile(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	str := string(data)
	return str
}

func createMetricTestData() consumerdata.MetricsData {
	return consumerdata.MetricsData{
		Node: &commonpb.Node{
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				conventions.AttributeServiceName:      "myServiceName",
				conventions.AttributeServiceNamespace: "myServiceNS",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
							{Value: "false", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 1,
								},
							},
						},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanGaugeCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_GAUGE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
							{Value: "false", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 1,
								},
							},
						},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanDoubleCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
							{Value: "false", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 0.1,
								},
							},
						},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanGaugeDoubleCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_GAUGE_DOUBLE,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
							{Value: "false", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_DoubleValue{
									DoubleValue: 0.1,
								},
							},
						},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanTimer",
					Description: "How long the spans take",
					Unit:        "Seconds",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_DistributionValue{
									DistributionValue: &metricspb.DistributionValue{
										Sum:   15.0,
										Count: 5,
										BucketOptions: &metricspb.DistributionValue_BucketOptions{
											Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
												Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
													Bounds: []float64{0, 10},
												},
											},
										},
										Buckets: []*metricspb.DistributionValue_Bucket{
											{
												Count: 0,
											},
											{
												Count: 4,
											},
											{
												Count: 1,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanTimer",
					Description: "How long the spans take",
					Unit:        "Seconds",
					Type:        metricspb.MetricDescriptor_SUMMARY,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan"},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_SummaryValue{
									SummaryValue: &metricspb.SummaryValue{
										Sum: &wrappers.DoubleValue{
											Value: 15.0,
										},
										Count: &wrappers.Int64Value{
											Value: 5,
										},
										Snapshot: &metricspb.SummaryValue_Snapshot{
											PercentileValues: []*metricspb.SummaryValue_Snapshot_ValueAtPercentile{{
												Percentile: 0,
												Value:      1,
											},
												{
													Percentile: 100,
													Value:      5,
												}},
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

func TestNeedsCalculateRate(t *testing.T) {
	metric := pdata.NewMetric()
	metric.InitEmpty()
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	assert.False(t, needsCalculateRate(&metric))
	metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	assert.False(t, needsCalculateRate(&metric))

	metric.SetDataType(pdata.MetricDataTypeIntHistogram)
	assert.False(t, needsCalculateRate(&metric))
	metric.SetDataType(pdata.MetricDataTypeDoubleHistogram)
	assert.False(t, needsCalculateRate(&metric))

	metric.SetDataType(pdata.MetricDataTypeIntSum)
	metric.IntSum().InitEmpty()
	metric.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	assert.True(t, needsCalculateRate(&metric))
	metric.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	assert.False(t, needsCalculateRate(&metric))

	metric.SetDataType(pdata.MetricDataTypeDoubleSum)
	metric.DoubleSum().InitEmpty()
	metric.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
	assert.True(t, needsCalculateRate(&metric))
	metric.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	assert.False(t, needsCalculateRate(&metric))
}
