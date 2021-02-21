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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

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
							{Value: "testSpan", HasValue: true},
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

func stringSlicesEqual(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i, expectedStr := range expected {
		if expectedStr != actual[i] {
			return false
		}
	}
	return true
}

// hashDimensions hashes dimensions for equality checking.
func hashDimensions(dims [][]string) []string {
	// Convert to string for easier sorting
	stringified := make([]string, len(dims))
	for i, v := range dims {
		sort.Strings(v)
		stringified[i] = strings.Join(v, ",")
	}
	// Sort across dimension sets for equality checking
	sort.Strings(stringified)
	return stringified
}

// hashMetricSlice hashes a metrics slice for equality checking.
func hashMetricSlice(metricSlice []map[string]string) []string {
	// Convert to string for easier sorting
	stringified := make([]string, len(metricSlice))
	for i, v := range metricSlice {
		stringified[i] = v["Name"] + "," + v["Unit"]
	}
	// Sort across metrics for equality checking
	sort.Strings(stringified)
	return stringified
}

// assertDimsEqual asserts whether dimension sets are equal
// (i.e. has same sets of dimensions), regardless of order.
func assertDimsEqual(t *testing.T, expected, actual [][]string) {
	assert.Equal(t, len(expected), len(actual))
	expectedHashedDimensions := hashDimensions(expected)
	actualHashedDimensions := hashDimensions(actual)
	assert.Equal(t, expectedHashedDimensions, actualHashedDimensions)
}

// cWMeasurementEqual returns true if CW Measurements are equal.
func cWMeasurementEqual(expected, actual CWMeasurement) bool {
	// Check namespace
	if expected.Namespace != actual.Namespace {
		return false
	}

	// Check metrics
	if len(expected.Metrics) != len(actual.Metrics) {
		return false
	}
	expectedHashedMetrics := hashMetricSlice(expected.Metrics)
	actualHashedMetrics := hashMetricSlice(actual.Metrics)
	if !stringSlicesEqual(expectedHashedMetrics, actualHashedMetrics) {
		return false
	}

	// Check dimensions
	if len(expected.Dimensions) != len(actual.Dimensions) {
		return false
	}
	expectedHashedDimensions := hashDimensions(expected.Dimensions)
	actualHashedDimensions := hashDimensions(actual.Dimensions)
	return stringSlicesEqual(expectedHashedDimensions, actualHashedDimensions)
}

// assertCWMeasurementEqual asserts whether CW Measurements are equal.
func assertCWMeasurementEqual(t *testing.T, expected, actual CWMeasurement) {
	// Check namespace
	assert.Equal(t, expected.Namespace, actual.Namespace)

	// Check metrics
	assert.Equal(t, len(expected.Metrics), len(actual.Metrics))
	expectedHashSlice := hashMetricSlice(expected.Metrics)
	actualHashSlice := hashMetricSlice(actual.Metrics)
	assert.Equal(t, expectedHashSlice, actualHashSlice)

	// Check dimensions
	assertDimsEqual(t, expected.Dimensions, actual.Dimensions)
}

// assertCWMeasurementSliceEqual asserts whether CW Measurements are equal, regardless of order.
func assertCWMeasurementSliceEqual(t *testing.T, expected, actual []CWMeasurement) {
	assert.Equal(t, len(expected), len(actual))
	seen := make([]bool, len(expected))
	for _, actualMeasurement := range actual {
		hasMatch := false
		for i, expectedMeasurement := range expected {
			if !seen[i] {
				if cWMeasurementEqual(actualMeasurement, expectedMeasurement) {
					seen[i] = true
					hasMatch = true
				}
			}
		}
		assert.True(t, hasMatch)
	}
}

// assertCWMetricsEqual asserts whether CW Metrics are equal.
func assertCWMetricsEqual(t *testing.T, expected, actual *CWMetrics) {
	assert.Equal(t, expected.TimestampMs, actual.TimestampMs)
	assert.Equal(t, expected.Fields, actual.Fields)
	assert.Equal(t, len(expected.Measurements), len(actual.Measurements))
	assertCWMeasurementSliceEqual(t, expected.Measurements, actual.Measurements)
}

