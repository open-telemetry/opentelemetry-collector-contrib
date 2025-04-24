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
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions127 "go.opentelemetry.io/collector/semconv/v1.27.0"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestNewExporter(t *testing.T) {
	if !isMetricExportV2Enabled() {
		require.NoError(t, enableNativeMetricExport())
		defer require.NoError(t, enableMetricExportSerializer())
	}
	server := testutil.DatadogServerMock()
	defer server.Close()

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: datadogconfig.HistogramConfig{
				Mode:             datadogconfig.HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeToDelta,
			},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled:        true,
			ReporterPeriod: 30 * time.Minute,
		},
	}
	cfg.HostMetadata.SetSourceTimeout(50 * time.Millisecond)

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetrics(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	assert.Empty(t, server.MetadataChan)

	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func TestNewExporter_Serializer(t *testing.T) {
	t.Skip("Flaky, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39601")
	server := testutil.DatadogServerMock()
	defer server.Close()

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: datadogconfig.HistogramConfig{
				Mode:             datadogconfig.HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeToDelta,
			},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled:        true,
			ReporterPeriod: 30 * time.Minute,
		},
	}
	cfg.HostMetadata.SetSourceTimeout(50 * time.Millisecond)

	params := exportertest.NewNopSettings(metadata.Type)
	var err error
	params.Logger, err = zap.NewDevelopment()
	require.NoError(t, err)
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetrics(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	assert.Empty(t, server.MetadataChan)

	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func Test_metricsExporter_PushMetricsData(t *testing.T) {
	if !isMetricExportV2Enabled() {
		require.NoError(t, enableNativeMetricExport())
		t.Cleanup(func() { require.NoError(t, enableMetricExportSerializer()) })
	}
	attrs := map[string]string{
		conventions.AttributeDeploymentEnvironment: "dev",
		"custom_attribute":                         "custom_value",
	}
	tests := []struct {
		metrics               pmetric.Metrics
		source                source.Source
		hostTags              []string
		histogramMode         datadogconfig.HistogramMode
		expectedSeries        map[string]any
		expectedSketchPayload *gogen.SketchPayload
		expectedErr           error
	}{
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeNoBuckets,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedErr:   errors.New("no buckets mode and no send count sum are incompatible"),
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric":    "int.gauge",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:-inf", "upper_bound:0", "env:dev"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:0", "upper_bound:inf", "env:dev"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(map[string]string{
				conventions127.AttributeDeploymentEnvironmentName: "new_env",
				"custom_attribute": "custom_value",
			}),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric":    "int.gauge",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:new_env"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:new_env"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:-inf", "upper_bound:0", "env:new_env"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:0", "upper_bound:inf", "env:new_env"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:new_env"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
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
			histogramMode: datadogconfig.HistogramModeDistributions,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric":    "int.gauge",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
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
			histogramMode: datadogconfig.HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric":    "int.gauge",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(222)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:-inf", "upper_bound:0", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:0", "upper_bound:inf", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
				},
			},
		},
	}
	gatewayUsage := attributes.NewGatewayUsage()
	for _, tt := range tests {
		t.Run(fmt.Sprintf("kind=%s,histogramMode=%s", tt.source.Kind, tt.histogramMode), func(t *testing.T) {
			seriesRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.MetricV2Endpoint}
			sketchRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.SketchesMetricEndpoint}
			server := testutil.DatadogServerMock(
				seriesRecorder.HandlerFunc,
				sketchRecorder.HandlerFunc,
			)
			defer server.Close()

			var once sync.Once
			pusher := newTestPusher(t)
			reporter, err := inframetadata.NewReporter(zap.NewNop(), pusher, 1*time.Second)
			require.NoError(t, err)
			attributesTranslator, err := attributes.NewTranslator(componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)
			acfg := traceconfig.New()
			exp, err := newMetricsExporter(
				context.Background(),
				exportertest.NewNopSettings(metadata.Type),
				newTestConfig(t, server.URL, tt.hostTags, tt.histogramMode),
				acfg,
				&once,
				attributesTranslator,
				&testutil.MockSourceProvider{Src: tt.source},
				reporter,
				nil,
				gatewayUsage,
			)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			assert.NoError(t, err, "unexpected error")
			exp.getPushTime = func() uint64 { return 0 }
			err = exp.PushMetricsData(context.Background(), tt.metrics)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			assert.NoError(t, err, "unexpected error")
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
				var actual map[string]any
				assert.NoError(t, dec.Decode(&actual))
				assert.Equal(t, tt.expectedSeries, actual)
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
		})
	}
}

