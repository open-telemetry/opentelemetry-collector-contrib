// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/featuregates"
)

func TestNewExporter(t *testing.T) {
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
		HostnameDetectionTimeout: 50 * time.Millisecond,
	}

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetrics(t.Context(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(t.Context(), testMetrics)
	require.NoError(t, err)
	assert.Empty(t, server.MetadataChan)

	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(t.Context(), testMetrics)
	require.NoError(t, err)
	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func TestNewExporter_Serializer(t *testing.T) {
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
		HostnameDetectionTimeout: 50 * time.Millisecond,
	}

	params := exportertest.NewNopSettings(metadata.Type)
	var err error
	params.Logger, err = zap.NewDevelopment()
	require.NoError(t, err)
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetrics(t.Context(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(t.Context(), testMetrics)
	require.NoError(t, err)
	assert.Empty(t, server.MetadataChan)

	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(t.Context(), testMetrics)
	require.NoError(t, err)
	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func Test_metricsExporter_PushMetricsData(t *testing.T) {
	attrs := map[string]string{
		"deployment.environment": "dev",
		"custom_attribute":       "custom_value",
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
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:-inf", "upper_bound:0", "env:dev"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:0", "upper_bound:inf", "env:dev"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
				},
			},
		},
		{
			metrics: createTestMetrics(map[string]string{
				"deployment.environment.name": "new_env",
				"custom_attribute":            "custom_value",
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
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:new_env"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:new_env"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:-inf", "upper_bound:0", "env:new_env"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:0", "upper_bound:inf", "env:new_env"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:new_env"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
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
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
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
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "otel.system.filesystem.utilization",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(2)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:-inf", "upper_bound:0", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "double.histogram.bucket",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(18)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_COUNT),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"lower_bound:0", "upper_bound:inf", "env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "system.disk.in_use",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(333)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"env:dev", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "datadog.otel.gateway",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(0)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
					map[string]any{
						"metric":    "otel.datadog_exporter.metrics.running",
						"points":    []any{map[string]any{"timestamp": float64(0), "value": float64(1)}},
						"type":      float64(datadogV2.METRICINTAKETYPE_GAUGE),
						"interval":  float64(0),
						"resources": []any{map[string]any{"name": "test-host", "type": "host"}},
						"tags":      []any{"version:latest", "command:otelcol", "key1:value1", "key2:value2"},
					},
				},
			},
		},
		{
			metrics: loadOTLPMetrics(t, "metrics_stats.json"),
			source: source.Source{
				Kind:       source.HostnameKind,
				Identifier: "test-host",
			},
			histogramMode: datadogconfig.HistogramModeDistributions,
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
				t.Context(),
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
			err = exp.PushMetricsData(t.Context(), tt.metrics)
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

func TestNewExporterWithProxy(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	type requestInfo struct {
		Path    string
		Headers map[string]string
	}
	var proxyRequests []requestInfo
	var mu sync.Mutex
	var wg sync.WaitGroup

	expectedRequests := 7

	wg.Add(expectedRequests)

	proxyServer := httptest.NewServer(&httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.Header.Set("X-Proxy-Test", "test-proxy-123")
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()

			// Copy request data to avoid race conditions
			headers := make(map[string]string)
			for key, values := range req.Header {
				if len(values) > 0 {
					headers[key] = values[0]
				}
			}

			mu.Lock()
			proxyRequests = append(proxyRequests, requestInfo{
				Path:    req.URL.Path,
				Headers: headers,
			})
			mu.Unlock()

			wg.Done()
		},
	})
	defer proxyServer.Close()

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: proxyServer.URL,
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
		HostnameDetectionTimeout: 50 * time.Millisecond,

		ClientConfig: confighttp.ClientConfig{
			ProxyURL: proxyServer.URL,
		},
	}

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()

	// The client should have been created correctly
	exp, err := f.CreateMetrics(t.Context(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)

	// Create & send test metrics (no metadata)
	testMetrics := pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(t.Context(), testMetrics)
	require.NoError(t, err)
	assert.Empty(t, server.MetadataChan)

	// Send another with metadata
	testMetrics = pmetric.NewMetrics()
	testutil.TestMetrics.CopyTo(testMetrics)
	err = exp.ConsumeMetrics(t.Context(), testMetrics)
	require.NoError(t, err)

	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)

	// Wait for all requests to be processed
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()

	assert.GreaterOrEqual(t, len(proxyRequests), 3, "Expected at least 3 requests to go through the proxy")

	// Verify got metrics & sketches requests
	hasMetricsRequest := false
	hasSketchesRequest := false
	for _, req := range proxyRequests {
		if req.Path == "/api/v2/series" {
			hasMetricsRequest = true
		}
		if req.Path == "/api/beta/sketches" || req.Path == "/api/v1/sketches" {
			hasSketchesRequest = true
		}
	}
	assert.True(t, hasMetricsRequest, "Expected to capture metrics request to /api/v2/series")
	assert.True(t, hasSketchesRequest, "Expected to capture sketches request")

	for _, req := range proxyRequests {
		assert.Equal(t, "test-proxy-123", req.Headers["X-Proxy-Test"],
			"Request should have gone through our proxy")

		assert.Equal(t, "gzip", req.Headers["Accept-Encoding"])
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
	cfg := datadogconfig.CreateDefaultConfig().(*datadogconfig.Config)
	cfg.HostMetadata = datadogconfig.HostMetadataConfig{
		Tags: hostTags,
	}
	cfg.TagsConfig = datadogconfig.TagsConfig{
		Hostname: "test-host",
	}

	cfg.Metrics = datadogconfig.MetricsConfig{
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
	}

	return cfg
}

func loadOTLPMetrics(t *testing.T, filename string) pmetric.Metrics {
	t.Helper()
	otlpbytes, err := os.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err)

	var unmarshaler pmetric.JSONUnmarshaler
	otlpmetrics, err := unmarshaler.UnmarshalMetrics(otlpbytes)
	require.NoError(t, err)

	return otlpmetrics
}

func createTestMetricsWithRuntimeMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	metricsArray := ilm.Metrics()

	runtimeMetrics := []string{
		"system.filesystem.utilization",
		"process.runtime.go.goroutines",
		"process.runtime.dotnet.exceptions.count",
		"process.runtime.jvm.threads.count",
	}

	for _, metricName := range runtimeMetrics {
		met := metricsArray.AppendEmpty()
		met.SetName(metricName)
		dps := met.SetEmptyGauge().DataPoints()
		dp := dps.AppendEmpty()
		dp.SetTimestamp(0)
		dp.SetIntValue(42)
	}

	return md
}

