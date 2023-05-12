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
	"sort"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func createMetricTestData() *agentmetricspb.ExportMetricsServiceRequest {
	request := &agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				conventions.AttributeServiceName:      "myServiceName",
				conventions.AttributeServiceNamespace: "myServiceNS",
				"ClusterName":                         "myCluster",
				"PodName":                             "myPod",
				attributeReceiver:                     prometheusReceiver,
			},
		},
		Metrics: []*metricspb.Metric{
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

	for i := 0; i < 2; i++ {
		request.Metrics = append(request.Metrics, &metricspb.Metric{
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
								Seconds: int64(i * 100),
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: int64(i * 1),
							},
						},
					},
				},
			},
		}, &metricspb.Metric{
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
								Seconds: int64(i * 100),
							},
							Value: &metricspb.Point_DoubleValue{
								DoubleValue: float64(i) * 0.1,
							},
						},
					},
				},
			},
		})

	}
	return request
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

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

type dimensionality [][]string

func (d dimensionality) Len() int {
	return len(d)
}

func (d dimensionality) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d dimensionality) Less(i, j int) bool {
	dim1 := d[i]
	dim2 := d[j]

	for k := 0; k < min(len(dim1), len(dim2)); k++ {
		if dim1[k] != dim2[k] {
			return dim1[k] < dim2[k]
		}
	}

	return len(dim1) < len(dim2)
}

// normalizes a dimensionality lexicographically so that it can be compared
func normalizeDimensionality(dims [][]string) [][]string {
	// Convert to string for easier sorting
	sortedDimensions := make([][]string, len(dims))
	for i, v := range dims {
		sort.Strings(v)
		sortedDimensions[i] = v
	}
	sort.Sort(dimensionality(sortedDimensions))
	return sortedDimensions
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
	expectedDimensions := normalizeDimensionality(expected)
	actualDimensions := normalizeDimensionality(actual)
	assert.Equal(t, expectedDimensions, actualDimensions)
}

// cWMeasurementEqual returns true if CW Measurements are equal.
func cWMeasurementEqual(expected, actual cWMeasurement) bool {
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
	expectedDimensions := normalizeDimensionality(expected.Dimensions)
	actualDimensions := normalizeDimensionality(actual.Dimensions)
	return reflect.DeepEqual(expectedDimensions, actualDimensions)
}

