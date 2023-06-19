// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutil"
)

func TestNewExporter(t *testing.T) {
	if !isMetricExportV2Enabled() {
		require.NoError(t, enableNativeMetricExport())
		defer require.NoError(t, enableZorkianMetricExport())
	}
	server := testutil.DatadogServerMock()
	defer server.Close()

	cfg := &Config{
		API: APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:             HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
		},
	}
	params := exportertest.NewNopCreateSettings()
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	assert.Equal(t, len(server.MetadataChan), 0)

	cfg.HostMetadata.Enabled = true
	cfg.HostMetadata.HostnameSource = HostnameSourceFirstResource
	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	body := <-server.MetadataChan
	var recvMetadata hostmetadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}

func Test_metricsExporter_PushMetricsData(t *testing.T) {
	if !isMetricExportV2Enabled() {
		require.NoError(t, enableNativeMetricExport())
		t.Cleanup(func() { require.NoError(t, enableZorkianMetricExport()) })
	}
	attrs := map[string]string{
		conventions.AttributeDeploymentEnvironment: "dev",
		"custom_attribute":                         "custom_value",
	}
	tests := []struct {
		metrics               pmetric.Metrics
		source                source.Source
		hostTags              []string
		histogramMode         HistogramMode
		expectedSeries        map[string]interface{}
		expectedSketchPayload *gogen.SketchPayload
		expectedErr           error
		expectedStats         []pb.ClientStatsPayload
	}{
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeNoBuckets,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedErr:   errors.New("no buckets mode and no send count sum are incompatible"),
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric":    "int.gauge",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric":    "double.histogram.bucket",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"lower_bound:-inf", "upper_bound:0", "env:dev"},
					},
					map[string]interface{}{
						"metric":    "double.histogram.bucket",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"lower_bound:0", "upper_bound:inf", "env:dev"},
					},
					map[string]interface{}{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"version:latest", "command:otelcol"},
					},
					map[string]interface{}{
						"metric":    "system.disk.in_use",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev"},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeDistributions,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric":    "int.gauge",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"version:latest", "command:otelcol"},
					},
					map[string]interface{}{
						"metric":    "system.disk.in_use",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev"},
					},
				},
			},
			expectedSketchPayload: &gogen.SketchPayload{
				Sketches: []gogen.SketchPayload_Sketch{
					{
						Metric: "double.histogram",
						Host:   "test-host",
						Tags:   []string{"env:dev"},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Cnt: 20,
								Avg: 0.3,
								Sum: 6,
								K:   []int32{0},
								N:   []uint32{20},
							},
						},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.AWSECSFargateKind,
				Identifier: "task_arn",
			},
			histogramMode: HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric":    "int.gauge",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric":    "double.histogram.bucket",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"lower_bound:-inf", "upper_bound:0", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric":    "double.histogram.bucket",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"lower_bound:0", "upper_bound:inf", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric":    "system.disk.in_use",
						"points":    []interface{}{map[string]interface{}{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []interface{}{map[string]interface{}{"name": "test-host", "type": "host"}},
						"tags":      []interface{}{"env:dev", "key1:value1", "key2:value2"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("kind=%s,histgramMode=%s", tt.source.Kind, tt.histogramMode), func(t *testing.T) {
			seriesRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.MetricV2Endpoint}
			sketchRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.SketchesMetricEndpoint}
			server := testutil.DatadogServerMock(
				seriesRecorder.HandlerFunc,
				sketchRecorder.HandlerFunc,
			)
			defer server.Close()

			var (
				once          sync.Once
				statsRecorder testutil.MockStatsProcessor
			)
			exp, err := newMetricsExporter(
				context.Background(),
				exportertest.NewNopCreateSettings(),
				newTestConfig(t, server.URL, tt.hostTags, tt.histogramMode),
				&once,
				&testutil.MockSourceProvider{Src: tt.source},
				&statsRecorder,
			)
			if tt.expectedErr == nil {
				assert.NoError(t, err, "unexpected error")
			} else {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			exp.getPushTime = func() uint64 { return 0 }
			err = exp.PushMetricsData(context.Background(), tt.metrics)
			if tt.expectedErr == nil {
				assert.NoError(t, err, "unexpected error")
			} else {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			if len(tt.expectedSeries) == 0 {
				assert.Nil(t, seriesRecorder.ByteBody)
			} else {
				assert.Equal(t, "gzip", seriesRecorder.Header.Get("Accept-Encoding"))
				assert.Equal(t, "application/json", seriesRecorder.Header.Get("Content-Type"))
				assert.Equal(t, "gzip", seriesRecorder.Header.Get("Content-Encoding"))
				assert.Equal(t, "otelcol/latest", seriesRecorder.Header.Get("User-Agent"))
				assert.NoError(t, err)
				buf := bytes.NewBuffer(seriesRecorder.ByteBody)
				reader, err := gzip.NewReader(buf)
				assert.NoError(t, err)
				dec := json.NewDecoder(reader)
				var actual map[string]interface{}
				assert.NoError(t, dec.Decode(&actual))
				assert.EqualValues(t, tt.expectedSeries, actual)
			}
			if tt.expectedSketchPayload == nil {
				assert.Nil(t, sketchRecorder.ByteBody)
			} else {
				assert.Equal(t, "gzip", sketchRecorder.Header.Get("Accept-Encoding"))
				assert.Equal(t, "application/x-protobuf", sketchRecorder.Header.Get("Content-Type"))
				assert.Equal(t, "otelcol/latest", sketchRecorder.Header.Get("User-Agent"))
				expected, err := tt.expectedSketchPayload.Marshal()
				assert.NoError(t, err)
				assert.Equal(t, expected, sketchRecorder.ByteBody)
			}
			if tt.expectedStats == nil {
				assert.Len(t, statsRecorder.In, 0)
			} else {
				assert.ElementsMatch(t, statsRecorder.In, tt.expectedStats)
			}
		})
	}
}

