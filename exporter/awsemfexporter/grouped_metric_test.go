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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	logger := zap.NewNop()

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
			expectedLabels:     map[string]string{oTellibDimensionKey: instrumentationLibName, "label1": "value1"},
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
			expectedLabels:     map[string]string{oTellibDimensionKey: instrumentationLibName, "label1": "value1"},
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
			expectedLabels:     map[string]string{oTellibDimensionKey: instrumentationLibName, "label1": "value1"},
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
			expectedLabels:     map[string]string{oTellibDimensionKey: instrumentationLibName, "label1": "value1"},
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
			setupDataPointCache()

			groupedMetrics := make(map[interface{}]*groupedMetric)
			rms := tc.metric.ResourceMetrics()
			ilms := rms.At(0).ScopeMetrics()
			metrics := ilms.At(0).Metrics()

			assert.Equal(t, 1, rms.Len())
			assert.Equal(t, 1, ilms.Len())

			for i := 0; i < metrics.Len(); i++ {
				err := addToGroupedMetric(metrics.At(i), groupedMetrics, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type()), true, zap.NewNop(), nil, testCfg)
				assert.Nil(t, err)
			}

			assert.Equal(t, 1, len(groupedMetrics))
			for _, v := range groupedMetrics {
				assert.Equal(t, len(tc.expectedMetricInfo), len(v.metrics))
				assert.Equal(t, tc.expectedMetricInfo, v.metrics)
				assert.Equal(t, 2, len(v.labels))
				assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, tc.expectedMetricType), v.metadata)
				assert.Equal(t, tc.expectedLabels, v.labels)
			}
		})
	}

	t.Run("Add multiple different metrics", func(t *testing.T) {
		setupDataPointCache()

		groupedMetrics := make(map[interface{}]*groupedMetric)
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
			err := addToGroupedMetric(metrics.At(i), groupedMetrics, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type()), true, logger, nil, testCfg)
			assert.Nil(t, err)
		}

		assert.Equal(t, 4, len(groupedMetrics))
		for _, group := range groupedMetrics {
			for metricName, metricInfo := range group.metrics {
				switch metricName {
				case "int-gauge", "double-gauge":
					assert.Len(t, group.metrics, 2)
					assert.Equal(t, "Count", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeGauge), group.metadata)
				case "int-sum", "double-sum":
					assert.Len(t, group.metrics, 2)
					assert.Equal(t, "Count", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSum), group.metadata)
				case "histogram":
					assert.Len(t, group.metrics, 1)
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeHistogram), group.metadata)
				case "summary":
					assert.Len(t, group.metrics, 1)
					assert.Equal(t, "Seconds", metricInfo.unit)
					assert.Equal(t, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeSummary), group.metadata)
				default:
					assert.Fail(t, fmt.Sprintf("Unhandled metric %s not expected", metricName))
				}
				expectedLabels := map[string]string{
					oTellibDimensionKey: "cloudwatch-otel",
					"label1":            "value1",
				}
				assert.Equal(t, expectedLabels, group.labels)
			}
		}
	})

	t.Run("Add same metric but different log group", func(t *testing.T) {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		otelMetrics := generateTestGaugeMetric("int-gauge", "int")
		ilms := otelMetrics.ResourceMetrics().At(0).ScopeMetrics()
		metric := ilms.At(0).Metrics().At(0)

		metricMetadata1 := generateTestMetricMetadata(namespace, timestamp, "log-group-1", logStreamName, instrumentationLibName, metric.Type())
		err := addToGroupedMetric(metric, groupedMetrics, metricMetadata1, true, logger, nil, testCfg)
		assert.Nil(t, err)

		metricMetadata2 := generateTestMetricMetadata(namespace, timestamp, "log-group-2", logStreamName, instrumentationLibName, metric.Type())
		err = addToGroupedMetric(metric, groupedMetrics, metricMetadata2, true, logger, nil, testCfg)
		assert.Nil(t, err)

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
				oTellibDimensionKey: "cloudwatch-otel",
				"label1":            "value1",
			}
			assert.Equal(t, expectedLabels, group.labels)

			if group.metadata.logGroup == "log-group-2" {
				seenLogGroup2 = true
			} else if group.metadata.logGroup == "log-group-1" {
				seenLogGroup1 = true
			}
		}
		assert.True(t, seenLogGroup1)
		assert.True(t, seenLogGroup2)
	})

	t.Run("Duplicate metric names", func(t *testing.T) {
		groupedMetrics := make(map[interface{}]*groupedMetric)
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
		obsLogger := zap.New(obs)

		for i := 0; i < metrics.Len(); i++ {
			err := addToGroupedMetric(metrics.At(i), groupedMetrics, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, metrics.At(i).Type()), true, obsLogger, nil, testCfg)
			assert.Nil(t, err)
		}
		assert.Equal(t, 1, len(groupedMetrics))

		labels := map[string]string{
			oTellibDimensionKey: instrumentationLibName,
			"label1":            "value1",
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
		groupedMetrics := make(map[interface{}]*groupedMetric)
		md := pmetric.NewMetrics()
		rms := md.ResourceMetrics()
		metric := rms.AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foo")
		metric.SetUnit("Count")

		obs, logs := observer.New(zap.WarnLevel)
		obsLogger := zap.New(obs)
		err := addToGroupedMetric(metric, groupedMetrics, generateTestMetricMetadata(namespace, timestamp, logGroup, logStreamName, instrumentationLibName, pmetric.MetricTypeEmpty), true, obsLogger, nil, testCfg)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(groupedMetrics))

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

func TestAddKubernetesWrapper(t *testing.T) {
	t.Run("Test basic creation", func(t *testing.T) {
		dockerObj := struct {
			ContainerID string `json:"container_id"`
		}{
			ContainerID: "Container mccontainter the third",
		}
		expectedCreatedObj := struct {
			ContainerName string      `json:"container_name"`
			Docker        interface{} `json:"docker"`
			Host          string      `json:"host"`
			PodID         string      `json:"pod_id"`
		}{
			ContainerName: "container mccontainer",
			Docker:        dockerObj,
			Host:          "hosty de la host",
			PodID:         "Le id de Pod",
		}

		inputs := make(map[string]string)
		inputs["container_id"] = "Container mccontainter the third"
		inputs["container"] = "container mccontainer"
		inputs["NodeName"] = "hosty de la host"
		inputs["PodId"] = "Le id de Pod"

		jsonBytes, _ := json.Marshal(expectedCreatedObj)
		addKubernetesWrapper(inputs)
		assert.Equal(t, string(jsonBytes), inputs["kubernetes"], "The created and expected objects should be the same")
	})
}

func BenchmarkAddToGroupedMetric(b *testing.B) {
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

	logger := zap.NewNop()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		for i := 0; i < numMetrics; i++ {
			metadata := generateTestMetricMetadata("namespace", int64(1596151098037), "log-group", "log-stream", "cloudwatch-otel", metrics.At(i).Type())
			err := addToGroupedMetric(metrics.At(i), groupedMetrics, metadata, true, logger, nil, testCfg)
			assert.Nil(b, err)
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
		"s":     "Seconds",
		"us":    "Microseconds",
		"By":    "Bytes",
		"Bi":    "Bits",
	}
	for input, output := range translateUnitCases {
		t.Run(input, func(tt *testing.T) {
			metric.SetUnit(input)

			v := translateUnit(metric, translator.metricDescriptor)
			assert.Equal(t, output, v)
		})
	}

	metric.SetName("forceOverwrite")
	v := translateUnit(metric, translator.metricDescriptor)
	assert.Equal(t, "Count", v)
}

func generateTestMetricMetadata(namespace string, timestamp int64, logGroup, logStreamName, instrumentationScopeName string, metricType pmetric.MetricType) cWMetricMetadata {
	return cWMetricMetadata{
		receiver: prometheusReceiver,
		groupedMetricMetadata: groupedMetricMetadata{
			namespace:      namespace,
			timestampMs:    timestamp,
			logGroup:       logGroup,
			logStream:      logStreamName,
			metricDataType: metricType,
		},
		instrumentationScopeName: instrumentationScopeName,
	}
}
