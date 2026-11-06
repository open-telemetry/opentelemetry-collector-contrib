// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

// unmarshalConfig loads a raw configuration the way the collector does.
func unmarshalConfig(t *testing.T, raw map[string]any) (*Config, error) {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	return cfg, confmap.NewFromStringMap(raw).Unmarshal(cfg)
}

func TestOTLPHTTPConfigBackwardCompatible(t *testing.T) {
	// A configuration without protocol::otlp_http must load exactly as before.
	cfg, err := unmarshalConfig(t, map[string]any{
		"routing_key": "service",
		"protocol": map[string]any{
			"otlp": map[string]any{
				"timeout":           "5s",
				"retry_on_failure":  map[string]any{"enabled": false},
				"tls":               map[string]any{"insecure": true},
				"sending_queue":     map[string]any{"enabled": false},
				"compression":       "none",
				"endpoint":          "placeholder:4317",
				"keepalive":         map[string]any{"time": "1m"},
				"balancer_name":     "round_robin",
				"write_buffer_size": 1024,
			},
		},
		"resolver": map[string]any{"static": map[string]any{"hostnames": []any{"a:4317"}}},
	})
	require.NoError(t, err)
	require.NoError(t, confmap.Validate(cfg))

	assert.False(t, cfg.Protocol.OTLPHTTP.HasValue())
	assert.Equal(t, "placeholder:4317", cfg.Protocol.OTLP.ClientConfig.Endpoint)
	assert.Equal(t, 5*time.Second, cfg.Protocol.OTLP.TimeoutConfig.Timeout)
	assert.False(t, cfg.Protocol.OTLP.RetryConfig.Enabled)
	assert.True(t, cfg.Protocol.OTLP.ClientConfig.TLS.Insecure)
	assert.False(t, cfg.Protocol.OTLP.QueueConfig.HasValue())
	assert.Equal(t, "round_robin", cfg.Protocol.OTLP.ClientConfig.BalancerName)

	// The default config still selects OTLP/gRPC and keeps its placeholder endpoint.
	def := createDefaultConfig().(*Config)
	assert.False(t, def.Protocol.OTLPHTTP.HasValue())
	assert.Equal(t, "placeholder:4317", def.Protocol.OTLP.ClientConfig.Endpoint)
}

func TestOTLPHTTPConfigDefaults(t *testing.T) {
	cfg, err := unmarshalConfig(t, map[string]any{
		"protocol": map[string]any{
			"otlp_http": map[string]any{
				"metrics_endpoint": "http://placeholder:8429/opentelemetry/v1/metrics",
			},
		},
		"resolver": map[string]any{"static": map[string]any{"hostnames": []any{"a:8429"}}},
	})
	require.NoError(t, err)
	require.NoError(t, confmap.Validate(cfg))

	require.True(t, cfg.Protocol.OTLPHTTP.HasValue())
	h := cfg.Protocol.OTLPHTTP.Get()
	// Defaults come from otlphttpexporter.
	assert.Equal(t, otlphttpexporter.EncodingProto, h.Encoding)
	assert.Equal(t, configcompression.TypeGzip, h.ClientConfig.Compression)
	assert.Equal(t, 30*time.Second, h.ClientConfig.Timeout)
	assert.True(t, h.RetryConfig.Enabled)
	assert.True(t, h.QueueConfig.HasValue())
}

func TestOTLPHTTPConfigOverrides(t *testing.T) {
	cfg, err := unmarshalConfig(t, map[string]any{
		"protocol": map[string]any{
			"otlp_http": map[string]any{
				"endpoint":         "https://placeholder",
				"encoding":         "json",
				"compression":      "none",
				"timeout":          "5s",
				"retry_on_failure": map[string]any{"enabled": false},
				"sending_queue":    map[string]any{"enabled": false},
				"headers":          map[string]any{"X-Test": "1"},
			},
		},
		"resolver": map[string]any{"static": map[string]any{"hostnames": []any{"a"}}},
	})
	require.NoError(t, err)
	require.NoError(t, confmap.Validate(cfg))

	h := cfg.Protocol.OTLPHTTP.Get()
	assert.Equal(t, otlphttpexporter.EncodingJSON, h.Encoding)
	assert.Equal(t, configcompression.Type("none"), h.ClientConfig.Compression)
	assert.Equal(t, 5*time.Second, h.ClientConfig.Timeout)
	assert.False(t, h.RetryConfig.Enabled, "retry_on_failure::enabled=false must be honored")
	assert.False(t, h.QueueConfig.HasValue())
	v, ok := h.ClientConfig.Headers.Get("X-Test")
	assert.True(t, ok)
	assert.Equal(t, "1", string(v))
}