func TestNewExporter_Zorkian(t *testing.T) {
	if isMetricExportV2Enabled() {
		require.NoError(t, enableZorkianMetricExport())
		defer require.NoError(t, enableMetricExportSerializer())
	}
	server := testutil.DatadogServerMock()
	defer server.Close()

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: datadogconfig.HistogramConfig{
				Mode:             datadogconfig.HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeToDelta,
			},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled:        true,
			ReporterPeriod: 30 * time.Minute,
		},
	}
	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetrics(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	assert.Empty(t, server.MetadataChan)

	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(context.Background(), testMetrics)
	require.NoError(t, err)
	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func Test_metricsExporter_PushMetricsData_Zorkian(t *testing.T) {
	if isMetricExportV2Enabled() {
		require.NoError(t, enableZorkianMetricExport())
		t.Cleanup(func() { require.NoError(t, enableMetricExportSerializer()) })
	}
	attrs := map[string]string{
		conventions.AttributeDeploymentEnvironment: "dev",
		"custom_attribute":                         "custom_value",
	}
	tests := []struct {
		metrics               pmetric.Metrics
		source                source.Source
		hostTags              []string
		histogramMode         datadogconfig.HistogramMode
		expectedSeries        map[string]any
		expectedSketchPayload *gogen.SketchPayload
		expectedErr           error
	}{
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeNoBuckets,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedErr:   errors.New("no buckets mode and no send count sum are incompatible"),
		},
		{
			metrics: createTestMetrics(attrs),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric": "int.gauge",
						"points": []any{[]any{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "otel.system.filesystem.utilization",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "double.histogram.bucket",
						"points": []any{[]any{float64(0), float64(2)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []any{"lower_bound:-inf", "upper_bound:0", "env:dev"},
					},
					map[string]any{
						"metric": "double.histogram.bucket",
						"points": []any{[]any{float64(0), float64(18)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []any{"lower_bound:0", "upper_bound:inf", "env:dev"},
					},
					map[string]any{
						"metric": "system.disk.in_use",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []any{[]any{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"version:latest", "command:otelcol"},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(map[string]string{
				conventions127.AttributeDeploymentEnvironmentName: "new_env",
				"custom_attribute": "custom_value",
			}),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric": "int.gauge",
						"points": []any{[]any{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:new_env"},
					},
					map[string]any{
						"metric": "otel.system.filesystem.utilization",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:new_env"},
					},
					map[string]any{
						"metric": "double.histogram.bucket",
						"points": []any{[]any{float64(0), float64(2)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []any{"lower_bound:-inf", "upper_bound:0", "env:new_env"},
					},
					map[string]any{
						"metric": "double.histogram.bucket",
						"points": []any{[]any{float64(0), float64(18)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []any{"lower_bound:0", "upper_bound:inf", "env:new_env"},
					},
					map[string]any{
						"metric": "system.disk.in_use",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:new_env"},
					},
					map[string]any{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []any{[]any{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"version:latest", "command:otelcol"},
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
			histogramMode: datadogconfig.HistogramModeDistributions,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric": "int.gauge",
						"points": []any{[]any{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "otel.system.filesystem.utilization",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "system.disk.in_use",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []any{[]any{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"version:latest", "command:otelcol"},
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
			histogramMode: datadogconfig.HistogramModeCounters,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric": "int.gauge",
						"points": []any{[]any{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric": "otel.system.filesystem.utilization",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric": "double.histogram.bucket",
						"points": []any{[]any{float64(0), float64(2)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []any{"lower_bound:-inf", "upper_bound:0", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric": "double.histogram.bucket",
						"points": []any{[]any{float64(0), float64(18)}},
						"type":   "count",
						"host":   "test-host",
						"tags":   []any{"lower_bound:0", "upper_bound:inf", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric": "system.disk.in_use",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []any{[]any{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
				},
			},
			expectedSketchPayload: nil,
			expectedErr:           nil,
		},
		{
			metrics: createTestMetrics(map[string]string{
				conventions.AttributeDeploymentEnvironment: "dev",
				"custom_attribute":                         "custom_value",
			}),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeDistributions,
			hostTags:      []string{"key1:value1", "key2:value2"},
			expectedSeries: map[string]any{
				"series": []any{
					map[string]any{
						"metric": "int.gauge",
						"points": []any{[]any{float64(0), float64(222)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "otel.system.filesystem.utilization",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "system.disk.in_use",
						"points": []any{[]any{float64(0), float64(333)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"env:dev"},
					},
					map[string]any{
						"metric": "otel.datadog_exporter.metrics.running",
						"points": []any{[]any{float64(0), float64(1)}},
						"type":   "gauge",
						"host":   "test-host",
						"tags":   []any{"version:latest", "command:otelcol"},
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
	}
	gatewayUsage := attributes.NewGatewayUsage()
	for _, tt := range tests {
		t.Run(fmt.Sprintf("kind=%s,histogramMode=%s", tt.source.Kind, tt.histogramMode), func(t *testing.T) {
			seriesRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.MetricV1Endpoint}
			sketchRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.SketchesMetricEndpoint}
			server := testutil.DatadogServerMock(
				seriesRecorder.HandlerFunc,
				sketchRecorder.HandlerFunc,
			)
			defer server.Close()

			var once sync.Once
			pusher := newTestPusher(t)
			reporter, err := inframetadata.NewReporter(zap.NewNop(), pusher, 1*time.Second)
			require.NoError(t, err)
			attributesTranslator, err := attributes.NewTranslator(componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)
			acfg := traceconfig.New()
			exp, err := newMetricsExporter(
				context.Background(),
				exportertest.NewNopSettings(metadata.Type),
				newTestConfig(t, server.URL, tt.hostTags, tt.histogramMode),
				acfg,
				&once,
				attributesTranslator,
				&testutil.MockSourceProvider{Src: tt.source},
				reporter,
				nil,
				gatewayUsage,
			)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			assert.NoError(t, err, "unexpected error")
			exp.getPushTime = func() uint64 { return 0 }
			err = exp.PushMetricsData(context.Background(), tt.metrics)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err, "expected error doesn't match")
				return
			}
			assert.NoError(t, err, "unexpected error")
			if len(tt.expectedSeries) == 0 {
				assert.Nil(t, seriesRecorder.ByteBody)
			} else {
				assert.Equal(t, "gzip", seriesRecorder.Header.Get("Accept-Encoding"))
				assert.Equal(t, "application/json", seriesRecorder.Header.Get("Content-Type"))
				assert.Equal(t, "otelcol/latest", seriesRecorder.Header.Get("User-Agent"))
				assert.NoError(t, err)
				var actual map[string]any
				assert.NoError(t, json.Unmarshal(seriesRecorder.ByteBody, &actual))
				assert.Equal(t, tt.expectedSeries, actual)
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
		})
	}
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

func newTestConfig(t *testing.T, endpoint string, hostTags []string, histogramMode datadogconfig.HistogramMode) *datadogconfig.Config {
	t.Helper()
	return &datadogconfig.Config{
		HostMetadata: datadogconfig.HostMetadataConfig{
			Tags: hostTags,
		},
		TagsConfig: datadogconfig.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: endpoint,
			},
			HistConfig: datadogconfig.HistogramConfig{
				Mode: histogramMode,
			},
			// Set values to avoid errors. No particular intention in value selection.
			DeltaTTL: 3600,
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeRawValue,
			},
		},
	}
}