func TestTranslateOtToGroupedMetric(t *testing.T) {
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	md := createMetricTestData()

	translator := newMetricTranslator(*config)

	noInstrLibMetric := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	instrLibMetric := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	ilm := instrLibMetric.InstrumentationLibraryMetrics().At(0)
	ilm.InstrumentationLibrary().SetName("cloudwatch-lib")

	noNamespaceMetric := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	noNamespaceMetric.Resource().Attributes().Delete(conventions.AttributeServiceNamespace)
	noNamespaceMetric.Resource().Attributes().Delete(conventions.AttributeServiceName)

	counterMetrics := map[string]*MetricInfo{
		"spanCounter": {
			Value: float64(0),
			Unit:  "Count",
		},
		"spanDoubleCounter": {
			Value: float64(0),
			Unit:  "Count",
		},
		"spanGaugeCounter": {
			Value: float64(1),
			Unit:  "Count",
		},
		"spanGaugeDoubleCounter": {
			Value: float64(0.1),
			Unit:  "Count",
		},
	}
	timerMetrics := map[string]*MetricInfo{
		"spanTimer": {
			Value: &CWMetricStats{
				Count: 5,
				Sum:   15,
			},
			Unit: "Seconds",
		},
	}

	testCases := []struct {
		testName          string
		metric            *pdata.ResourceMetrics
		counterLabels     map[string]string
		timerLabels       map[string]string
		expectedNamespace string
	}{
		{
			"w/ instrumentation library and namespace",
			&instrLibMetric,
			map[string]string{
				(oTellibDimensionKey): "cloudwatch-lib",
				"isItAnError":         "false",
				"spanName":            "testSpan",
			},
			map[string]string{
				(oTellibDimensionKey): "cloudwatch-lib",
				"spanName":            "testSpan",
			},
			"myServiceNS/myServiceName",
		},
		{
			"w/o instrumentation library, w/ namespace",
			&noInstrLibMetric,
			map[string]string{
				"isItAnError": "false",
				"spanName":    "testSpan",
			},
			map[string]string{
				"spanName": "testSpan",
			},
			"myServiceNS/myServiceName",
		},
		{
			"w/o instrumentation library and namespace",
			&noNamespaceMetric,
			map[string]string{
				"isItAnError": "false",
				"spanName":    "testSpan",
			},
			map[string]string{
				"spanName": "testSpan",
			},
			defaultNamespace,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			groupedMetrics := make(map[interface{}]*GroupedMetric)
			translator.translateOTelToGroupedMetric(tc.metric, groupedMetrics, config)
			assert.NotNil(t, groupedMetrics)
			assert.Equal(t, 2, len(groupedMetrics))

			for _, v := range groupedMetrics {
				assert.Equal(t, tc.expectedNamespace, v.Metadata.Namespace)
				if len(v.Metrics) == 4 {
					assert.Equal(t, tc.counterLabels, v.Labels)
					assert.Equal(t, counterMetrics, v.Metrics)
				} else {
					assert.Equal(t, 1, len(v.Metrics))
					assert.Equal(t, tc.timerLabels, v.Labels)
					assert.Equal(t, timerMetrics, v.Metrics)
				}
			}
		})
	}

	t.Run("No metrics", func(t *testing.T) {
		md = consumerdata.MetricsData{
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
		groupedMetrics := make(map[interface{}]*GroupedMetric)
		translator.translateOTelToGroupedMetric(&rm, groupedMetrics, config)
		assert.Equal(t, 0, len(groupedMetrics))
	})
}

func TestTranslateCWMetricToEMF(t *testing.T) {
	cwMeasurement := CWMeasurement{
		Namespace:  "test-emf",
		Dimensions: [][]string{{oTellibDimensionKey}, {oTellibDimensionKey, "spanName"}},
		Metrics: []map[string]string{{
			"Name": "spanCounter",
			"Unit": "Count",
		}},
	}
	timestamp := int64(1596151098037)
	fields := make(map[string]interface{})
	fields[oTellibDimensionKey] = "cloudwatch-otel"
	fields["spanName"] = "test"
	fields["spanCounter"] = 0

	met := &CWMetrics{
		TimestampMs:  timestamp,
		Fields:       fields,
		Measurements: []CWMeasurement{cwMeasurement},
	}
	inputLogEvent := translateCWMetricToEMF(met)

	assert.Equal(t, readFromFile("testdata/testTranslateCWMetricToEMF.json"), *inputLogEvent.InputLogEvent.Message, "Expect to be equal")
}

