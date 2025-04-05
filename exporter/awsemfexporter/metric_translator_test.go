// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
)

func createTestResourceMetrics() pmetric.ResourceMetrics {
	rm := pmetric.NewResourceMetrics()
	rm.Resource().Attributes().PutStr(occonventions.AttributeExporterVersion, "SomeVersion")
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "myServiceName")
	rm.Resource().Attributes().PutStr(conventions.AttributeServiceNamespace, "myServiceNS")
	rm.Resource().Attributes().PutStr("ClusterName", "myCluster")
	rm.Resource().Attributes().PutStr("PodName", "myPod")
	rm.Resource().Attributes().PutStr(attributeReceiver, prometheusReceiver)
	sm := rm.ScopeMetrics().AppendEmpty()

	m := sm.Metrics().AppendEmpty()
	m.SetName("spanGaugeCounter")
	m.SetDescription("Counting all the spans")
	m.SetUnit("Count")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(100_000_000))
	dp.SetIntValue(1)
	dp.Attributes().PutStr("spanName", "testSpan")
	dp.Attributes().PutStr("isItAnError", "false")

	m = sm.Metrics().AppendEmpty()
	m.SetName("spanGaugeDoubleCounter")
	m.SetDescription("Counting all the spans")
	m.SetUnit("Count")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(100_000_000))
	dp.SetDoubleValue(0.1)
	dp.Attributes().PutStr("spanName", "testSpan")
	dp.Attributes().PutStr("isItAnError", "false")

	m = sm.Metrics().AppendEmpty()
	m.SetName("spanTimer")
	m.SetDescription("How long the spans take")
	m.SetUnit("Seconds")
	hdp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
	hdp.SetTimestamp(pcommon.Timestamp(100_000_000))
	hdp.SetCount(5)
	hdp.SetSum(15)
	hdp.ExplicitBounds().FromRaw([]float64{0, 10})
	hdp.BucketCounts().FromRaw([]uint64{0, 4, 1})
	hdp.Attributes().PutStr("spanName", "testSpan")

	m = sm.Metrics().AppendEmpty()
	m.SetName("spanTimer")
	m.SetDescription("How long the spans take")
	m.SetUnit("Seconds")
	sdp := m.SetEmptySummary().DataPoints().AppendEmpty()
	sdp.SetTimestamp(pcommon.Timestamp(100_000_000))
	sdp.SetCount(5)
	sdp.SetSum(15)
	q1 := sdp.QuantileValues().AppendEmpty()
	q1.SetQuantile(0)
	q1.SetValue(1)
	q2 := sdp.QuantileValues().AppendEmpty()
	q2.SetQuantile(1)
	q2.SetValue(5)

	for i := 0; i < 2; i++ {
		m = sm.Metrics().AppendEmpty()
		m.SetName("spanCounter")
		m.SetDescription("Counting all the spans")
		m.SetUnit("Count")
		sum := m.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp = sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(i * 100_000_000))
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("spanName", "testSpan")
		dp.Attributes().PutStr("isItAnError", "false")

		m = sm.Metrics().AppendEmpty()
		m.SetName("spanDoubleCounter")
		m.SetDescription("Counting all the spans")
		m.SetUnit("Count")
		sum = m.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp = sum.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(i * 100_000_000))
		dp.SetDoubleValue(float64(i) * 0.1)
		dp.Attributes().PutStr("spanName", "testSpan")
		dp.Attributes().PutStr("isItAnError", "false")
	}

	return rm
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
func hashMetricSlice(metricSlice []cWMetricInfo) []string {
	// Convert to string for easier sorting
	stringified := make([]string, len(metricSlice))
	for i, v := range metricSlice {
		stringified[i] = fmt.Sprint(v.Name) + "," + fmt.Sprint(v.Unit) + "," + fmt.Sprint(v.StorageResolution)
	}
	// Sort across metrics for equality checking
	sort.Strings(stringified)
	return stringified
}