func TestOTLPHTTPConfigInvalid(t *testing.T) {
	resolver := map[string]any{"static": map[string]any{"hostnames": []any{"a"}}}
	tests := []struct {
		name        string
		protocol    map[string]any
		unmarshal   string
		validateErr string
	}{
		{
			name: "both protocols",
			protocol: map[string]any{
				"otlp":      map[string]any{"endpoint": "placeholder:4317"},
				"otlp_http": map[string]any{"endpoint": "http://placeholder"},
			},
			unmarshal: "only one of protocol::otlp and protocol::otlp_http should be specified",
		},
		{
			name:        "no endpoint",
			protocol:    map[string]any{"otlp_http": map[string]any{"encoding": "proto"}},
			validateErr: "at least one endpoint must be specified",
		},
		{
			name:        "endpoint without scheme",
			protocol:    map[string]any{"otlp_http": map[string]any{"endpoint": "placeholder:4318"}},
			validateErr: "protocol::otlp_http::endpoint must be an http:// or https:// URL with a host",
		},
		{
			name:        "per signal endpoint without host",
			protocol:    map[string]any{"otlp_http": map[string]any{"logs_endpoint": "http:///v1/logs"}},
			validateErr: "protocol::otlp_http::logs_endpoint must be an http:// or https:// URL with a host",
		},
		{
			name:        "invalid encoding",
			protocol:    map[string]any{"otlp_http": map[string]any{"endpoint": "http://placeholder", "encoding": "xml"}},
			unmarshal:   "invalid encoding type",
			validateErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := unmarshalConfig(t, map[string]any{"protocol": tt.protocol, "resolver": resolver})
			if tt.unmarshal != "" {
				require.ErrorContains(t, err, tt.unmarshal)
				return
			}
			require.NoError(t, err)
			require.ErrorContains(t, confmap.Validate(cfg), tt.validateErr)
		})
	}
}

func TestBuildHTTPExporterConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	h := httpDefaults()
	h.ClientConfig.Endpoint = "https://placeholder/base"
	h.MetricsEndpoint = "http://placeholder:8429/opentelemetry/v1/metrics"
	h.LogsEndpoint = "http://placeholder:1/custom/logs?x=1"
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)

	first := buildHTTPExporterConfig(cfg, "10.0.0.1:4318")
	second := buildHTTPExporterConfig(cfg, "10.0.0.2:4318")

	assert.Equal(t, "https://10.0.0.1:4318/base", first.ClientConfig.Endpoint)
	assert.Equal(t, "http://10.0.0.1:4318/opentelemetry/v1/metrics", first.MetricsEndpoint)
	assert.Equal(t, "http://10.0.0.1:4318/custom/logs?x=1", first.LogsEndpoint)
	assert.Empty(t, first.TracesEndpoint, "unset endpoints stay unset")
	assert.Empty(t, first.ProfilesEndpoint)

	// Each backend gets its own copy and the shared configuration is not modified.
	assert.Equal(t, "https://10.0.0.2:4318/base", second.ClientConfig.Endpoint)
	shared := cfg.Protocol.OTLPHTTP.Get()
	assert.Equal(t, "https://placeholder/base", shared.ClientConfig.Endpoint)
	assert.Equal(t, "http://placeholder:8429/opentelemetry/v1/metrics", shared.MetricsEndpoint)
}

func TestLoadBalancerDefaultPort(t *testing.T) {
	tests := []struct {
		name     string
		http     bool
		expected string
	}{
		{name: "otlp grpc keeps 4317", expected: "host:4317"},
		{name: "otlp http uses 4318", http: true, expected: "host:4318"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"host"}})
			if tt.http {
				h := httpDefaults()
				h.ClientConfig.Endpoint = "http://placeholder"
				cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
			}
			lb, err := newLoadBalancer(exportertest.NewNopSettings(metadata.Type).Logger, cfg, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, lb.endpointWithPort("host"))
			assert.Equal(t, "host:9000", lb.endpointWithPort("host:9000"), "an explicit port is kept")
		})
	}
}

func httpDefaults() *otlphttpexporter.Config {
	return otlphttpexporter.NewFactory().CreateDefaultConfig().(*otlphttpexporter.Config)
}

type recordedRequest struct {
	path            string
	contentType     string
	contentEncoding string
	header          string
	body            []byte
}

func newRecordingServer(t *testing.T) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var got []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, recordedRequest{
			path:            r.URL.Path,
			contentType:     r.Header.Get("Content-Type"),
			contentEncoding: r.Header.Get("Content-Encoding"),
			header:          r.Header.Get("X-Test"),
			body:            body,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), got...)
	}
}