func TestTranslateGroupedMetricToCWMetric(t *testing.T) {
	timestamp := int64(1596151098037)
	namespace := "Namespace"
	testCases := []struct {
		testName           string
		groupedMetric      *GroupedMetric
		metricDeclarations []*MetricDeclaration
		expectedCWMetric   *CWMetrics
	}{
		{
			"single metric w/o metric declarations",
			&GroupedMetric{
				Labels: map[string]string{
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			nil,
			&CWMetrics{
				Measurements: []CWMeasurement{
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1"}},
						Metrics: []map[string]string{
							{
								"Name": "metric1",
								"Unit": "Count",
							},
						},
					},
				},
				TimestampMs: timestamp,
				Fields: map[string]interface{}{
					"label1":  "value1",
					"metric1": 1,
				},
			},
		},
		{
			"single metric w/ metric declarations",
			&GroupedMetric{
				Labels: map[string]string{
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"label1"}, {"label1", "label2"}},
					MetricNameSelectors: []string{"metric.*"},
				},
			},
			&CWMetrics{
				Measurements: []CWMeasurement{
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1"}},
						Metrics: []map[string]string{
							{
								"Name": "metric1",
								"Unit": "Count",
							},
						},
					},
				},
				TimestampMs: timestamp,
				Fields: map[string]interface{}{
					"label1":  "value1",
					"metric1": 1,
				},
			},
		},
		{
			"multiple metrics w/o metric declarations",
			&GroupedMetric{
				Labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
					"metric2": {
						Value: 200,
						Unit:  "Count",
					},
					"metric3": {
						Value: 3.14,
						Unit:  "Seconds",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			nil,
			&CWMetrics{
				Measurements: []CWMeasurement{
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1", "label2"}},
						Metrics: []map[string]string{
							{
								"Name": "metric1",
								"Unit": "Count",
							},
							{
								"Name": "metric2",
								"Unit": "Count",
							},
							{
								"Name": "metric3",
								"Unit": "Seconds",
							},
						},
					},
				},
				TimestampMs: timestamp,
				Fields: map[string]interface{}{
					"label1":  "value1",
					"label2":  "value2",
					"metric1": 1,
					"metric2": 200,
					"metric3": 3.14,
				},
			},
		},
		{
			"multiple metrics w/ metric declarations",
			&GroupedMetric{
				Labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
					"metric2": {
						Value: 200,
						Unit:  "Count",
					},
					"metric3": {
						Value: 3.14,
						Unit:  "Seconds",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			[]*MetricDeclaration{
				{
					Dimensions: [][]string{
						{"label1"},
						{"label1", "label3"},
					},
					MetricNameSelectors: []string{"metric1"},
				},
				{
					Dimensions: [][]string{
						{"label1", "label2"},
						{"label1", "label3"},
					},
					MetricNameSelectors: []string{"metric2"},
				},
			},
			&CWMetrics{
				Measurements: []CWMeasurement{
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1"}},
						Metrics: []map[string]string{
							{
								"Name": "metric1",
								"Unit": "Count",
							},
						},
					},
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1", "label2"}},
						Metrics: []map[string]string{
							{
								"Name": "metric2",
								"Unit": "Count",
							},
						},
					},
				},
				TimestampMs: timestamp,
				Fields: map[string]interface{}{
					"label1":  "value1",
					"label2":  "value2",
					"metric1": 1,
					"metric2": 200,
					"metric3": 3.14,
				},
			},
		},
		{
			"no metrics",
			&GroupedMetric{
				Labels: map[string]string{
					"label1": "value1",
				},
				Metrics: nil,
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			nil,
			&CWMetrics{
				Measurements: []CWMeasurement{
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1"}},
						Metrics:    nil,
					},
				},
				TimestampMs: timestamp,
				Fields: map[string]interface{}{
					"label1": "value1",
				},
			},
		},
	}

	logger := zap.NewNop()

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			config := &Config{
				MetricDeclarations:    tc.metricDeclarations,
				DimensionRollupOption: "",
				logger:                logger,
			}
			for _, decl := range tc.metricDeclarations {
				decl.Init(logger)
			}
			cWMetric := translateGroupedMetricToCWMetric(tc.groupedMetric, config)
			assert.NotNil(t, cWMetric)
			assertCWMetricsEqual(t, tc.expectedCWMetric, cWMetric)
		})
	}
}

