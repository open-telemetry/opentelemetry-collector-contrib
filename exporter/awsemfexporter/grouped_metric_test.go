// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var (
	logGroup      = "logGroup"
	logStreamName = "logStream"
	testCfg       = createDefaultConfig().(*Config)
)

func TestAddToGroupedMetric(t *testing.T) {
	namespace := "namespace"
	instrumentationLibName := "cloudwatch-otel"
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	testCases := []struct {
		name               string
		metric             pmetric.Metrics
		expectedMetricType pmetric.MetricType
		expectedLabels     map[string]string
		expectedMetricInfo map[string]*metricInfo
	}{
		{
			name:               "Double gauge",
			metric:             generateTestGaugeMetric("foo", doubleValueType),
			expectedMetricType: pmetric.MetricTypeGauge,
			expectedLabels:     map[string]string{"label1": "value1"},
			expectedMetricInfo: map[string]*metricInfo{
				"foo": {
					value: 0.1,
					unit:  "Count",
				},
			},
		},
		{
			name:               "Int sum",
			metric:             generateTestSumMetric("foo", intValueType),
			expectedMetricType: pmetric.MetricTypeSum,
			expectedLabels:     map[string]string{"label1": "value1"},
			expectedMetricInfo: map[string]*metricInfo{
				"foo": {
					value: float64(1),
					unit:  "Count",
				},
			},
		},
		{
			name:               "Histogram",
			metric:             generateTestHistogramMetric("foo"),
			expectedMetricType: pmetric.MetricTypeHistogram,
			expectedLabels:     map[string]string{"label1": "value1"},
			expectedMetricInfo: map[string]*metricInfo{
				"foo": {
					value: &cWMetricStats{
						Count: 18,
						Sum:   35.0,
					},
					unit: "Seconds",
				},
			},
		},
		{
			name:               "Summary",
			metric:             generateTestSummaryMetric("foo"),
			expectedMetricType: pmetric.MetricTypeSummary,
			expectedLabels:     map[string]string{"label1": "value1"},
			expectedMetricInfo: map[string]*metricInfo{
				"foo": {
					value: &cWMetricStats{
						Min:   1,
						Max:   5,
						Count: 5,
						Sum:   15,
					},
					unit: "Seconds",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			emfCalcs := setupEmfCalculators()
			defer require.NoError(t, shutdownEmfCalculators(emfCalcs))

			groupedMetrics := make(map[any]*groupedMetric)
			rms := tc.metric.ResourceMetrics()
			ilms := rms.At(0).ScopeMetrics()
			metrics := ilms.At(0).Metrics()

			assert.Equal(t, 1, rms.Len())
			assert.Equal(t, 1, ilms.Len())

			for i := 0; i < metrics.Len(); i++ {
				err := addToGroupedMetric(metrics.At(i), groupedMetrics,
					generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type(), 0),
					true,
					nil,
					testCfg,
					emfCalcs)
				assert.NoError(t, err)
			}

			assert.Len(t, groupedMetrics, 1)
			for _, v := range groupedMetrics {
				assert.Len(t, v.metrics, len(tc.expectedMetricInfo))
				assert.Equal(t, tc.expectedMetricInfo, v.metrics)
				assert.Len(t, v.labels, 1)
				assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, tc.expectedMetricType, 0), v.metadata)
				assert.Equal(t, tc.expectedLabels, v.labels)
			}
		})
	}

	t.Run("Add multiple different metrics", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))

		groupedMetrics := make(map[any]*groupedMetric)
		generateMetrics := []pmetric.Metrics{
			generateTestGaugeMetric("int-gauge", intValueType),
			generateTestGaugeMetric("double-gauge", doubleValueType),
			generateTestHistogramMetric("histogram"),
			generateTestSumMetric("int-sum", intValueType),
			generateTestSumMetric("double-sum", doubleValueType),
			generateTestSummaryMetric("summary"),
		}

		finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)
		rms := finalOtelMetrics.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 9, metrics.Len())

		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(metrics.At(i),
				groupedMetrics,
				generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type(), 0),
				true,
				nil,
				testCfg,
				emfCalcs)
			assert.NoError(t, err)
		}

		assert.Len(t, groupedMetrics, 4)
		for _, group := range groupedMetrics {
			for metricName, metricInfo := range group.metrics {
				switch metricName {
				case "int-gauge", "double-gauge":
					assert.Len(t, group.metrics, 2)
					assert.Equal(t, "Count", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeGauge, 0), group.metadata)
				case "int-sum", "double-sum":
					assert.Len(t, group.metrics, 2)
					assert.Equal(t, "Count", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSum, 0), group.metadata)
				case "histogram":
					assert.Len(t, group.metrics, 1)
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeHistogram, 0), group.metadata)
				case "summary":
					assert.Len(t, group.metrics, 1)
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSummary, 0), group.metadata)
				default:
					assert.Fail(t, fmt.Sprintf("Unhandled metric %s not expected", metricName))
				}
				expectedLabels := map[string]string{
					"label1": "value1",
				}
				assert.Equal(t, expectedLabels, group.labels)
			}
		}
	})

	t.Run("Add multiple different metrics with NaN and Inf types", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))

		groupedMetrics := make(map[any]*groupedMetric)
		generateMetrics := []pmetric.Metrics{
			generateTestGaugeMetric("int-gauge", intValueType),
			generateTestGaugeMetric("double-gauge", doubleValueType),
			generateTestHistogramMetric("histogram"),
			generateTestSumMetric("int-sum", intValueType),
			generateTestSumMetric("double-sum", doubleValueType),
			generateTestSummaryMetric("summary"),
			// We do not expect these to be added to the grouped metric. Metrics with NaN or Inf values should be dropped.
			generateTestGaugeMetricNaN("double-gauge-nan"),
			generateTestExponentialHistogramMetricWithNaNs("expo-with-nan"),
			generateTestHistogramMetricWithNaNs("histo-with-nan"),
			generateTestSummaryMetricWithNaN("sum-with-nan"),
			generateTestGaugeMetricInf("double-gauge-inf"),
			generateTestExponentialHistogramMetricWithInfs("expo-with-inf"),
			generateTestHistogramMetricWithInfs("histo-with-inf"),
			generateTestSummaryMetricWithInf("sum-with-inf"),
		}

		finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)
		rms := finalOtelMetrics.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()
		// Verify if all metrics are generated, including NaN, Inf values
		require.Equal(t, 19, metrics.Len(), "mock metric creation failed")
		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(metrics.At(i),
				groupedMetrics,
				generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type(), 0),
				true,
				nil,
				testCfg,
				emfCalcs)
			assert.NoError(t, err)
		}

		assert.Len(t, groupedMetrics, 4)
		for _, group := range groupedMetrics {
			for metricName, metricInfo := range group.metrics {
				switch metricName {
				case "int-gauge", "double-gauge":
					assert.Len(t, group.metrics, 2)
					assert.Equal(t, "Count", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeGauge, 0), group.metadata)
				case "int-sum", "double-sum":
					assert.Len(t, group.metrics, 2)
					assert.Equal(t, "Count", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSum, 0), group.metadata)
				case "histogram":
					assert.Len(t, group.metrics, 1)
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeHistogram, 0), group.metadata)
				case "summary":
					assert.Len(t, group.metrics, 1)
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSummary, 0), group.metadata)
				default:
					assert.Fail(t, fmt.Sprintf("Unhandled metric %s not expected", metricName))
				}
				expectedLabels := map[string]string{
					"label1": "value1",
				}
				assert.Equal(t, expectedLabels, group.labels)
			}
		}
	})

	t.Run("Add same metric but different log group", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
		groupedMetrics := make(map[any]*groupedMetric)
		otelMetrics := generateTestGaugeMetric("int-gauge", "int")
		ilms := otelMetrics.ResourceMetrics().At(0).ScopeMetrics()
		metric := ilms.At(0).Metrics().At(0)

		metricMetadata1 := generateTestMetricMetadata(namespace, timestamp, "log-group-1", logStreamName, instrumentationLibName, metric.Type(), 0)
		err := addToGroupedMetric(metric,
			groupedMetrics,
			metricMetadata1,
			true,
			nil,
			testCfg,
			emfCalcs)
		assert.NoError(t, err)

		metricMetadata2 := generateTestMetricMetadata(namespace,
			timestamp,
			"log-group-2",
			logStreamName,
			instrumentationLibName,
			metric.Type(),
			0,
		)
		err = addToGroupedMetric(metric, groupedMetrics, metricMetadata2, true, nil, testCfg, emfCalcs)
		assert.NoError(t, err)

		assert.Len(t, groupedMetrics, 2)
		seenLogGroup1 := false
		seenLogGroup2 := false
		for _, group := range groupedMetrics {
			assert.Len(t, group.metrics, 1)
			expectedMetrics := map[string]*metricInfo{
				"int-gauge": {
					value: float64(1),
					unit:  "Count",
				},
			}
			assert.Equal(t, expectedMetrics, group.metrics)
			expectedLabels := map[string]string{
				"label1": "value1",
			}
			assert.Equal(t, expectedLabels, group.labels)

			switch group.metadata.logGroup {
			case "log-group-2":
				seenLogGroup2 = true
			case "log-group-1":
				seenLogGroup1 = true
			}
		}
		assert.True(t, seenLogGroup1)
		assert.True(t, seenLogGroup2)
	})

	t.Run("Duplicate metric names", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
		groupedMetrics := make(map[any]*groupedMetric)
		generateMetrics := []pmetric.Metrics{
			generateTestGaugeMetric("foo", "int"),
			generateTestGaugeMetric("foo", "double"),
		}

		finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)

		rms := finalOtelMetrics.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 2, metrics.Len())

		obs, logs := observer.New(zap.WarnLevel)
		testCfg.logger = zap.New(obs)

		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(metrics.At(i),
				groupedMetrics,
				generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type(), 0),
				true,
				nil,
				testCfg,
				emfCalcs,
			)
			assert.NoError(t, err)
		}
		assert.Len(t, groupedMetrics, 1)

		labels := map[string]string{
			"label1": "value1",
		}
		// Test output warning logs
		expectedLogs := []observer.LoggedEntry{
			{
				Entry: zapcore.Entry{Level: zap.WarnLevel, Message: "Duplicate metric found"},
				Context: []zapcore.Field{
					zap.String("Name", "foo"),
					zap.Any("Labels", labels),
				},
			},
		}
		assert.Equal(t, 1, logs.Len())
		assert.Equal(t, expectedLogs, logs.AllUntimed())
	})

	t.Run("Unhandled metric type", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
		groupedMetrics := make(map[any]*groupedMetric)
		md := pmetric.NewMetrics()
		rms := md.ResourceMetrics()
		metric := rms.AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foo")
		metric.SetUnit("Count")

		obs, logs := observer.New(zap.WarnLevel)
		testCfg.logger = zap.New(obs)
		err := addToGroupedMetric(metric,
			groupedMetrics,
			generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeEmpty, 0),
			true,
			nil,
			testCfg,
			emfCalcs,
		)
		assert.NoError(t, err)
		assert.Empty(t, groupedMetrics)

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

	t.Run("Duplicate metric names with different metricIndex", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
		groupedMetrics := make(map[any]*groupedMetric)
		generateMetrics := []pmetric.Metrics{
			generateTestExponentialHistogramMetricWithLongBuckets("test_multiBucket_metric"),
		}
		finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)

		rms := finalOtelMetrics.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 1, metrics.Len())

		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(metrics.At(i),
				groupedMetrics,
				generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type(), 0),
				true,
				nil,
				testCfg,
				emfCalcs,
			)
			assert.NoError(t, err)
		}
		assert.Len(t, groupedMetrics, 2)
		expectedLabels := map[string]string{"label1": "value1"}

		// Sort metadata list to prevent race condition
		var metadataList []cWMetricMetadata
		for _, v := range groupedMetrics {
			metadataList = append(metadataList, v.metadata)
		}
		sort.Slice(metadataList, func(i, j int) bool {
			return metadataList[i].batchIndex < metadataList[j].batchIndex
		})

		for i, metadata := range metadataList {
			expectedMetadata := generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(0).Type(), i)
			assert.Equal(t, expectedMetadata, metadata)
		}
		for _, v := range groupedMetrics {
			assert.Len(t, v.metrics, 1)
			assert.Len(t, v.labels, 1)
			assert.Equal(t, expectedLabels, v.labels)
		}
	})

	t.Run("Detailed summary metrics", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))
		groupedMetrics := make(map[any]*groupedMetric)
		generateMetrics := []pmetric.Metrics{
			generateTestSummaryMetric("foo"),
		}
		finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)

		rms := finalOtelMetrics.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 2, metrics.Len(), "2 metrics are required to form 1 delta metric")

		cfg := createDefaultConfig().(*Config)
		cfg.DetailedMetrics = true

		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(metrics.At(i),
				groupedMetrics,
				generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type()),
				true,
				nil,
				cfg,
				emfCalcs,
			)
			assert.NoError(t, err)
		}
		assert.Len(t, groupedMetrics, 3) // sum + count, quantile 0, quantile 100 (see generateTestSummaryMetric)
		for _, group := range groupedMetrics {
			for metricName, metricInfo := range group.metrics {
				switch metricName {
				case "foo_sum", "foo_count":
					assert.Len(t, group.metrics, 2, "sum and count should be grouped together for detailed summary metrics")
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSummary), group.metadata)
				case "foo":
					quantileVal, ok := group.labels["quantile"]
					assert.True(t, ok)
					switch quantileVal {
					case "0":
						assert.Equal(t, float64(1), metricInfo.value)
					case "100":
						assert.Equal(t, float64(5), metricInfo.value)
					default:
						assert.Fail(t, "Unexpected quantile value")
					}
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSummary), group.metadata)
				default:
					assert.Fail(t, fmt.Sprintf("Unhandled metric %s not expected", metricName))
				}
				// ensure label1:value1 is always present (may not be the only label though)
				assert.Equal(t, "value1", group.labels["label1"])
			}
		}
	})

	t.Run("Gauge metric converted to histogram metric of values and counts ", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))

		// Create a gauge metric with multiple data points
		gpuMetricName := "container_gpu_utilization"
		gaugeMetric := generateTestGaugeMetricWithDataPoints(gpuMetricName, doubleValueType, []float64{10, 25, 15, 30, 20})

		// Create a config that includes the metric in MetricAsDistribution
		cfg := createDefaultConfig().(*Config)
		cfg.MetricAsDistribution = []string{gpuMetricName}

		// Set up the test
		groupedMetrics := make(map[any]*groupedMetric)
		rms := gaugeMetric.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()

		// Call addToGroupedMetric
		err := addToGroupedMetric(
			metrics.At(0),
			groupedMetrics,
			generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(0).Type()),
			true,
			nil,
			cfg,
			emfCalcs,
		)

		// Verify results
		assert.NoError(t, err)
		assert.Len(t, groupedMetrics, 1, "Should have one grouped metric")

		// Get the grouped metric
		var group *groupedMetric
		for _, g := range groupedMetrics {
			group = g
			break
		}

		// Verify the metric was in histogram
		assert.NotNil(t, group)
		assert.Contains(t, group.metrics, gpuMetricName)

		// Verify the metric value is in format of histogram values and counts
		metricInfo := group.metrics[gpuMetricName]
		histogram, ok := metricInfo.value.(*cWMetricHistogram)
		assert.True(t, ok, "Metric should be converted to a format of histogram values and counts")

		// Verify histogram properties
		assert.Equal(t, uint64(5), histogram.Count, "Histogram should have count of 5")
		assert.Equal(t, 100.0, histogram.Sum, "Histogram sum should match the sum of all values")
		assert.Equal(t, 10.0, histogram.Min, "Histogram min should match the minimum value")
		assert.Equal(t, 30.0, histogram.Max, "Histogram max should match the maximum value")

		// Check that we have the right number of unique values and counts
		assert.Len(t, histogram.Values, 5, "Histogram should have 5 unique values")
		assert.Len(t, histogram.Counts, 5, "Histogram should have 5 counts")

		// Verify the metadata was updated
		assert.Equal(t, pmetric.MetricTypeGauge, group.metadata.metricDataType)
	})

	t.Run("Gauge metric converted to histogram metric with duplicate metric name", func(t *testing.T) {
		emfCalcs := setupEmfCalculators()
		defer require.NoError(t, shutdownEmfCalculators(emfCalcs))

		// Create two gauge metrics with the same name but different data points
		gpuMetricName := "container_gpu_utilization"
		generateMetrics := []pmetric.Metrics{
			generateTestGaugeMetricWithDataPoints(gpuMetricName, doubleValueType, []float64{10, 25, 15, 30, 20}),
			generateTestGaugeMetricWithDataPoints(gpuMetricName, doubleValueType, []float64{35, 40, 45}),
		}

		finalOtelMetrics := generateOtelTestMetrics(generateMetrics...)

		// Create a config that includes the metric in MetricAsDistribution
		cfg := createDefaultConfig().(*Config)
		cfg.MetricAsDistribution = []string{gpuMetricName}

		// Set up logger to capture warning logs
		obs, logs := observer.New(zap.WarnLevel)
		cfg.logger = zap.New(obs)

		// Set up the test
		groupedMetrics := make(map[any]*groupedMetric)
		rms := finalOtelMetrics.ResourceMetrics()
		ilms := rms.At(0).ScopeMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 2, metrics.Len(), "Should have 2 gauge metrics with same name")

		// Process both metrics
		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(
				metrics.At(i),
				groupedMetrics,
				generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type()),
				true,
				nil,
				cfg,
				emfCalcs,
			)
			assert.NoError(t, err)
		}

		// Verify that we still have only one grouped metric (second one should be ignored)
		assert.Len(t, groupedMetrics, 1, "Should have one grouped metric")

		// Get the grouped metric
		var group *groupedMetric
		for _, g := range groupedMetrics {
			group = g
			break
		}

		// Verify the first metric was converted to histogram and the second was ignored
		assert.NotNil(t, group)
		assert.Contains(t, group.metrics, gpuMetricName)

		// Verify the labels in the grouped metric
		expectedLabels := map[string]string{"label1": "value1"}
		assert.Equal(t, expectedLabels, group.labels, "Grouped metric should have correct labels")

		// Verify the metric value is still from the first metric (values from second should be ignored)
		metricInfo := group.metrics[gpuMetricName]
		histogram, ok := metricInfo.value.(*cWMetricHistogram)
		assert.True(t, ok, "Metric should be converted to a format of values and counts")

		// Verify histogram properties match the first metric only
		assert.Equal(t, uint64(5), histogram.Count, "Histogram should have count of 5 from first metric")
		assert.Equal(t, 100.0, histogram.Sum, "Histogram sum should match the sum of first metric values")
		assert.Equal(t, 10.0, histogram.Min, "Histogram min should match the minimum value from first metric")
		assert.Equal(t, 30.0, histogram.Max, "Histogram max should match the maximum value from first metric")

		// Check that we have the right number of unique values and counts from first metric
		assert.Len(t, histogram.Values, 5, "Histogram should have 5 unique values from first metric")
		assert.Len(t, histogram.Counts, 5, "Histogram should have 5 counts from first metric")

		// Test that warning log was generated for duplicate metric
		expectedLogs := []observer.LoggedEntry{
			{
				Entry: zapcore.Entry{Level: zap.WarnLevel, Message: "Duplicate metric found"},
				Context: []zapcore.Field{
					zap.String("Name", gpuMetricName),
					zap.Any("Labels", map[string]string{"label1": "value1"}),
				},
			},
		}
		assert.Equal(t, 1, logs.Len(), "Should have one warning log")
		assert.Equal(t, expectedLogs, logs.AllUntimed(), "Should log duplicate metric warning")
	})
}