// httpTestConfig routes to the test server with synchronous sub-exporters.
func httpTestConfig(t *testing.T, srv *httptest.Server, mutate func(*otlphttpexporter.Config)) *Config {
	t.Helper()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{u.Host}})

	h := httpDefaults()
	h.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	h.RetryConfig.Enabled = false
	mutate(h)
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	require.NoError(t, confmap.Validate(cfg))
	return cfg
}

func TestOTLPHTTPExportAllSignals(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*otlphttpexporter.Config)
		expected map[string]string // signal -> path
	}{
		{
			name:   "endpoint only gets the default paths",
			mutate: func(h *otlphttpexporter.Config) { h.ClientConfig.Endpoint = "http://placeholder" },
			expected: map[string]string{
				"traces": "/v1/traces", "metrics": "/v1/metrics", "logs": "/v1/logs",
			},
		},
		{
			name: "endpoint with a base path",
			mutate: func(h *otlphttpexporter.Config) {
				h.ClientConfig.Endpoint = "http://placeholder/prefix/"
			},
			expected: map[string]string{
				"traces": "/prefix/v1/traces", "metrics": "/prefix/v1/metrics", "logs": "/prefix/v1/logs",
			},
		},
		{
			name: "per signal endpoints keep custom paths",
			mutate: func(h *otlphttpexporter.Config) {
				h.TracesEndpoint = "http://placeholder:1/t"
				h.MetricsEndpoint = "http://placeholder:8429/opentelemetry/v1/metrics"
				h.LogsEndpoint = "http://placeholder:2/l"
			},
			expected: map[string]string{
				"traces": "/t", "metrics": "/opentelemetry/v1/metrics", "logs": "/l",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, requests := newRecordingServer(t)
			cfg := httpTestConfig(t, srv, tt.mutate)
			ctx := t.Context()
			set := exportertest.NewNopSettings(metadata.Type)
			host := componenttest.NewNopHost()

			te, err := createTracesExporter(ctx, set, cfg)
			require.NoError(t, err)
			require.NoError(t, te.Start(ctx, host))
			t.Cleanup(func() { require.NoError(t, te.Shutdown(context.WithoutCancel(t.Context()))) })
			me, err := createMetricsExporter(ctx, set, cfg)
			require.NoError(t, err)
			require.NoError(t, me.Start(ctx, host))
			t.Cleanup(func() { require.NoError(t, me.Shutdown(context.WithoutCancel(t.Context()))) })
			le, err := createLogsExporter(ctx, set, cfg)
			require.NoError(t, err)
			require.NoError(t, le.Start(ctx, host))
			t.Cleanup(func() { require.NoError(t, le.Shutdown(context.WithoutCancel(t.Context()))) })

			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", "svc")
			rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("span")
			require.NoError(t, te.ConsumeTraces(ctx, td))

			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			rm.Resource().Attributes().PutStr("service.name", "svc")
			rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric")
			require.NoError(t, me.ConsumeMetrics(ctx, md))

			ld := plog.NewLogs()
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", "svc")
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
			require.NoError(t, le.ConsumeLogs(ctx, ld))

			got := requests()
			require.Len(t, got, 3)
			paths := map[string]bool{}
			for _, r := range got {
				paths[r.path] = true
				assert.Equal(t, "application/x-protobuf", r.contentType)
				assert.Equal(t, "gzip", r.contentEncoding)
			}
			for _, p := range tt.expected {
				assert.True(t, paths[p], "missing request on %s, got %v", p, paths)
			}
		})
	}
}

func TestOTLPHTTPExportJSONWithHeaders(t *testing.T) {
	srv, requests := newRecordingServer(t)
	cfg := httpTestConfig(t, srv, func(h *otlphttpexporter.Config) {
		h.ClientConfig.Endpoint = "http://placeholder"
		h.Encoding = otlphttpexporter.EncodingJSON
		h.ClientConfig.Compression = ""
		h.ClientConfig.Headers.Set("X-Test", "value")
	})
	ctx := t.Context()
	me, err := createMetricsExporter(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, me.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, me.Shutdown(context.WithoutCancel(t.Context()))) })

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric")
	require.NoError(t, me.ConsumeMetrics(ctx, md))

	got := requests()
	require.Len(t, got, 1)
	assert.Equal(t, "application/json", got[0].contentType)
	assert.Empty(t, got[0].contentEncoding)
	assert.Equal(t, "value", got[0].header)
	assert.Contains(t, string(got[0].body), `"metric"`)
}