func TestGroupedMetricToCWMeasurement(t *testing.T) {
	timestamp := int64(1596151098037)
	namespace := "Namespace"
	testCases := []struct {
		testName              string
		dimensionRollupOption string
		groupedMetric         *GroupedMetric
		expectedMeasurement   CWMeasurement
	}{
		{
			"single metric, no dim rollup",
			"",
			&GroupedMetric{
				Labels: map[string]string{
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			CWMeasurement{
				Namespace:  namespace,
				Dimensions: [][]string{{"label1"}},
				Metrics: []map[string]string{
					{
						"Name": "metric1",
						"Unit": "Count",
					},
				},
			},
		},
		{
			"multiple metrics, no dim rollup",
			"",
			&GroupedMetric{
				Labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
					"metric2": {
						Value: 200,
						Unit:  "Count",
					},
					"metric3": {
						Value: 3.14,
						Unit:  "Seconds",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			CWMeasurement{
				Namespace:  namespace,
				Dimensions: [][]string{{"label1", "label2"}},
				Metrics: []map[string]string{
					{
						"Name": "metric1",
						"Unit": "Count",
					},
					{
						"Name": "metric2",
						"Unit": "Count",
					},
					{
						"Name": "metric3",
						"Unit": "Seconds",
					},
				},
			},
		},
		{
			"single metric, single dim rollup",
			singleDimensionRollupOnly,
			&GroupedMetric{
				Labels: map[string]string{
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			CWMeasurement{
				Namespace:  namespace,
				Dimensions: [][]string{{"label1"}},
				Metrics: []map[string]string{
					{
						"Name": "metric1",
						"Unit": "Count",
					},
				},
			},
		},
		{
			"multiple metrics, zero & single dim rollup",
			zeroAndSingleDimensionRollup,
			&GroupedMetric{
				Labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
					"metric2": {
						Value: 200,
						Unit:  "Count",
					},
					"metric3": {
						Value: 3.14,
						Unit:  "Seconds",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			CWMeasurement{
				Namespace: namespace,
				Dimensions: [][]string{
					{"label1", "label2"},
					{"label1"},
					{"label2"},
					{},
				},
				Metrics: []map[string]string{
					{
						"Name": "metric1",
						"Unit": "Count",
					},
					{
						"Name": "metric2",
						"Unit": "Count",
					},
					{
						"Name": "metric3",
						"Unit": "Seconds",
					},
				},
			},
		},
		{
			"no metrics",
			"",
			&GroupedMetric{
				Labels: map[string]string{
					"label1": "value1",
				},
				Metrics: nil,
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			},
			CWMeasurement{
				Namespace:  namespace,
				Dimensions: [][]string{{"label1"}},
				Metrics:    nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			config := &Config{
				MetricDeclarations:    nil,
				DimensionRollupOption: tc.dimensionRollupOption,
			}
			cWMeasurement := groupedMetricToCWMeasurement(tc.groupedMetric, config)
			assertCWMeasurementEqual(t, tc.expectedMeasurement, cWMeasurement)
		})
	}

	// Test rollup options and labels
	instrLibName := "cloudwatch-otel"
	rollUpTestCases := []struct {
		testName              string
		labels                map[string]string
		dimensionRollupOption string
		expectedDims          [][]string
	}{
		{
			"Single label, no rollup, no otel dim",
			map[string]string{"a": "foo"},
			"",
			[][]string{
				{"a"},
			},
		},
		{
			"Single label, no rollup, w/ otel dim",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			"",
			[][]string{
				{"a", oTellibDimensionKey},
			},
		},
		{
			"Single label, single rollup, no otel dim",
			map[string]string{"a": "foo"},
			singleDimensionRollupOnly,
			[][]string{
				{"a"},
			},
		},
		{
			"Single label, single rollup, w/ otel dim",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			singleDimensionRollupOnly,
			[][]string{
				{"a", oTellibDimensionKey},
			},
		},
		{
			"Single label, zero + single rollup, no otel dim",
			map[string]string{"a": "foo"},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a"},
				{},
			},
		},
		{
			"Single label, zero + single rollup, w/ otel dim",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a", oTellibDimensionKey},
				{oTellibDimensionKey},
			},
		},
		{
			"Multiple label, no rollup, no otel dim",
			map[string]string{
				"a": "foo",
				"b": "bar",
				"c": "car",
			},
			"",
			[][]string{
				{"a", "b", "c"},
			},
		},
		{
			"Multiple label, no rollup, w/ otel dim",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			"",
			[][]string{
				{"a", "b", "c", oTellibDimensionKey},
			},
		},
		{
			"Multiple label, rollup, no otel dim",
			map[string]string{
				"a": "foo",
				"b": "bar",
				"c": "car",
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a", "b", "c"},
				{"a"},
				{"b"},
				{"c"},
				{},
			},
		},
		{
			"Multiple label, rollup, w/ otel dim",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a", "b", "c", oTellibDimensionKey},
				{oTellibDimensionKey, "a"},
				{oTellibDimensionKey, "b"},
				{oTellibDimensionKey, "c"},
				{oTellibDimensionKey},
			},
		},
	}

	for _, tc := range rollUpTestCases {
		t.Run(tc.testName, func(t *testing.T) {
			groupedMetric := &GroupedMetric{
				Labels: tc.labels,
				Metrics: map[string]*MetricInfo{
					"metric1": {
						Value: 1,
						Unit:  "Count",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			}
			config := &Config{
				DimensionRollupOption: tc.dimensionRollupOption,
			}
			cWMeasurement := groupedMetricToCWMeasurement(groupedMetric, config)
			assertDimsEqual(t, tc.expectedDims, cWMeasurement.Dimensions)
		})
	}
}

func TestGroupedMetricToCWMeasurementsWithFilters(t *testing.T) {
	timestamp := int64(1596151098037)
	namespace := "Namespace"

	labels := map[string]string{
		"a": "A",
		"b": "B",
		"c": "C",
	}
	metrics := map[string]*MetricInfo{
		"metric1": {
			Value: 1,
			Unit:  "Count",
		},
		"metric2": {
			Value: 200,
			Unit:  "Count",
		},
		"metric3": {
			Value: 3.14,
			Unit:  "Seconds",
		},
	}
	testCases := []struct {
		testName             string
		metricDeclarations   []*MetricDeclaration
		expectedMeasurements []CWMeasurement
	}{
		{
			"single metric declaration",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}, {"a", "c"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
				},
			},
			[]CWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}, {"a", "c"}},
					Metrics: []map[string]string{
						{
							"Name": "metric1",
							"Unit": "Count",
						},
						{
							"Name": "metric2",
							"Unit": "Count",
						},
						{
							"Name": "metric3",
							"Unit": "Seconds",
						},
					},
				},
			},
		},
		{
			"multiple metric declarations, all unique",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
				},
				{
					Dimensions:          [][]string{{"a", "c"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric1"},
				},
				{
					Dimensions:          [][]string{{"a"}, {"b"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric(1|2)"},
				},
			},
			[]CWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}, {"b"}, {"a", "c"}},
					Metrics: []map[string]string{
						{
							"Name": "metric1",
							"Unit": "Count",
						},
					},
				},
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}, {"b"}},
					Metrics: []map[string]string{
						{
							"Name": "metric2",
							"Unit": "Count",
						},
					},
				},
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}},
					Metrics: []map[string]string{
						{
							"Name": "metric3",
							"Unit": "Seconds",
						},
					},
				},
			},
		},
		{
			"multiple metric declarations, hybrid",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
				},
				{
					Dimensions:          [][]string{{"a"}, {"b"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric(1|2)"},
				},
			},
			[]CWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}, {"b"}},
					Metrics: []map[string]string{
						{
							"Name": "metric1",
							"Unit": "Count",
						},
						{
							"Name": "metric2",
							"Unit": "Count",
						},
					},
				},
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}},
					Metrics: []map[string]string{
						{
							"Name": "metric3",
							"Unit": "Seconds",
						},
					},
				},
			},
		},
		{
			"some dimensions match",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
				},
				{
					Dimensions:          [][]string{{"a"}, {"b"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric(1|2)"},
				},
			},
			[]CWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}, {"b"}},
					Metrics: []map[string]string{
						{
							"Name": "metric1",
							"Unit": "Count",
						},
						{
							"Name": "metric2",
							"Unit": "Count",
						},
					},
				},
			},
		},
		{
			"no dimension match",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
				},
			},
			nil,
		},
		{
			"label matchers",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}, {"a", "c"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
					LabelMatchers: []*LabelMatcher{
						{
							LabelNames: []string{"a", "b", "d"},
							Regex:      "A;B;D",
						},
					},
				},
				{
					Dimensions:          [][]string{{"b"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
					LabelMatchers: []*LabelMatcher{
						{
							LabelNames: []string{"a", "b"},
							Regex:      "A;B",
						},
					},
				},
			},
			[]CWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"b"}},
					Metrics: []map[string]string{
						{
							"Name": "metric1",
							"Unit": "Count",
						},
						{
							"Name": "metric2",
							"Unit": "Count",
						},
						{
							"Name": "metric3",
							"Unit": "Seconds",
						},
					},
				},
			},
		},
	}

	logger := zap.NewNop()

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			groupedMetric := &GroupedMetric{
				Labels:  labels,
				Metrics: metrics,
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			}
			config := &Config{
				DimensionRollupOption: "",
				MetricDeclarations:    tc.metricDeclarations,
				logger:                logger,
			}
			for _, decl := range tc.metricDeclarations {
				err := decl.Init(logger)
				assert.Nil(t, err)
			}

			cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
			assert.NotNil(t, cWMeasurements)
			assert.Equal(t, len(tc.expectedMeasurements), len(cWMeasurements))
			assertCWMeasurementSliceEqual(t, tc.expectedMeasurements, cWMeasurements)
		})
	}

	t.Run("No label match", func(t *testing.T) {
		groupedMetric := &GroupedMetric{
			Labels:  labels,
			Metrics: metrics,
			Metadata: CWMetricMetadata{
				Namespace:   namespace,
				TimestampMs: timestamp,
			},
		}
		metricDeclarations := []*MetricDeclaration{
			{
				Dimensions:          [][]string{{"b"}, {"b", "d"}},
				MetricNameSelectors: []string{"metric.*"},
				LabelMatchers: []*LabelMatcher{
					{
						LabelNames: []string{"a", "b"},
						Regex:      "A;C",
					},
				},
			},
			{
				Dimensions:          [][]string{{"b"}, {"b", "d"}},
				MetricNameSelectors: []string{"metric.*"},
				LabelMatchers: []*LabelMatcher{
					{
						LabelNames: []string{"a", "b"},
						Regex:      "a;B",
					},
				},
			},
		}
		for _, decl := range metricDeclarations {
			err := decl.Init(zap.NewNop())
			assert.Nil(t, err)
		}
		obs, logs := observer.New(zap.DebugLevel)
		logger := zap.New(obs)
		config := &Config{
			DimensionRollupOption: "",
			MetricDeclarations:    metricDeclarations,
			logger:                logger,
		}

		cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
		assert.Nil(t, cWMeasurements)

		// Test output warning logs
		expectedLog := observer.LoggedEntry{
			Entry: zapcore.Entry{Level: zap.DebugLevel, Message: "Dropped batch of metrics: no metric declaration matched labels"},
			Context: []zapcore.Field{
				zap.String("Labels", "{\"a\":\"A\",\"b\":\"B\",\"c\":\"C\"}"),
				zap.Strings("Metric Names", []string{"metric1", "metric2", "metric3"}),
			},
		}
		assert.Equal(t, 1, logs.Len())
		log := logs.AllUntimed()[0]
		// Have to perform this hacky equality check because the metric names might not
		// be in the right order due to map iteration
		assert.Equal(t, expectedLog.Entry, log.Entry)
		assert.Equal(t, 2, len(log.Context))
		assert.Equal(t, expectedLog.Context[0], log.Context[0])
		isMatch := false
		possibleOrders := []zapcore.Field{
			zap.Strings("Metric Names", []string{"metric1", "metric2", "metric3"}),
			zap.Strings("Metric Names", []string{"metric1", "metric3", "metric2"}),
			zap.Strings("Metric Names", []string{"metric2", "metric1", "metric3"}),
			zap.Strings("Metric Names", []string{"metric2", "metric3", "metric1"}),
			zap.Strings("Metric Names", []string{"metric3", "metric2", "metric1"}),
			zap.Strings("Metric Names", []string{"metric3", "metric1", "metric2"}),
		}
		for _, field := range possibleOrders {
			if field.Equals(log.Context[1]) {
				isMatch = true
				break
			}
		}
		assert.True(t, isMatch)
	})

	t.Run("No metric name match", func(t *testing.T) {
		groupedMetric := &GroupedMetric{
			Labels:  labels,
			Metrics: metrics,
			Metadata: CWMetricMetadata{
				Namespace:   namespace,
				TimestampMs: timestamp,
			},
		}
		metricDeclarations := []*MetricDeclaration{
			{
				Dimensions:          [][]string{{"b"}, {"b", "d"}},
				MetricNameSelectors: []string{"metric4"},
			},
		}
		for _, decl := range metricDeclarations {
			err := decl.Init(zap.NewNop())
			assert.Nil(t, err)
		}
		obs, logs := observer.New(zap.DebugLevel)
		logger := zap.New(obs)
		config := &Config{
			DimensionRollupOption: "",
			MetricDeclarations:    metricDeclarations,
			logger:                logger,
		}

		cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
		assert.Nil(t, cWMeasurements)

		// Test output warning logs
		expectedEntry := zapcore.Entry{Level: zap.DebugLevel, Message: "Dropped metric: no metric declaration matched metric name"}
		expectedContexts := []zapcore.Field{
			zap.String("Metric name", "metric1"),
			zap.String("Metric name", "metric2"),
			zap.String("Metric name", "metric3"),
		}
		assert.Equal(t, 3, logs.Len())
		// Match logs (possibly out of order)
		seen := make([]bool, 3)
		for _, log := range logs.AllUntimed() {
			assert.Equal(t, expectedEntry, log.Entry)
			assert.Equal(t, 1, len(log.Context))
			hasMatch := false
			for i, expectedCtx := range expectedContexts {
				if !seen[i] && log.Context[0].Equals(expectedCtx) {
					hasMatch = true
					seen[i] = true
					break
				}
			}
			assert.True(t, hasMatch)
		}
	})

	// Test metric filtering with various roll-up options
	metricName := "metric1"
	instrLibName := "cloudwatch-otel"
	rollupTestCases := []struct {
		testName              string
		labels                map[string]string
		metricDeclarations    []*MetricDeclaration
		dimensionRollupOption string
		expectedDims          [][]string
	}{
		{
			"Single label w/ no rollup",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{{"a"}},
		},
		{
			"Single label + OTelLib w/ no rollup",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", oTellibDimensionKey}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{{"a", oTellibDimensionKey}},
		},
		{
			"Single label w/ single rollup",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			singleDimensionRollupOnly,
			[][]string{{"a"}, {"a", oTellibDimensionKey}},
		},
		{
			"Single label w/ zero/single rollup",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{{"a"}, {"a", oTellibDimensionKey}, {oTellibDimensionKey}},
		},
		{
			"Single label + Otel w/ zero/single rollup",
			map[string]string{
				"a":                   "foo",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", oTellibDimensionKey}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{{"a", oTellibDimensionKey}, {oTellibDimensionKey}},
		},
		{
			"multiple labels w/ no rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{{"a"}},
		},
		{
			"multiple labels w/ rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a"},
				{oTellibDimensionKey, "a"},
				{oTellibDimensionKey, "b"},
				{oTellibDimensionKey},
			},
		},
		{
			"multiple labels + multiple dimensions w/ no rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{{"a", "b"}, {"b"}},
		},
		{
			"multiple labels + multiple dimensions + oTellibDimensionKey w/ no rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b", oTellibDimensionKey}, {oTellibDimensionKey}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{{"a", "b"}, {"b", oTellibDimensionKey}, {oTellibDimensionKey}},
		},
		{
			"multiple labels + multiple dimensions w/ rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a", "b"},
				{"b"},
				{oTellibDimensionKey, "a"},
				{oTellibDimensionKey, "b"},
				{oTellibDimensionKey},
			},
		},
		{
			"multiple labels, multiple dimensions w/ invalid dimension",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b", "c"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"b"},
				{oTellibDimensionKey, "a"},
				{oTellibDimensionKey, "b"},
				{oTellibDimensionKey},
			},
		},
		{
			"multiple labels, multiple dimensions w/ missing dimension",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a", "b"},
				{"b"},
				{oTellibDimensionKey, "a"},
				{oTellibDimensionKey, "b"},
				{oTellibDimensionKey, "c"},
				{oTellibDimensionKey},
			},
		},
		{
			"multiple metric declarations w/ no rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
				{
					Dimensions:          [][]string{{"a", "c"}, {"b"}, {"c"}},
					MetricNameSelectors: []string{metricName},
				},
				{
					Dimensions:          [][]string{{"a", "d"}, {"b", "c"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{
				{"a", "b"},
				{"b"},
				{"a", "c"},
				{"c"},
				{"b", "c"},
			},
		},
		{
			"multiple metric declarations w/ rollup",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
				{
					Dimensions:          [][]string{{"a", "c"}, {"b"}, {"c"}},
					MetricNameSelectors: []string{metricName},
				},
				{
					Dimensions:          [][]string{{"a", "d"}, {"b", "c"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{
				{"a", "b"},
				{"b"},
				{oTellibDimensionKey, "a"},
				{oTellibDimensionKey, "b"},
				{oTellibDimensionKey, "c"},
				{oTellibDimensionKey},
				{"a", "c"},
				{"c"},
				{"b", "c"},
			},
		},
		{
			"remove measurements with no dimensions",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
				{
					Dimensions:          [][]string{{"a", "d"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			[][]string{
				{"a", "b"},
				{"b"},
			},
		},
		{
			"multiple declarations w/ no dimensions",
			map[string]string{
				"a":                   "foo",
				"b":                   "bar",
				"c":                   "car",
				(oTellibDimensionKey): instrLibName,
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "e"}, {"d"}},
					MetricNameSelectors: []string{metricName},
				},
				{
					Dimensions:          [][]string{{"a", "d"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			"",
			nil,
		},
		{
			"no labels",
			map[string]string{},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a", "b", "c"}, {"b"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			nil,
		},
	}

	for _, tc := range rollupTestCases {
		t.Run(tc.testName, func(t *testing.T) {
			groupedMetric := &GroupedMetric{
				Labels: tc.labels,
				Metrics: map[string]*MetricInfo{
					(metricName): {
						Value: int64(5),
						Unit:  "Count",
					},
				},
				Metadata: CWMetricMetadata{
					Namespace:   namespace,
					TimestampMs: timestamp,
				},
			}
			for _, decl := range tc.metricDeclarations {
				err := decl.Init(zap.NewNop())
				assert.Nil(t, err)
			}
			config := &Config{
				DimensionRollupOption: tc.dimensionRollupOption,
				MetricDeclarations:    tc.metricDeclarations,
				logger:                zap.NewNop(),
			}

			cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
			if len(tc.expectedDims) == 0 {
				assert.Equal(t, 0, len(cWMeasurements))
			} else {
				assert.Equal(t, 1, len(cWMeasurements))
				dims := cWMeasurements[0].Dimensions
				assertDimsEqual(t, tc.expectedDims, dims)
			}
		})
	}
}

func TestTranslateCWMetricToEMFNoMeasurements(t *testing.T) {
	timestamp := int64(1596151098037)
	fields := make(map[string]interface{})
	fields[oTellibDimensionKey] = "cloudwatch-otel"
	fields["spanName"] = "test"
	fields["spanCounter"] = 0

	met := &CWMetrics{
		TimestampMs:  timestamp,
		Fields:       fields,
		Measurements: nil,
	}
	inputLogEvent := translateCWMetricToEMF(met)
	expected := "{\"OTelLib\":\"cloudwatch-otel\",\"spanCounter\":0,\"spanName\":\"test\"}"

	assert.Equal(t, expected, *inputLogEvent.InputLogEvent.Message)
}

func BenchmarkTranslateOtToGroupedMetricWithInstrLibrary(b *testing.B) {
	md := createMetricTestData()
	rm := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.At(0)
	ilm.InstrumentationLibrary().SetName("cloudwatch-lib")
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetric := make(map[interface{}]*GroupedMetric)
		translator.translateOTelToGroupedMetric(&rm, groupedMetric, config)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithoutInstrLibrary(b *testing.B) {
	md := createMetricTestData()
	rm := internaldata.OCToMetrics(md).ResourceMetrics().At(0)
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[interface{}]*GroupedMetric)
		translator.translateOTelToGroupedMetric(&rm, groupedMetrics, config)
	}
}

func BenchmarkTranslateGroupedMetricToCWMetric(b *testing.B) {
	groupedMetric := &GroupedMetric{
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
			"label3": "value3",
		},
		Metrics: map[string]*MetricInfo{
			"metric1": {
				Value: 1,
				Unit:  "Count",
			},
			"metric2": {
				Value: 200,
				Unit:  "Seconds",
			},
		},
		Metadata: CWMetricMetadata{
			Namespace:   "Namespace",
			TimestampMs: int64(1596151098037),
		},
	}
	config := &Config{
		MetricDeclarations:    nil,
		DimensionRollupOption: zeroAndSingleDimensionRollup,
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		translateGroupedMetricToCWMetric(groupedMetric, config)
	}
}

func BenchmarkTranslateGroupedMetricToCWMetricWithFiltering(b *testing.B) {
	groupedMetric := &GroupedMetric{
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
			"label3": "value3",
		},
		Metrics: map[string]*MetricInfo{
			"metric1": {
				Value: 1,
				Unit:  "Count",
			},
			"metric2": {
				Value: 200,
				Unit:  "Seconds",
			},
		},
		Metadata: CWMetricMetadata{
			Namespace:   "Namespace",
			TimestampMs: int64(1596151098037),
		},
	}
	m := &MetricDeclaration{
		Dimensions:          [][]string{{"label1"}, {"label2"}},
		MetricNameSelectors: []string{"metric1", "metric2"},
	}
	logger := zap.NewNop()
	m.Init(logger)
	config := &Config{
		MetricDeclarations:    []*MetricDeclaration{m},
		DimensionRollupOption: zeroAndSingleDimensionRollup,
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		translateGroupedMetricToCWMetric(groupedMetric, config)
	}
}

func BenchmarkTranslateCWMetricToEMF(b *testing.B) {
	cwMeasurement := CWMeasurement{
		Namespace:  "test-emf",
		Dimensions: [][]string{{oTellibDimensionKey}, {oTellibDimensionKey, "spanName"}},
		Metrics: []map[string]string{{
			"Name": "spanCounter",
			"Unit": "Count",
		}},
	}
	timestamp := int64(1596151098037)
	fields := make(map[string]interface{})
	fields[oTellibDimensionKey] = "cloudwatch-otel"
	fields["spanName"] = "test"
	fields["spanCounter"] = 0

	met := &CWMetrics{
		TimestampMs:  timestamp,
		Fields:       fields,
		Measurements: []CWMeasurement{cwMeasurement},
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		translateCWMetricToEMF(met)
	}
}