func TestAddKubernetesWrapper(t *testing.T) {
	t.Run("Test basic creation", func(t *testing.T) {
		dockerObj := struct {
			ContainerID string `json:"container_id"`
		}{
			ContainerID: "Container mccontainer the third",
		}
		expectedCreatedObj := struct {
			ContainerName string `json:"container_name"`
			Docker        any    `json:"docker"`
			Host          string `json:"host"`
			PodID         string `json:"pod_id"`
		}{
			ContainerName: "container mccontainer",
			Docker:        dockerObj,
			Host:          "hosty de la host",
			PodID:         "Le id de Pod",
		}

		inputs := make(map[string]string)
		inputs["container_id"] = "Container mccontainer the third"
		inputs["container"] = "container mccontainer"
		inputs["NodeName"] = "hosty de la host"
		inputs["PodId"] = "Le id de Pod"

		jsonBytes, _ := json.Marshal(expectedCreatedObj)
		addKubernetesWrapper(inputs)
		assert.JSONEq(t, string(jsonBytes), inputs["kubernetes"], "The created and expected objects should be the same")
	})
}

func BenchmarkAddToGroupedMetric(b *testing.B) {
	emfCalcs := setupEmfCalculators()
	defer require.NoError(b, shutdownEmfCalculators(emfCalcs))
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
	numMetrics := metrics.Len()

	for b.Loop() {
		groupedMetrics := make(map[any]*groupedMetric)
		for i := range numMetrics {
			metadata := generateTestMetricMetadata("namespace", int64(1596151098037), "log-group", "log-stream", "cloudwatch-otel", metrics.At(i).Type(), 0)
			err := addToGroupedMetric(metrics.At(i), groupedMetrics, metadata, true, nil, testCfg, emfCalcs)
			assert.NoError(b, err)
		}
	}
}