// assertDimsEqual asserts whether dimension sets are equal
// (i.e. has same sets of dimensions), regardless of order.
func assertDimsEqual(t *testing.T, expected, actual [][]string) {
	assert.Len(t, actual, len(expected))
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
	assert.Len(t, actual.Metrics, len(expected.Metrics))
	expectedHashSlice := hashMetricSlice(expected.Metrics)
	actualHashSlice := hashMetricSlice(actual.Metrics)
	assert.Equal(t, expectedHashSlice, actualHashSlice)

	// Check dimensions
	assertDimsEqual(t, expected.Dimensions, actual.Dimensions)
}

// assertCWMeasurementSliceEqual asserts whether CW Measurements are equal, regardless of order.
func assertCWMeasurementSliceEqual(t *testing.T, expected, actual []cWMeasurement) {
	assert.Len(t, actual, len(expected))
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
	assert.Len(t, actual.measurements, len(expected.measurements))
	assertCWMeasurementSliceEqual(t, expected.measurements, actual.measurements)
}

func TestTranslateOtToGroupedMetric(t *testing.T) {
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)
	defer require.NoError(t, translator.Shutdown())

	noInstrLibMetric := createTestResourceMetrics()
	instrLibMetric := createTestResourceMetrics()
	ilm := instrLibMetric.ScopeMetrics().At(0)
	ilm.Scope().SetName("cloudwatch-lib")

	noNamespaceMetric := createTestResourceMetrics()
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
			groupedMetrics := make(map[any]*groupedMetric)
			err := translator.translateOTelToGroupedMetric(tc.metric, groupedMetrics, config)
			assert.NoError(t, err)
			assert.NotNil(t, groupedMetrics)
			assert.Len(t, groupedMetrics, 3)

			for _, v := range groupedMetrics {
				assert.Equal(t, tc.expectedNamespace, v.metadata.namespace)
				switch {
				case v.metadata.metricDataType == pmetric.MetricTypeSum:
					assert.Len(t, v.metrics, 2)
					assert.Equal(t, tc.counterLabels, v.labels)
					assert.Equal(t, counterSumMetrics, v.metrics)
				case v.metadata.metricDataType == pmetric.MetricTypeGauge:
					assert.Len(t, v.metrics, 2)
					assert.Equal(t, tc.counterLabels, v.labels)
					assert.Equal(t, counterGaugeMetrics, v.metrics)
				case v.metadata.metricDataType == pmetric.MetricTypeHistogram:
					assert.Len(t, v.metrics, 1)
					assert.Equal(t, tc.timerLabels, v.labels)
					assert.Equal(t, timerMetrics, v.metrics)
				default:
					assert.Fail(t, fmt.Sprintf("Unhandled metric type %s not expected", v.metadata.metricDataType))
				}
			}
		})
	}

	t.Run("No metrics", func(t *testing.T) {
		rm := pmetric.NewResourceMetrics()
		rm.Resource().Attributes().PutStr(conventions.AttributeServiceName, "myServiceName")
		rm.Resource().Attributes().PutStr(occonventions.AttributeExporterVersion, "SomeVersion")
		groupedMetrics := make(map[any]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.NoError(t, err)
		assert.Empty(t, groupedMetrics)
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
				Metrics: []cWMetricInfo{{
					Name:              "spanCounter",
					Unit:              "Count",
					StorageResolution: 1,
				}},
			}},
			expectedEMFLogEvent: "{\"OTelLib\":\"cloudwatch-otel\",\"Sources\":[\"cadvisor\",\"pod\",\"calculated\"],\"Version\":\"1\",\"_aws\":{\"CloudWatchMetrics\":[{\"Namespace\":\"test-emf\",\"Dimensions\":[[\"OTelLib\"],[\"OTelLib\",\"spanName\"]],\"Metrics\":[{\"Name\":\"spanCounter\",\"Unit\":\"Count\",\"StorageResolution\":1}]}],\"Timestamp\":1596151098037},\"kubernetes\":{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]},\"spanCounter\":0,\"spanName\":\"test\"}",
		},
		"WithMeasurementAndEMFV0": {
			emfVersion: "0",
			measurements: []cWMeasurement{{
				Namespace:  "test-emf",
				Dimensions: [][]string{{oTellibDimensionKey}, {oTellibDimensionKey, "spanName"}},
				Metrics: []cWMetricInfo{{
					Name:              "spanCounter",
					Unit:              "Count",
					StorageResolution: 60,
				}},
			}},
			expectedEMFLogEvent: "{\"CloudWatchMetrics\":[{\"Namespace\":\"test-emf\",\"Dimensions\":[[\"OTelLib\"],[\"OTelLib\",\"spanName\"]],\"Metrics\":[{\"Name\":\"spanCounter\",\"Unit\":\"Count\",\"StorageResolution\":60}]}],\"OTelLib\":\"cloudwatch-otel\",\"Sources\":[\"cadvisor\",\"pod\",\"calculated\"],\"Timestamp\":\"1596151098037\",\"Version\":\"0\",\"kubernetes\":{\"container_name\":\"cloudwatch-agent\",\"docker\":{\"container_id\":\"fc1b0a4c3faaa1808e187486a3a90cbea883dccaf2e2c46d4069d663b032a1ca\"},\"host\":\"ip-192-168-58-245.ec2.internal\",\"labels\":{\"controller-revision-hash\":\"5bdbf497dc\",\"name\":\"cloudwatch-agent\",\"pod-template-generation\":\"1\"},\"namespace_name\":\"amazon-cloudwatch\",\"pod_id\":\"e23f3413-af2e-4a98-89e0-5df2251e7f05\",\"pod_name\":\"cloudwatch-agent-26bl6\",\"pod_owners\":[{\"owner_kind\":\"DaemonSet\",\"owner_name\":\"cloudwatch-agent\"}]},\"spanCounter\":0,\"spanName\":\"test\"}",
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

			fields := map[string]any{
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

			emfLogEvent, err := translateCWMetricToEMF(cloudwatchMetric, config)
			require.NoError(t, err)

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
						Metrics: []cWMetricInfo{
							{
								Name:              "metric1",
								Unit:              "Count",
								StorageResolution: 60,
							},
						},
					},
				},
				timestampMs: timestamp,
				fields: map[string]any{
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
						Metrics: []cWMetricInfo{
							{
								Name:              "metric1",
								Unit:              "Count",
								StorageResolution: 60,
							},
						},
					},
				},
				timestampMs: timestamp,
				fields: map[string]any{
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
						Metrics: []cWMetricInfo{
							{
								Name:              "metric1",
								Unit:              "Count",
								StorageResolution: 60,
							},
							{
								Name:              "metric2",
								Unit:              "Count",
								StorageResolution: 60,
							},
							{
								Name:              "metric3",
								Unit:              "Seconds",
								StorageResolution: 60,
							},
						},
					},
				},
				timestampMs: timestamp,
				fields: map[string]any{
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
						Metrics: []cWMetricInfo{
							{
								Name:              "metric1",
								Unit:              "Count",
								StorageResolution: 60,
							},
						},
					},
					{
						Namespace:  namespace,
						Dimensions: [][]string{{"label1", "label2"}},
						Metrics: []cWMetricInfo{
							{
								Name:              "metric2",
								Unit:              "Count",
								StorageResolution: 60,
							},
						},
					},
				},
				timestampMs: timestamp,
				fields: map[string]any{
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
				fields: map[string]any{
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
						Metrics: []cWMetricInfo{
							{
								Name:              "metric1",
								Unit:              "Count",
								StorageResolution: 60,
							},
						},
					},
				},
				timestampMs: timestamp,
				fields: map[string]any{
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
				assert.NoError(t, err)
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
				Metrics: []cWMetricInfo{
					{
						Name:              "metric1",
						Unit:              "Count",
						StorageResolution: 60,
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
				Metrics: []cWMetricInfo{
					{
						Name:              "metric1",
						Unit:              "Count",
						StorageResolution: 60,
					},
					{
						Name:              "metric2",
						Unit:              "Count",
						StorageResolution: 60,
					},
					{
						Name:              "metric3",
						Unit:              "Seconds",
						StorageResolution: 60,
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
				Metrics: []cWMetricInfo{
					{
						Name:              "metric1",
						Unit:              "Count",
						StorageResolution: 60,
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
				Metrics: []cWMetricInfo{
					{
						Name:              "metric1",
						Unit:              "Count",
						StorageResolution: 60,
					},
					{
						Name:              "metric2",
						Unit:              "Count",
						StorageResolution: 60,
					},
					{
						Name:              "metric3",
						Unit:              "Seconds",
						StorageResolution: 60,
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
					Metrics: []cWMetricInfo{
						{
							Name:              "metric1",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric2",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric3",
							Unit:              "Seconds",
							StorageResolution: 60,
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
					Metrics: []cWMetricInfo{
						{
							Name:              "metric1",
							Unit:              "Count",
							StorageResolution: 60,
						},
					},
				},
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}, {"b"}},
					Metrics: []cWMetricInfo{
						{
							Name:              "metric2",
							Unit:              "Count",
							StorageResolution: 60,
						},
					},
				},
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}},
					Metrics: []cWMetricInfo{
						{
							Name:              "metric3",
							Unit:              "Seconds",
							StorageResolution: 60,
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
					Metrics: []cWMetricInfo{
						{
							Name:              "metric1",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric2",
							Unit:              "Count",
							StorageResolution: 60,
						},
					},
				},
				{
					Namespace:  namespace,
					Dimensions: [][]string{{"a"}},
					Metrics: []cWMetricInfo{
						{
							Name:              "metric3",
							Unit:              "Seconds",
							StorageResolution: 60,
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
					Metrics: []cWMetricInfo{
						{
							Name:              "metric1",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric2",
							Unit:              "Count",
							StorageResolution: 60,
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
			},
			[]cWMeasurement{
				{
					Namespace:  namespace,
					Dimensions: [][]string{{}},
					Metrics: []cWMetricInfo{
						{
							Name:              "metric1",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric3",
							Unit:              "Seconds",
							StorageResolution: 60,
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
					Metrics: []cWMetricInfo{
						{
							Name:              "metric1",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric2",
							Unit:              "Count",
							StorageResolution: 60,
						},
						{
							Name:              "metric3",
							Unit:              "Seconds",
							StorageResolution: 60,
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
				assert.NoError(t, err)
			}

			cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
			assert.NotNil(t, cWMeasurements)
			assert.Len(t, cWMeasurements, len(tc.expectedMeasurements))
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
			assert.NoError(t, err)
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
		assert.Len(t, log.Context, 2)
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
			assert.NoError(t, err)
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
			assert.Len(t, log.Context, 1)
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
				assert.NoError(t, err)
			}
			config := &Config{
				DimensionRollupOption: tc.dimensionRollupOption,
				MetricDeclarations:    tc.metricDeclarations,
				logger:                zap.NewNop(),
			}

			cWMeasurements := groupedMetricToCWMeasurementsWithFilters(groupedMetric, config)
			if len(tc.expectedDims) == 0 {
				assert.Empty(t, cWMeasurements)
			} else {
				assert.Len(t, cWMeasurements, 1)
				dims := cWMeasurements[0].Dimensions
				assertDimsEqual(t, tc.expectedDims, dims)
			}
		})
	}
}

func BenchmarkTranslateOtToGroupedMetricWithInstrLibrary(b *testing.B) {
	rm := createTestResourceMetrics()
	ilms := rm.ScopeMetrics()
	ilm := ilms.At(0)
	ilm.Scope().SetName("cloudwatch-lib")
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)
	defer require.NoError(b, translator.Shutdown())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetric := make(map[any]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetric, config)
		assert.NoError(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithoutConfigReplacePattern(b *testing.B) {
	rm := createTestResourceMetrics()
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
	defer require.NoError(b, translator.Shutdown())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[any]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.NoError(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithConfigReplaceWithResource(b *testing.B) {
	rm := createTestResourceMetrics()
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
	defer require.NoError(b, translator.Shutdown())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[any]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.NoError(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithConfigReplaceWithLabel(b *testing.B) {
	rm := createTestResourceMetrics()
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
	defer require.NoError(b, translator.Shutdown())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[any]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.NoError(b, err)
	}
}

func BenchmarkTranslateOtToGroupedMetricWithoutInstrLibrary(b *testing.B) {
	rm := createTestResourceMetrics()
	config := &Config{
		Namespace:             "",
		DimensionRollupOption: zeroAndSingleDimensionRollup,
		logger:                zap.NewNop(),
	}
	translator := newMetricTranslator(*config)
	defer require.NoError(b, translator.Shutdown())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[any]*groupedMetric)
		err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
		assert.NoError(b, err)
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
	assert.NoError(b, err)
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
		Metrics: []cWMetricInfo{{
			Name: "spanCounter",
			Unit: "Count",
		}},
	}
	timestamp := int64(1596151098037)
	fields := make(map[string]any)
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
		_, err := translateCWMetricToEMF(met, &Config{})
		require.NoError(b, err)
	}
}

type testMetric struct {
	metricNames          []string
	metricValues         [][]float64
	resourceAttributeMap map[string]any
	attributeMap         map[string]any
}

type logGroupStreamTest struct {
	name             string
	inputMetrics     pmetric.Metrics
	inLogGroupName   string
	inLogStreamName  string
	outLogGroupName  string
	outLogStreamName string
}

var logGroupStreamTestCases = []logGroupStreamTest{
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
			resourceAttributeMap: map[string]any{
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
			resourceAttributeMap: map[string]any{
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
			attributeMap: map[string]any{
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
			attributeMap: map[string]any{
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
			resourceAttributeMap: map[string]any{
				"ClusterName": "test-cluster",
			},
			attributeMap: map[string]any{
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
			attributeMap: map[string]any{
				"PodName": "test-pod",
			},
		}),
		inLogGroupName:   "test-log-group-{ClusterName}",
		inLogStreamName:  "test-log-stream-{PodName}",
		outLogGroupName:  "test-log-group-undefined",
		outLogStreamName: "test-log-stream-test-pod",
	},
}

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
			defer require.NoError(t, translator.Shutdown())

			groupedMetrics := make(map[any]*groupedMetric)

			rm := test.inputMetrics.ResourceMetrics().At(0)
			err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
			assert.NoError(t, err)

			assert.NotNil(t, groupedMetrics)
			assert.Len(t, groupedMetrics, 1)

			for _, actual := range groupedMetrics {
				assert.Equal(t, test.outLogGroupName, actual.metadata.logGroup)
				assert.Equal(t, test.outLogStreamName, actual.metadata.logStream)
			}
		})
	}
}

func TestTranslateOtToGroupedMetricForInitialDeltaValue(t *testing.T) {
	for _, test := range logGroupStreamTestCases {
		t.Run(test.name, func(t *testing.T) {
			config := &Config{
				Namespace:                       "",
				LogGroupName:                    test.inLogGroupName,
				LogStreamName:                   test.inLogStreamName,
				DimensionRollupOption:           zeroAndSingleDimensionRollup,
				logger:                          zap.NewNop(),
				RetainInitialValueOfDeltaMetric: true,
			}

			translator := newMetricTranslator(*config)

			groupedMetrics := make(map[any]*groupedMetric)

			rm := test.inputMetrics.ResourceMetrics().At(0)
			err := translator.translateOTelToGroupedMetric(rm, groupedMetrics, config)
			assert.NoError(t, err)

			assert.NotNil(t, groupedMetrics)
			assert.Len(t, groupedMetrics, 1)

			for _, actual := range groupedMetrics {
				assert.True(t, actual.metadata.retainInitialValueForDelta)
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