func TestMetricRemapping(t *testing.T) {
	tests := []struct {
		newGate         bool
		oldGate         bool
		expectedMetrics []string
	}{
		{
			newGate: false,
			oldGate: false,
			expectedMetrics: []string{
				// Mapped system metrics
				"system.disk.in_use",
				"otel.system.filesystem.utilization",
				// Mapped runtime metrics
				"runtime.go.num_goroutine",
				"otel.process.runtime.go.goroutines",
				"runtime.dotnet.exceptions.count",
				"otel.process.runtime.dotnet.exceptions.count",
				"jvm.thread_count",
				"otel.process.runtime.jvm.threads.count",
				// One per lang
				"otel.datadog_exporter.runtime_metrics.running",
				"otel.datadog_exporter.runtime_metrics.running",
				"otel.datadog_exporter.runtime_metrics.running",
			},
		},
		{
			newGate: true,
			oldGate: false,
			expectedMetrics: []string{
				// Unmapped system metrics
				"system.filesystem.utilization",
				// Unmapped runtime metrics
				"process.runtime.go.goroutines",
				"process.runtime.dotnet.exceptions.count",
				"process.runtime.jvm.threads.count",
			},
		},
		{
			newGate: false,
			oldGate: true,
			expectedMetrics: []string{
				// Unmapped system metrics
				"system.filesystem.utilization",
				// Mapped runtime metrics
				"runtime.go.num_goroutine",
				"process.runtime.go.goroutines",
				"runtime.dotnet.exceptions.count",
				"process.runtime.dotnet.exceptions.count",
				"jvm.thread_count",
				"process.runtime.jvm.threads.count",
				// One per lang
				"otel.datadog_exporter.runtime_metrics.running",
				"otel.datadog_exporter.runtime_metrics.running",
				"otel.datadog_exporter.runtime_metrics.running",
			},
		},
		{
			newGate: true,
			oldGate: true,
			expectedMetrics: []string{
				// Unmapped system metrics
				"system.filesystem.utilization",
				// Unmapped runtime metrics
				"process.runtime.go.goroutines",
				"process.runtime.dotnet.exceptions.count",
				"process.runtime.jvm.threads.count",
			},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("new=%v,old=%v", tt.newGate, tt.oldGate), func(t *testing.T) {
			seriesRecorder := &testutil.HTTPRequestRecorder{Pattern: testutil.MetricV2Endpoint}
			server := testutil.DatadogServerMock(seriesRecorder.HandlerFunc)
			defer server.Close()

			reg := featuregate.GlobalRegistry()
			prevNewVal := featuregates.DisableMetricRemappingFeatureGate.IsEnabled()
			prevOldVal := featuregates.MetricRemappingDisabledFeatureGate.IsEnabled()
			require.NoError(t, reg.Set(featuregates.DisableMetricRemappingFeatureGate.ID(), tt.newGate))
			require.NoError(t, reg.Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), tt.oldGate))
			defer func() {
				require.NoError(t, reg.Set(featuregates.DisableMetricRemappingFeatureGate.ID(), prevNewVal))
				require.NoError(t, reg.Set(featuregates.MetricRemappingDisabledFeatureGate.ID(), prevOldVal))
			}()

			var once sync.Once
			pusher := newTestPusher(t)
			reporter, err := inframetadata.NewReporter(zap.NewNop(), pusher, 1*time.Second)
			require.NoError(t, err)
			attributesTranslator, err := attributes.NewTranslator(componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)
			acfg := traceconfig.New()
			gatewayUsage := attributes.NewGatewayUsage()

			// TODO: Update to test with serializer and legacy paths.
			// Right now we only test the legacy path as the serializer exporter needs to be updated in a separate PR.
			exp, err := newMetricsExporter(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				newTestConfig(t, server.URL, []string{}, datadogconfig.HistogramModeDistributions),
				acfg,
				&once,
				attributesTranslator,
				&testutil.MockSourceProvider{},
				reporter,
				nil,
				gatewayUsage,
			)
			require.NoError(t, err)

			// Push metrics and validate output
			if len(tt.expectedMetrics) > 0 {
				exp.getPushTime = func() uint64 { return 0 }
				testMetrics := createTestMetricsWithRuntimeMetrics()
				err = exp.PushMetricsData(t.Context(), testMetrics)
				require.NoError(t, err)

				// Parse the series payload
				reader, err := gzip.NewReader(bytes.NewBuffer(seriesRecorder.ByteBody))
				require.NoError(t, err)
				var payload map[string]any
				require.NoError(t, json.NewDecoder(reader).Decode(&payload))

				// Extract and validate metric names
				actualMetrics := make([]string, 0, len(payload["series"].([]any)))
				for _, s := range payload["series"].([]any) {
					actualMetrics = append(actualMetrics, s.(map[string]any)["metric"].(string))
				}
				assert.ElementsMatch(t, tt.expectedMetrics, actualMetrics)
			}
		})
	}
}