func TestTranslateUnit(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("writeIfNotExist")

	translator := &metricTranslator{
		metricDescriptor: map[string]MetricDescriptor{
			"writeIfNotExist": {
				MetricName: "writeIfNotExist",
				Unit:       "Count",
				Overwrite:  false,
			},
			"forceOverwrite": {
				MetricName: "forceOverwrite",
				Unit:       "Count",
				Overwrite:  true,
			},
		},
	}

	translateUnitCases := map[string]string{
		"Count": "Count",
		"ms":    "Milliseconds",
		"ns":    "",
		"1":     "",
		"s":     "Seconds",
		"us":    "Microseconds",
		"By":    "Bytes",
		"bit":   "Bits",
	}
	for input, output := range translateUnitCases {
		t.Run(input, func(_ *testing.T) {
			metric.SetUnit(input)

			v := translateUnit(metric, translator.metricDescriptor)
			assert.Equal(t, output, v)
		})
	}

	metric.SetName("forceOverwrite")
	v := translateUnit(metric, translator.metricDescriptor)
	assert.Equal(t, "Count", v)
}

func generateTestMetricMetadata(namespace string, timestamp int64, logGroup, logStreamName, instrumentationScopeName string, metricType pmetric.MetricType, batchIndex int) cWMetricMetadata {
	return cWMetricMetadata{
		receiver: prometheusReceiver,
		groupedMetricMetadata: groupedMetricMetadata{
			namespace:      namespace,
			timestampMs:    timestamp,
			logGroup:       logGroup,
			logStream:      logStreamName,
			metricDataType: metricType,
			batchIndex:     batchIndex,
		},
		instrumentationScopeName: instrumentationScopeName,
	}
}

func generateTestGaugeMetricWithDataPoints(name string, valueType metricValueType, values []float64) pmetric.Metrics {
	otelMetrics := pmetric.NewMetrics()
	rs := otelMetrics.ResourceMetrics().AppendEmpty()
	metrics := rs.ScopeMetrics().AppendEmpty().Metrics()
	metric := metrics.AppendEmpty()
	metric.SetName(name)
	metric.SetUnit("Count")
	gaugeMetric := metric.SetEmptyGauge()

	// Use current time as base timestamp
	baseTimestamp := time.Now().UnixNano()

	for i, val := range values {
		gaugeDatapoint := gaugeMetric.DataPoints().AppendEmpty()
		gaugeDatapoint.Attributes().PutStr("label1", "value1")
		// Set timestamp so labels get extracted properly in convertToDistribution
		gaugeDatapoint.SetTimestamp(pcommon.Timestamp(baseTimestamp + int64(i)))

		switch valueType {
		case doubleValueType:
			gaugeDatapoint.SetDoubleValue(val)
		default:
			gaugeDatapoint.SetIntValue(int64(val))
		}
	}

	return otelMetrics
}