// assertCWMeasurementEqual asserts whether CW Measurements are equal.
func assertCWMeasurementEqual(t *testing.T, expected, actual cWMeasurement) {
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
func assertCWMeasurementSliceEqual(t *testing.T, expected, actual []cWMeasurement) {
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
func assertCWMetricsEqual(t *testing.T, expected, actual *cWMetrics) {
	assert.Equal(t, expected.timestampMs, actual.timestampMs)
	assert.Equal(t, expected.fields, actual.fields)
	assert.Equal(t, len(expected.measurements), len(actual.measurements))
	assertCWMeasurementSliceEqual(t, expected.measurements, actual.measurements)
}

func TestTranslateOtToGroupedMetric(t *testing.T) {
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	oc := createMetricTestData()

	translator := newMetricTranslator(*config)

	noInstrLibMetric := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	instrLibMetric := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	ilm := instrLibMetric.ScopeMetrics().At(0)
	ilm.Scope().SetName("cloudwatch-lib")

	noNamespaceMetric := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	noNamespaceMetric.Resource().Attributes().Remove(conventions.AttributeServiceNamespace)
	noNamespaceMetric.Resource().Attributes().Remove(conventions.AttributeServiceName)

	counterSumMetrics := map[string]*metricInfo{
		"spanCounter": {
			value: float64(1),
			unit:  "Count",
		},
		"spanDoubleCounter": {
			value: 0.1,
			unit:  "Count",
		},
	}
	counterGaugeMetrics := map[string]*metricInfo{
		"spanGaugeCounter": {
			value: float64(1),
			unit:  "Count",
		},
		"spanGaugeDoubleCounter": {
			value: 0.1,
			unit:  "Count",
		},
	}
	timerMetrics := map[string]*metricInfo{
		"spanTimer": {
			value: &cWMetricStats{
				Count: 5,
				Sum:   15,
			},
			unit: "Seconds",
		},
	}

	testCases := []struct {
		testName          string
		metric            pmetric.ResourceMetrics
		counterLabels     map[string]string
		timerLabels       map[string]string
		expectedNamespace string
	}{
		{
			"w/ instrumentation library and namespace",
			instrLibMetric,
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
			noInstrLibMetric,
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
			noNamespaceMetric,
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
			setupDataPointCache()

			groupedMetrics := make(map[interface{}]*groupedMetric)
			err := translator.translateOTelToGroupedMetric(tc.metric, groupedMetrics, config)
			assert.Nil(t, err)
			assert.NotNil(t, groupedMetrics)
			assert.Equal(t, 3, len(groupedMetrics))

			for _, v := range groupedMetrics {
				assert.Equal(t, tc.expectedNamespace, v.metadata.namespace)
				switch {
				case v.metadata.metricDataType == pmetric.MetricTypeSum:
					assert.Equal(t, 2, len(v.metrics))
					assert.Equal(t, tc.counterLabels, v.labels)
					assert.Equal(t, counterSumMetrics, v.metrics)
				case v.metadata.metricDataType == pmetric.MetricTypeGauge:
					assert.Equal(t, 2, len(v.metrics))
					assert.Equal(t, tc.counterLabels, v.labels)
					assert.Equal(t, counterGaugeMetrics, v.metrics)
				case v.metadata.metricDataType == pmetric.MetricTypeHistogram:
					assert.Equal(t, 1, len(v.metrics))
					assert.Equal(t, tc.timerLabels, v.labels)
					assert.Equal(t, timerMetrics, v.metrics)
				default:
					assert.Fail(t, fmt.Sprintf("Unhandled metric type %s not expected", v.metadata.metricDataType))
				}
			}
		})
	}

	configTestCases := []struct {
		testName string
		retain   bool
	}{
		{
			"w/o retain initial value for delta",
			false,
		},
		{
			"w retain initial value for delta",
			true,
		},
	}

	for _, tc := range configTestCases {

		t.Run(tc.testName, func(t *testing.T) {
			setupDataPointCache()
			c := &Config{
				Namespace:                       "",
				DimensionRollupOption:           zeroAndSingleDimensionRollup,
				logger:                          zap.NewNop(),
				RetainInitialValueOfDeltaMetric: tc.retain,
			}
			setupDataPointCache()
			groupedMetrics := make(map[interface{}]*groupedMetric)
			err := translator.translateOTelToGroupedMetric(instrLibMetric, groupedMetrics, c)
			assert.Nil(t, err)
			assert.NotNil(t, groupedMetrics)

			for _, v := range groupedMetrics {
				assert.Equal(t, c.RetainInitialValueOfDeltaMetric, v.metadata.retainInitialValueForDelta)
			}
		})
	}

	t.Run("No metrics", func(t *testing.T) {
		oc = &agentmetricspb.ExportMetricsServiceRequest{
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
		rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
		groupedMetrics := make(map[interface{}]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(groupedMetrics))
	})
}

func TestTranslateCWMetricToEMF(t *testing.T) {
	testCases := map[string]struct {
		emfVersion          string
		measurements        []cWMeasurement
		expectedEMFLogEvent string
	}{
		"WithMeasurementAndEMFV1": {
			emfVersion: "1",
			measurements: []cWMeasurement{{
				Namespace:  "test-emf",
				Dimensions: [][]string{{oTellibDimensionKey}, {oTellibDimensionKey, "spanName"}},
				Metrics: []map[string]string{{
					"Name": "spanCounter",
					"Unit": "Count",
				}},
			}},
			expectedEMFLogEvent: "{\"OTelLib\":\"cloudwatch-otel\",\"Sources\":[\"cadvisor\",\"pod\",\"calculated\"],\"Version\":\"1\",\"_aws\":{\"CloudWatchMetrics\":[{\"Namespace\":\"test-emf\",\"Dimensions\":[[\"OTelLib\"],[\"OTelLib\",\"spanName\"]],\"Metrics\":[{\"Name\":\"spanCounter\",\"Unit\":\"Count\"}]}],\"Timestamp\":1596151098037},\"kubernetes\":{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]},\"spanCounter\":0,\"spanName\":\"test\"}",
		},
		"WithMeasurementAndEMFV0": {
			emfVersion: "0",
			measurements: []cWMeasurement{{
				Namespace:  "test-emf",
				Dimensions: [][]string{{oTellibDimensionKey}, {oTellibDimensionKey, "spanName"}},
				Metrics: []map[string]string{{
					"Name": "spanCounter",
					"Unit": "Count",
				}},
			}},
			expectedEMFLogEvent: "{\"CloudWatchMetrics\":[{\"Namespace\":\"test-emf\",\"Dimensions\":[[\"OTelLib\"],[\"OTelLib\",\"spanName\"]],\"Metrics\":[{\"Name\":\"spanCounter\",\"Unit\":\"Count\"}]}],\"OTelLib\":\"cloudwatch-otel\",\"Sources\":[\"cadvisor\",\"pod\",\"calculated\"],\"Timestamp\":\"1596151098037\",\"Version\":\"0\",\"kubernetes\":{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]},\"spanCounter\":0,\"spanName\":\"test\"}",
		},
		"WithNoMeasurementAndEMFV1": {
			emfVersion:          "1",
			measurements:        nil,
			expectedEMFLogEvent: "{\"OTelLib\":\"cloudwatch-otel\",\"Sources\":[\"cadvisor\",\"pod\",\"calculated\"],\"kubernetes\":{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]},\"spanCounter\":0,\"spanName\":\"test\"}",
		},
		"WithNoMeasurementAndEMFV0": {
			emfVersion:          "0",
			measurements:        nil,
			expectedEMFLogEvent: "{\"OTelLib\":\"cloudwatch-otel\",\"Sources\":[\"cadvisor\",\"pod\",\"calculated\"],\"Timestamp\":\"1596151098037\",\"kubernetes\":{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]},\"spanCounter\":0,\"spanName\":\"test\"}",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(_ *testing.T) {
			config := &Config{

				// include valid json string, a non-existing key, and keys whose value are not json/string
				ParseJSONEncodedAttributeValues: []string{"kubernetes", "Sources", "NonExistingAttributeKey", "spanName", "spanCounter"},
				Version:                         tc.emfVersion,
				logger:                          zap.NewNop(),
			}

			fields := map[string]interface{}{
				oTellibDimensionKey: "cloudwatch-otel",
				"spanName":          "test",
				"spanCounter":       0,
				"kubernetes":        "{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]}",
				"Sources":           "[\"cadvisor\",\"pod\",\"calculated\"]",
			}

			cloudwatchMetric := &cWMetrics{
				timestampMs:  int64(1596151098037),
				fields:       fields,
				measurements: tc.measurements,
			}

			emfLogEvent := translateCWMetricToEMF(cloudwatchMetric, config)

			assert.Equal(t, tc.expectedEMFLogEvent, *emfLogEvent.InputLogEvent.Message)
		})
	}

}

func TestTranslateGroupedMetricToCWMetric(t *testing.T) {
	timestamp := int64(1596151098037)
	namespace := "Namespace"
	testCases := []struct {
		testName           string
		groupedMetric      *groupedMetric
		metricDeclarations []*MetricDeclaration
		expectedCWMetric   *cWMetrics
	}{
		{
			"single metric w/o metric declarations",
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			nil,
			&cWMetrics{
				measurements: []cWMeasurement{
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
				timestampMs: timestamp,
				fields: map[string]interface{}{
					"label1":  "value1",
					"metric1": 1,
				},
			},
		},
		{
			"single metric w/ metric declarations",
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"label1"}, {"label1", "label2"}},
					MetricNameSelectors: []string{"metric.*"},
				},
			},
			&cWMetrics{
				measurements: []cWMeasurement{
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
				timestampMs: timestamp,
				fields: map[string]interface{}{
					"label1":  "value1",
					"metric1": 1,
				},
			},
		},
		{
			"multiple metrics w/o metric declarations",
			&groupedMetric{
				labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
					"metric2": {
						value: 200,
						unit:  "Count",
					},
					"metric3": {
						value: 3.14,
						unit:  "Seconds",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			nil,
			&cWMetrics{
				measurements: []cWMeasurement{
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
				timestampMs: timestamp,
				fields: map[string]interface{}{
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
			&groupedMetric{
				labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
					"metric2": {
						value: 200,
						unit:  "Count",
					},
					"metric3": {
						value: 3.14,
						unit:  "Seconds",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
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
			&cWMetrics{
				measurements: []cWMeasurement{
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
				timestampMs: timestamp,
				fields: map[string]interface{}{
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
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: nil,
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			nil,
			&cWMetrics{
				measurements: []cWMeasurement{
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1"}},
						Metrics:    nil,
					},
				},
				timestampMs: timestamp,
				fields: map[string]interface{}{
					"label1": "value1",
				},
			},
		},
		{
			"prometheus metrics",
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:      namespace,
						timestampMs:    timestamp,
						metricDataType: pmetric.MetricTypeGauge,
					},
					receiver: prometheusReceiver,
				},
			},
			nil,
			&cWMetrics{
				measurements: []cWMeasurement{
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
				timestampMs: timestamp,
				fields: map[string]interface{}{
					"label1":                  "value1",
					"metric1":                 1,
					fieldPrometheusMetricType: "gauge",
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
				err := decl.init(logger)
				assert.Nil(t, err)
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
		groupedMetric         *groupedMetric
		expectedMeasurement   cWMeasurement
	}{
		{
			"single metric, no dim rollup",
			"",
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			cWMeasurement{
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
			&groupedMetric{
				labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
					"metric2": {
						value: 200,
						unit:  "Count",
					},
					"metric3": {
						value: 3.14,
						unit:  "Seconds",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			cWMeasurement{
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
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			cWMeasurement{
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
			&groupedMetric{
				labels: map[string]string{
					"label2": "value2",
					"label1": "value1",
				},
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
					"metric2": {
						value: 200,
						unit:  "Count",
					},
					"metric3": {
						value: 3.14,
						unit:  "Seconds",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			cWMeasurement{
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
			&groupedMetric{
				labels: map[string]string{
					"label1": "value1",
				},
				metrics: nil,
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			},
			cWMeasurement{
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
			cWMeasurementGrp := groupedMetricToCWMeasurement(tc.groupedMetric, config)
			assertCWMeasurementEqual(t, tc.expectedMeasurement, cWMeasurementGrp)
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
			groupedMetric := &groupedMetric{
				labels: tc.labels,
				metrics: map[string]*metricInfo{
					"metric1": {
						value: 1,
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			}
			config := &Config{
				DimensionRollupOption: tc.dimensionRollupOption,
			}
			cWMeasurementGrp := groupedMetricToCWMeasurement(groupedMetric, config)
			assertDimsEqual(t, tc.expectedDims, cWMeasurementGrp.Dimensions)
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
	metrics := map[string]*metricInfo{
		"metric1": {
			value: 1,
			unit:  "Count",
		},
		"metric2": {
			value: 200,
			unit:  "Count",
		},
		"metric3": {
			value: 3.14,
			unit:  "Seconds",
		},
	}
	testCases := []struct {
		testName             string
		metricDeclarations   []*MetricDeclaration
		expectedMeasurements []cWMeasurement
	}{
		{
			"single metric declaration",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{"a"}, {"a", "c"}, {"b", "d"}},
					MetricNameSelectors: []string{"metric.*"},
				},
			},
			[]cWMeasurement{
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
			[]cWMeasurement{
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
			[]cWMeasurement{
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
			[]cWMeasurement{
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
			"empty dimension set matches",
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{}},
					MetricNameSelectors: []string{"metric(1|3)"},
				},
			}, []cWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{}},
					Metrics: []map[string]string{
						{
							"Name": "metric1",
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
			[]cWMeasurement{
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
			groupedMetric := &groupedMetric{
				labels:  labels,
				metrics: metrics,
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			}
			config := &Config{
				DimensionRollupOption: "",
				MetricDeclarations:    tc.metricDeclarations,
				logger:                logger,
			}
			for _, decl := range tc.metricDeclarations {
				err := decl.init(logger)
				assert.Nil(t, err)
			}

			cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
			assert.NotNil(t, cWMeasurements)
			assert.Equal(t, len(tc.expectedMeasurements), len(cWMeasurements))
			assertCWMeasurementSliceEqual(t, tc.expectedMeasurements, cWMeasurements)
		})
	}

	t.Run("No label match", func(t *testing.T) {
		groupedMetric := &groupedMetric{
			labels:  labels,
			metrics: metrics,
			metadata: cWMetricMetadata{
				groupedMetricMetadata: groupedMetricMetadata{
					namespace:   namespace,
					timestampMs: timestamp,
				},
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
			err := decl.init(zap.NewNop())
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
		groupedMetric := &groupedMetric{
			labels:  labels,
			metrics: metrics,
			metadata: cWMetricMetadata{
				groupedMetricMetadata: groupedMetricMetadata{
					namespace:   namespace,
					timestampMs: timestamp,
				},
			},
		}
		metricDeclarations := []*MetricDeclaration{
			{
				Dimensions:          [][]string{{"b"}, {"b", "d"}},
				MetricNameSelectors: []string{"metric4"},
			},
		}
		for _, decl := range metricDeclarations {
			err := decl.init(zap.NewNop())
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
		{
			"no labels with empty dimension",
			map[string]string{},
			[]*MetricDeclaration{
				{
					Dimensions:          [][]string{{}, {"a"}},
					MetricNameSelectors: []string{metricName},
				},
			},
			zeroAndSingleDimensionRollup,
			[][]string{{}},
		},
	}

	for _, tc := range rollupTestCases {
		t.Run(tc.testName, func(t *testing.T) {
			groupedMetric := &groupedMetric{
				labels: tc.labels,
				metrics: map[string]*metricInfo{
					(metricName): {
						value: int64(5),
						unit:  "Count",
					},
				},
				metadata: cWMetricMetadata{
					groupedMetricMetadata: groupedMetricMetadata{
						namespace:   namespace,
						timestampMs: timestamp,
					},
				},
			}
			for _, decl := range tc.metricDeclarations {
				err := decl.init(zap.NewNop())
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

func BenchmarkTranslateOtToGroupedMetricWithInstrLibrary(b *testing.B) {
	oc := createMetricTestData()
	rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics()
	ilm := ilms.At(0)
	ilm.Scope().SetName("cloudwatch-lib")
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetric := make(map[interface{}]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetric, config)
		assert.Nil(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithoutConfigReplacePattern(b *testing.B) {
	oc := createMetricTestData()
	rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics()
	ilm := ilms.At(0)
	ilm.Scope().SetName("cloudwatch-lib")
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		LogGroupName:          "group.no.replace.pattern",
		LogStreamName:         "stream.no.replace.pattern",
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.Nil(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithConfigReplaceWithResource(b *testing.B) {
	oc := createMetricTestData()
	rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics()
	ilm := ilms.At(0)
	ilm.Scope().SetName("cloudwatch-lib")
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		LogGroupName:          "group.{ClusterName}",
		LogStreamName:         "stream.no.replace.pattern",
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.Nil(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithConfigReplaceWithLabel(b *testing.B) {
	oc := createMetricTestData()
	rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics()
	ilm := ilms.At(0)
	ilm.Scope().SetName("cloudwatch-lib")
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		LogGroupName:          "group.no.replace.pattern",
		LogStreamName:         "stream.{PodName}",
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.Nil(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithoutInstrLibrary(b *testing.B) {
	oc := createMetricTestData()
	rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics().At(0)
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.Nil(b, err)
	}
}

func BenchmarkTranslateGroupedMetricToCWMetric(b *testing.B) {
	groupedMetric := &groupedMetric{
		labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
			"label3": "value3",
		},
		metrics: map[string]*metricInfo{
			"metric1": {
				value: 1,
				unit:  "Count",
			},
			"metric2": {
				value: 200,
				unit:  "Seconds",
			},
		},
		metadata: cWMetricMetadata{
			groupedMetricMetadata: groupedMetricMetadata{
				namespace:   "Namespace",
				timestampMs: int64(1596151098037),
			},
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
	groupedMetric := &groupedMetric{
		labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
			"label3": "value3",
		},
		metrics: map[string]*metricInfo{
			"metric1": {
				value: 1,
				unit:  "Count",
			},
			"metric2": {
				value: 200,
				unit:  "Seconds",
			},
		},
		metadata: cWMetricMetadata{
			groupedMetricMetadata: groupedMetricMetadata{
				namespace:   "Namespace",
				timestampMs: int64(1596151098037),
			},
		},
	}
	m := &MetricDeclaration{
		Dimensions:          [][]string{{"label1"}, {"label2"}},
		MetricNameSelectors: []string{"metric1", "metric2"},
	}
	logger := zap.NewNop()
	err := m.init(logger)
	assert.Nil(b, err)
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
	cwMeasurement := cWMeasurement{
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

	met := &cWMetrics{
		timestampMs:  timestamp,
		fields:       fields,
		measurements: []cWMeasurement{cwMeasurement},
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		translateCWMetricToEMF(met, &Config{})
	}
}

type testMetric struct {
	metricNames          []string
	metricValues         [][]float64
	resourceAttributeMap map[string]interface{}
	attributeMap         map[string]interface{}
}

type logGroupStreamTest struct {
	name             string
	inputMetrics     pmetric.Metrics
	inLogGroupName   string
	inLogStreamName  string
	outLogGroupName  string
	outLogStreamName string
}

var (
	logGroupStreamTestCases = []logGroupStreamTest{
		{
			name: "log_group_stream_expect_same",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
			}),
			inLogGroupName:   "test-log-group",
			inLogStreamName:  "test-log-stream",
			outLogGroupName:  "test-log-group",
			outLogStreamName: "test-log-stream",
		},
		{
			name: "log_group_pattern_from_resource",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				resourceAttributeMap: map[string]interface{}{
					"ClusterName": "test-cluster",
					"PodName":     "test-pod",
				},
			}),
			inLogGroupName:   "test-log-group-{ClusterName}",
			inLogStreamName:  "test-log-stream",
			outLogGroupName:  "test-log-group-test-cluster",
			outLogStreamName: "test-log-stream",
		},
		{
			name: "log_stream_pattern_from_resource",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				resourceAttributeMap: map[string]interface{}{
					"ClusterName": "test-cluster",
					"PodName":     "test-pod",
				},
			}),
			inLogGroupName:   "test-log-group",
			inLogStreamName:  "test-log-stream-{PodName}",
			outLogGroupName:  "test-log-group",
			outLogStreamName: "test-log-stream-test-pod",
		},
		{
			name: "log_group_pattern_from_label",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				attributeMap: map[string]interface{}{
					"ClusterName": "test-cluster",
					"PodName":     "test-pod",
				},
			}),
			inLogGroupName:   "test-log-group-{ClusterName}",
			inLogStreamName:  "test-log-stream",
			outLogGroupName:  "test-log-group-test-cluster",
			outLogStreamName: "test-log-stream",
		},
		{
			name: "log_stream_pattern_from_label",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				attributeMap: map[string]interface{}{
					"ClusterName": "test-cluster",
					"PodName":     "test-pod",
				},
			}),
			inLogGroupName:   "test-log-group",
			inLogStreamName:  "test-log-stream-{PodName}",
			outLogGroupName:  "test-log-group",
			outLogStreamName: "test-log-stream-test-pod",
		},
		{
			name: "config_pattern_from_both_attributes",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				resourceAttributeMap: map[string]interface{}{
					"ClusterName": "test-cluster",
				},
				attributeMap: map[string]interface{}{
					"PodName": "test-pod",
				},
			}),
			inLogGroupName:   "test-log-group-{ClusterName}",
			inLogStreamName:  "test-log-stream-{PodName}",
			outLogGroupName:  "test-log-group-test-cluster",
			outLogStreamName: "test-log-stream-test-pod",
		},
		{
			name: "config_pattern_missing_from_both_attributes",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
			}),
			inLogGroupName:   "test-log-group-{ClusterName}",
			inLogStreamName:  "test-log-stream-{PodName}",
			outLogGroupName:  "test-log-group-undefined",
			outLogStreamName: "test-log-stream-undefined",
		},
		{
			name: "config_pattern_group_missing_stream_present",
			inputMetrics: generateTestMetrics(testMetric{
				metricNames:  []string{"metric_1", "metric_2"},
				metricValues: [][]float64{{100}, {4}},
				attributeMap: map[string]interface{}{
					"PodName": "test-pod",
				},
			}),
			inLogGroupName:   "test-log-group-{ClusterName}",
			inLogStreamName:  "test-log-stream-{PodName}",
			outLogGroupName:  "test-log-group-undefined",
			outLogStreamName: "test-log-stream-test-pod",
		},
	}
)

func TestTranslateOtToGroupedMetricForLogGroupAndStream(t *testing.T) {
	for _, test := range logGroupStreamTestCases {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{
				Namespace:             "",
				LogGroupName:          test.inLogGroupName,
				LogStreamName:         test.inLogStreamName,
				DimensionRollupOption: zeroAndSingleDimensionRollup,
				logger:                zap.NewNop(),
			}

			translator := newMetricTranslator(*config)

			groupedMetrics := make(map[interface{}]*groupedMetric)

			rm := test.inputMetrics.ResourceMetrics().At(0)
			err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
			assert.Nil(t, err)

			assert.NotNil(t, groupedMetrics)
			assert.Equal(t, 1, len(groupedMetrics))

			for _, actual := range groupedMetrics {
				assert.Equal(t, test.outLogGroupName, actual.metadata.logGroup)
				assert.Equal(t, test.outLogStreamName, actual.metadata.logStream)
			}
		})
	}
}

func generateTestMetrics(tm testMetric) pmetric.Metrics {
	md := pmetric.NewMetrics()
	now := time.Now()

	rm := md.ResourceMetrics().AppendEmpty()
	//nolint:errcheck
	rm.Resource().Attributes().FromRaw(tm.resourceAttributeMap)
	ms := rm.ScopeMetrics().AppendEmpty().Metrics()

	for i, name := range tm.metricNames {
		m := ms.AppendEmpty()
		m.SetName(name)
		g := m.SetEmptyGauge()
		for _, value := range tm.metricValues[i] {
			dp := g.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Second)))
			dp.SetDoubleValue(value)
			//nolint:errcheck
			dp.Attributes().FromRaw(tm.attributeMap)
		}
	}
	return md
}