func TestNewExporter_Zorkian(t *testing.T) {
	if isMetricExportV2Enabled() {
		require.NoError(t, enableZorkianMetricExport())
		defer require.NoError(t, enableNativeMetricExport())
	}
	server := testutil.DatadogServerMock()
	defer server.Close()

	cfg := &Config{
		API: APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:             HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
		},
	}
	params := exportertest.NewNopCreateSettings()
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	assert.Equal(t, len(server.MetadataChan), 0)

	cfg.HostMetadata.Enabled = true
	cfg.HostMetadata.HostnameSource = HostnameSourceFirstResource
	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	body := <-server.MetadataChan
	var recvMetadata hostmetadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}

func Test_metricsExporter_PushMetricsData_Zorkian(t *testing.T) {
	if isMetricExportV2Enabled() {
		require.NoError(t, enableZorkianMetricExport())
		t.Cleanup(func() { require.NoError(t, enableNativeMetricExport()) })
	}
	attrs := map[string]string{
		conventions.AttributeDeploymentEnvironment: "dev",
		"custom_attribute":                         "custom_value",
	}
	tests := []struct {
		metrics               pmetric.Metrics
		source                source.Source
		hostTags              []string
		histogramMode         HistogramMode
		expectedSeries        map[string]interface{}
		expectedSketchPayload *gogen.SketchPayload
		expectedErr           error
		expectedStats         []pb.ClientStatsPayload
	}{
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeNoBuckets,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedErr:   errors.New("no buckets mode and no send count sum are incompatible"),
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric": "int.gauge",
						"points": []interface{}{[]interface{}{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric": "otel.system.filesystem.utilization",
						"points": []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric": "double.histogram.bucket",
						"points": []interface{}{[]interface{}{float64(0), float64(2)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []interface{}{"lower_bound:-inf", "upper_bound:0", "env:dev"},
					},
					map[string]interface{}{
						"metric": "double.histogram.bucket",
						"points": []interface{}{[]interface{}{float64(0), float64(18)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []interface{}{"lower_bound:0", "upper_bound:inf", "env:dev"},
					},
					map[string]interface{}{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []interface{}{[]interface{}{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"version:latest", "command:otelcol"},
					},
					map[string]interface{}{
						"metric":   "system.disk.in_use",
						"points":   []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":     "gauge",
						"host":     "test-host",
						"interval": float64(1),
						"tags":     []interface{}{"env:dev"},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeDistributions,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric": "int.gauge",
						"points": []interface{}{[]interface{}{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric": "otel.system.filesystem.utilization",
						"points": []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []interface{}{[]interface{}{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"version:latest", "command:otelcol"},
					},
					map[string]interface{}{
						"metric":   "system.disk.in_use",
						"points":   []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":     "gauge",
						"host":     "test-host",
						"interval": float64(1),
						"tags":     []interface{}{"env:dev"},
					},
				},
			},
			expectedSketchPayload: &gogen.SketchPayload{
				Sketches: []gogen.SketchPayload_Sketch{
					{
						Metric: "double.histogram",
						Host:   "test-host",
						Tags:   []string{"env:dev"},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Cnt: 20,
								Avg: 0.3,
								Sum: 6,
								K:   []int32{0},
								N:   []uint32{20},
							},
						},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.AWSECSFargateKind,
				Identifier: "task_arn",
			},
			histogramMode: HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric": "int.gauge",
						"points": []interface{}{[]interface{}{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric": "otel.system.filesystem.utilization",
						"points": []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric": "double.histogram.bucket",
						"points": []interface{}{[]interface{}{float64(0), float64(2)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []interface{}{"lower_bound:-inf", "upper_bound:0", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric": "double.histogram.bucket",
						"points": []interface{}{[]interface{}{float64(0), float64(18)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []interface{}{"lower_bound:0", "upper_bound:inf", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []interface{}{[]interface{}{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
					map[string]interface{}{
						"metric":   "system.disk.in_use",
						"points":   []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":     "gauge",
						"host":     "test-host",
						"interval": float64(1),
						"tags":     []interface{}{"env:dev", "key1:value1", "key2:value2"},
					},
				},
			},
			expectedSketchPayload: nil,
			expectedErr:           nil,
		},
		{
			metrics: createTestMetricsWithStats(),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: HistogramModeDistributions,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]interface{}{
				"series": []interface{}{
					map[string]interface{}{
						"metric": "int.gauge",
						"points": []interface{}{[]interface{}{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric": "otel.system.filesystem.utilization",
						"points": []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"env:dev"},
					},
					map[string]interface{}{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []interface{}{[]interface{}{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []interface{}{"version:latest", "command:otelcol"},
					},
					map[string]interface{}{
						"metric":   "system.disk.in_use",
						"points":   []interface{}{[]interface{}{float64(0), float64(333)}},
						"type":     "gauge",
						"host":     "test-host",
						"interval": float64(1),
						"tags":     []interface{}{"env:dev"},
					},
				},
			},
			expectedSketchPayload: &gogen.SketchPayload{
				Sketches: []gogen.SketchPayload_Sketch{
					{
						Metric: "double.histogram",
						Host:   "test-host",
						Tags:   []string{"env:dev"},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Cnt: 20,
								Avg: 0.3,
								Sum: 6,
								K:   []int32{0},
								N:   []uint32{20},
							},
						},
					},
				},
			},
			expectedStats: testutil.StatsPayloads,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("kind=%s,histgramMode=%s", tt.source.Kind, tt.histogramMode), func(t *testing.T) {
			seriesRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.MetricV1Endpoint}
			sketchRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.SketchesMetricEndpoint}
			server := testutil.DatadogServerMock(
				seriesRecorder.HandlerFunc,
				sketchRecorder.HandlerFunc,
			)
			defer server.Close()

			var (
				once          sync.Once
				statsRecorder testutil.MockStatsProcessor
			)
			exp, err := newMetricsExporter(
				context.Background(),
				exportertest.NewNopCreateSettings(),
				newTestConfig(t, server.URL, tt.hostTags, tt.histogramMode),
				&once,
				&testutil.MockSourceProvider{Src: tt.source},
				&statsRecorder,
			)
			if tt.expectedErr == nil {
				assert.NoError(t, err, "unexpected error")
			} else {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			exp.getPushTime = func() uint64 { return 0 }
			err = exp.PushMetricsData(context.Background(), tt.metrics)
			if tt.expectedErr == nil {
				assert.NoError(t, err, "unexpected error")
			} else {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			if len(tt.expectedSeries) == 0 {
				assert.Nil(t, seriesRecorder.ByteBody)
			} else {
				assert.Equal(t, "gzip", seriesRecorder.Header.Get("Accept-Encoding"))
				assert.Equal(t, "application/json", seriesRecorder.Header.Get("Content-Type"))
				assert.Equal(t, "otelcol/latest", seriesRecorder.Header.Get("User-Agent"))
				assert.NoError(t, err)
				var actual map[string]interface{}
				assert.NoError(t, json.Unmarshal(seriesRecorder.ByteBody, &actual))
				assert.EqualValues(t, tt.expectedSeries, actual)
			}
			if tt.expectedSketchPayload == nil {
				assert.Nil(t, sketchRecorder.ByteBody)
			} else {
				assert.Equal(t, "gzip", sketchRecorder.Header.Get("Accept-Encoding"))
				assert.Equal(t, "application/x-protobuf", sketchRecorder.Header.Get("Content-Type"))
				assert.Equal(t, "otelcol/latest", sketchRecorder.Header.Get("User-Agent"))
				expected, err := tt.expectedSketchPayload.Marshal()
				assert.NoError(t, err)
				assert.Equal(t, expected, sketchRecorder.ByteBody)
			}
			if tt.expectedStats == nil {
				assert.Len(t, statsRecorder.In, 0)
			} else {
				assert.ElementsMatch(t, statsRecorder.In, tt.expectedStats)
			}
		})
	}
}

func createTestMetricsWithStats() pmetric.Metrics {
	md := createTestMetrics(map[string]string{
		conventions.AttributeDeploymentEnvironment: "dev",
		"custom_attribute":                         "custom_value",
	})
	dest := md.ResourceMetrics()
	logger, _ := zap.NewDevelopment()
	trans, err := metrics.NewTranslator(logger)
	if err != nil {
		panic(err)
	}
	src := trans.
		StatsPayloadToMetrics(pb.StatsPayload{Stats: testutil.StatsPayloads}).
		ResourceMetrics()
	src.MoveAndAppendTo(dest)
	return md
}

func createTestMetrics(additionalAttributes map[string]string) pmetric.Metrics {
	const (
		host    = "test-host"
		name    = "test-metrics"
		version = "v0.0.1"
	)
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	attrs := rm.Resource().Attributes()
	attrs.PutStr("datadog.host.name", host)
	for attr, val := range additionalAttributes {
		attrs.PutStr(attr, val)
	}
	ilms := rm.ScopeMetrics()

	ilm := ilms.AppendEmpty()
	ilm.Scope().SetName(name)
	ilm.Scope().SetVersion(version)
	metricsArray := ilm.Metrics()
	metricsArray.AppendEmpty() // first one is TypeNone to test that it's ignored

	// IntGauge
	met := metricsArray.AppendEmpty()
	met.SetName("int.gauge")
	dpsInt := met.SetEmptyGauge().DataPoints()
	dpInt := dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntValue(222)

	// host metric
	met = metricsArray.AppendEmpty()
	met.SetName("system.filesystem.utilization")
	dpsInt = met.SetEmptyGauge().DataPoints()
	dpInt = dpsInt.AppendEmpty()
	dpInt.SetTimestamp(seconds(0))
	dpInt.SetIntValue(333)

	// Histogram (delta)
	met = metricsArray.AppendEmpty()
	met.SetName("double.histogram")
	met.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dpsDoubleHist := met.Histogram().DataPoints()
	dpDoubleHist := dpsDoubleHist.AppendEmpty()
	dpDoubleHist.SetCount(20)
	dpDoubleHist.SetSum(6)
	dpDoubleHist.BucketCounts().FromRaw([]uint64{2, 18})
	dpDoubleHist.ExplicitBounds().FromRaw([]float64{0})
	dpDoubleHist.SetTimestamp(seconds(0))

	return md
}

func seconds(i int) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(time.Unix(int64(i), 0))
}

func newTestConfig(t *testing.T, endpoint string, hostTags []string, histogramMode HistogramMode) *Config {
	t.Helper()
	return &Config{
		HostMetadata: HostMetadataConfig{
			Tags: hostTags,
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: endpoint,
			},
			HistConfig: HistogramConfig{
				Mode: histogramMode,
			},
			// Set values to avoid errors. No particular intention in value selection.
			DeltaTTL: 3600,
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeRawValue,
			},
		},
	}
}
