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
	"encoding/json"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/protobuf/types/known/timestamppb"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func TestAddToGroupedMetric(t *testing.T) {
	namespace := "namespace"
	instrumentationLibName := "cloudwatch-otel"
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	logger := zap.NewNop()

	metadata := cWMetricMetadata{
		receiver: prometheusReceiver,
		groupedMetricMetadata: groupedMetricMetadata{
			namespace:   namespace,
			timestampMs: timestamp,
			logGroup:    logGroup,
			logStream:   logStreamName,
		},
		instrumentationLibraryName: instrumentationLibName,
	}

	testCases := []struct {
		testName string
		metric   []*metricspb.Metric
		expected map[string]*metricInfo
	}{
		{
			"Int gauge",
			[]*metricspb.Metric{generateTestIntGauge("foo")},
			map[string]*metricInfo{
				"foo": {
					value: float64(1),
					unit:  "Count",
				},
			},
		},
		{
			"Double gauge",
			[]*metricspb.Metric{generateTestDoubleGauge("foo")},
			map[string]*metricInfo{
				"foo": {
					value: 0.1,
					unit:  "Count",
				},
			},
		},
		{
			"Int sum",
			generateTestIntSum("foo"),
			map[string]*metricInfo{
				"foo": {
					value: float64(1),
					unit:  "Count",
				},
			},
		},
		{
			"Double sum",
			generateTestDoubleSum("foo"),
			map[string]*metricInfo{
				"foo": {
					value: 0.1,
					unit:  "Count",
				},
			},
		},
		{
			"Double histogram",
			[]*metricspb.Metric{generateTestHistogram("foo")},
			map[string]*metricInfo{
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
			"Summary",
			generateTestSummary("foo"),
			map[string]*metricInfo{
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
		t.Run(tc.testName, func(t *testing.T) {
			setupDataPointCache()

			groupedMetrics := make(map[interface{}]*groupedMetric)
			oc := agentmetricspb.ExportMetricsServiceRequest{
				Node: &commonpb.Node{},
				Resource: &resourcepb.Resource{
					Labels: map[string]string{
						conventions.AttributeServiceName:      "myServiceName",
						conventions.AttributeServiceNamespace: "myServiceNS",
					},
				},
				Metrics: tc.metric,
			}
			// Retrieve *pdata.Metric
			rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
			rms := rm.ResourceMetrics()
			assert.Equal(t, 1, rms.Len())
			ilms := rms.At(0).InstrumentationLibraryMetrics()
			assert.Equal(t, 1, ilms.Len())
			metrics := ilms.At(0).Metrics()
			assert.Equal(t, len(tc.metric), metrics.Len())

			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				addToGroupedMetric(&metric, groupedMetrics, metadata, true, zap.NewNop(), nil, nil)
			}

			expectedLabels := map[string]string{
				oTellibDimensionKey: instrumentationLibName,
				"label1":            "value1",
			}

			assert.Equal(t, 1, len(groupedMetrics))
			for _, v := range groupedMetrics {
				assert.Equal(t, len(tc.expected), len(v.metrics))
				assert.Equal(t, tc.expected, v.metrics)
				assert.Equal(t, 2, len(v.labels))
				assert.Equal(t, metadata, v.metadata)
				assert.Equal(t, expectedLabels, v.labels)
			}
		})
	}

	t.Run("Add multiple different metrics", func(t *testing.T) {
		setupDataPointCache()

		groupedMetrics := make(map[interface{}]*groupedMetric)
		oc := agentmetricspb.ExportMetricsServiceRequest{
			Node: &commonpb.Node{},
			Resource: &resourcepb.Resource{
				Labels: map[string]string{
					conventions.AttributeServiceName:      "myServiceName",
					conventions.AttributeServiceNamespace: "myServiceNS",
				},
			},
			Metrics: []*metricspb.Metric{
				generateTestIntGauge("int-gauge"),
				generateTestDoubleGauge("double-gauge"),
				generateTestHistogram("double-histogram"),
			},
		}
		oc.Metrics = append(oc.Metrics, generateTestIntSum("int-sum")...)
		oc.Metrics = append(oc.Metrics, generateTestDoubleSum("double-sum")...)
		oc.Metrics = append(oc.Metrics, generateTestSummary("summary")...)
		rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
		rms := rm.ResourceMetrics()
		ilms := rms.At(0).InstrumentationLibraryMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 9, metrics.Len())

		for i := 0; i < metrics.Len(); i++ {
			metric := metrics.At(i)
			addToGroupedMetric(&metric, groupedMetrics, metadata, true, logger, nil, nil)
		}

		assert.Equal(t, 1, len(groupedMetrics))
		for _, group := range groupedMetrics {
			assert.Equal(t, 6, len(group.metrics))
			for metricName, metricInfo := range group.metrics {
				if metricName == "double-histogram" || metricName == "summary" {
					assert.Equal(t, "Seconds", metricInfo.unit)
				} else {
					assert.Equal(t, "Count", metricInfo.unit)
				}
			}
			expectedLabels := map[string]string{
				oTellibDimensionKey: "cloudwatch-otel",
				"label1":            "value1",
			}
			assert.Equal(t, expectedLabels, group.labels)
			assert.Equal(t, metadata, group.metadata)
		}
	})

	t.Run("Add multiple metrics w/ different timestamps", func(t *testing.T) {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		oc := agentmetricspb.ExportMetricsServiceRequest{
			Node: &commonpb.Node{},
			Resource: &resourcepb.Resource{
				Labels: map[string]string{
					conventions.AttributeServiceName:      "myServiceName",
					conventions.AttributeServiceNamespace: "myServiceNS",
				},
			},
			Metrics: []*metricspb.Metric{
				generateTestIntGauge("int-gauge"),
				generateTestDoubleGauge("double-gauge"),
				generateTestIntSum("int-sum")[1],
				generateTestSummary("summary")[1],
			},
		}

		timestamp1 := &timestamppb.Timestamp{
			Seconds: int64(1608068109),
			Nanos:   347942000,
		}
		timestamp2 := &timestamppb.Timestamp{
			Seconds: int64(1608068110),
			Nanos:   347942000,
		}

		// Give int gauge and int-sum the same timestamp
		oc.Metrics[0].Timeseries[0].Points[0].Timestamp = timestamp1
		oc.Metrics[2].Timeseries[0].Points[0].Timestamp = timestamp1
		// Give summary a different timestamp
		oc.Metrics[3].Timeseries[0].Points[0].Timestamp = timestamp2

		rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
		rms := rm.ResourceMetrics()
		ilms := rms.At(0).InstrumentationLibraryMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 4, metrics.Len())

		for i := 0; i < metrics.Len(); i++ {
			metric := metrics.At(i)
			addToGroupedMetric(&metric, groupedMetrics, metadata, true, logger, nil, nil)
		}

		assert.Equal(t, 3, len(groupedMetrics))
		for _, group := range groupedMetrics {
			for metricName := range group.metrics {
				if metricName == "int-gauge" || metricName == "int-sum" {
					assert.Equal(t, 2, len(group.metrics))
					assert.Equal(t, int64(1608068109347), group.metadata.timestampMs)
				} else if metricName == "summary" {
					assert.Equal(t, 1, len(group.metrics))
					assert.Equal(t, int64(1608068110347), group.metadata.timestampMs)
				} else {
					// double-gauge should use the default timestamp
					assert.Equal(t, 1, len(group.metrics))
					assert.Equal(t, "double-gauge", metricName)
					assert.Equal(t, timestamp, group.metadata.timestampMs)
				}
			}
			expectedLabels := map[string]string{
				oTellibDimensionKey: "cloudwatch-otel",
				"label1":            "value1",
			}
			assert.Equal(t, expectedLabels, group.labels)
		}
	})

	t.Run("Add same metric but different log group", func(t *testing.T) {
		groupedMetrics := make(map[interface{}]*groupedMetric)
		oc := agentmetricspb.ExportMetricsServiceRequest{
			Metrics: []*metricspb.Metric{
				generateTestIntGauge("int-gauge"),
			},
		}
		rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
		ilms := rm.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
		metric := ilms.At(0).Metrics().At(0)

		metricMetadata1 := cWMetricMetadata{
			groupedMetricMetadata: groupedMetricMetadata{
				namespace:   namespace,
				timestampMs: timestamp,
				logGroup:    "log-group-1",
				logStream:   logStreamName,
			},
			instrumentationLibraryName: instrumentationLibName,
		}
		addToGroupedMetric(&metric, groupedMetrics, metricMetadata1, true, logger, nil, nil)

		metricMetadata2 := cWMetricMetadata{
			groupedMetricMetadata: groupedMetricMetadata{
				namespace:   namespace,
				timestampMs: timestamp,
				logGroup:    "log-group-2",
				logStream:   logStreamName,
			},
			instrumentationLibraryName: instrumentationLibName,
		}
		addToGroupedMetric(&metric, groupedMetrics, metricMetadata2, true, logger, nil, nil)

		assert.Equal(t, 2, len(groupedMetrics))
		seenLogGroup1 := false
		seenLogGroup2 := false
		for _, group := range groupedMetrics {
			assert.Equal(t, 1, len(group.metrics))
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
		oc := agentmetricspb.ExportMetricsServiceRequest{
			Resource: &resourcepb.Resource{
				Labels: map[string]string{
					conventions.AttributeServiceName:      "myServiceName",
					conventions.AttributeServiceNamespace: "myServiceNS",
				},
			},
			Metrics: []*metricspb.Metric{
				generateTestIntGauge("foo"),
				generateTestDoubleGauge("foo"),
			},
		}
		rm := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics)
		rms := rm.ResourceMetrics()
		ilms := rms.At(0).InstrumentationLibraryMetrics()
		metrics := ilms.At(0).Metrics()
		assert.Equal(t, 2, metrics.Len())

		obs, logs := observer.New(zap.WarnLevel)
		obsLogger := zap.New(obs)

		for i := 0; i < metrics.Len(); i++ {
			metric := metrics.At(i)
			addToGroupedMetric(&metric, groupedMetrics, metadata, true, obsLogger, nil, nil)
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
		md := pdata.NewMetrics()
		rms := md.ResourceMetrics()
		metric := rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foo")
		metric.SetUnit("Count")
		metric.SetDataType(pdata.MetricDataTypeNone)

		obs, logs := observer.New(zap.WarnLevel)
		obsLogger := zap.New(obs)
		addToGroupedMetric(&metric, groupedMetrics, metadata, true, obsLogger, nil, nil)
		assert.Equal(t, 0, len(groupedMetrics))

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
		groupedMetrics := make(map[interface{}]*groupedMetric)
		addToGroupedMetric(nil, groupedMetrics, metadata, true, logger, nil, nil)
		assert.Equal(t, 0, len(groupedMetrics))
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
	oc := agentmetricspb.ExportMetricsServiceRequest{
		Metrics: []*metricspb.Metric{
			generateTestIntGauge("int-gauge"),
			generateTestDoubleGauge("double-gauge"),
			generateTestHistogram("double-histogram"),
		},
	}
	oc.Metrics = append(oc.Metrics, generateTestIntSum("int-sum")...)
	oc.Metrics = append(oc.Metrics, generateTestDoubleSum("double-sum")...)
	oc.Metrics = append(oc.Metrics, generateTestSummary("summary")...)
	rms := internaldata.OCToMetrics(oc.Node, oc.Resource, oc.Metrics).ResourceMetrics()
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
		groupedMetrics := make(map[interface{}]*groupedMetric)
		for i := 0; i < numMetrics; i++ {
			metric := metrics.At(i)
			addToGroupedMetric(&metric, groupedMetrics, metadata, true, logger, nil, nil)
		}
	}
}

func TestTranslateUnit(t *testing.T) {
	metric := pdata.NewMetric()
	metric.SetName("writeIfNotExist")

	translator := &metricTranslator{
		metricDescriptor: map[string]MetricDescriptor{
			"writeIfNotExist": {
				metricName: "writeIfNotExist",
				unit:       "Count",
				overwrite:  false,
			},
			"forceOverwrite": {
				metricName: "forceOverwrite",
				unit:       "Count",
				overwrite:  true,
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

			v := translateUnit(&metric, translator.metricDescriptor)
			assert.Equal(t, output, v)
		})
	}

	metric.SetName("forceOverwrite")
	v := translateUnit(&metric, translator.metricDescriptor)
	assert.Equal(t, "Count", v)
}
