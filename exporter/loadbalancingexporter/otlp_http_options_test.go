// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

// TestOTLPHTTPAllOptions checks that every option of the OTLP/HTTP exporter can be set under
// protocol::otlp_http and reaches the sub-exporter configuration.
func TestOTLPHTTPAllOptions(t *testing.T) {
	cfg, err := unmarshalConfig(t, map[string]any{
		"protocol": map[string]any{
			"otlp_http": map[string]any{
				"endpoint":                "http://placeholder:4318",
				"traces_endpoint":         "http://placeholder:4318/t",
				"metrics_endpoint":        "http://placeholder:4318/m",
				"logs_endpoint":           "http://placeholder:4318/l",
				"encoding":                "json",
				"timeout":                 "7s",
				"compression":             "zstd",
				"proxy_url":               "http://proxy:3128",
				"read_buffer_size":        1024,
				"write_buffer_size":       2048,
				"headers":                 map[string]any{"X-A": "a"},
				"tls":                     map[string]any{"insecure_skip_verify": true, "min_version": "1.2"},
				"auth":                    map[string]any{"authenticator": "basicauth/client"},
				"middlewares":             []any{map[string]any{"id": "limiter"}},
				"keepalive":               map[string]any{"max_idle_conns": 11, "max_idle_conns_per_host": 12, "idle_conn_timeout": "14s"},
				"max_conns_per_host":      13,
				"force_attempt_http2":     true,
				"http2_read_idle_timeout": "15s",
				"http2_ping_timeout":      "16s",
				"cookies":                 map[string]any{},
				"sending_queue":           map[string]any{"enabled": true, "queue_size": 17, "num_consumers": 2},
				"retry_on_failure":        map[string]any{"enabled": true, "initial_interval": "3s", "max_interval": "30s", "max_elapsed_time": "1m"},
			},
		},
		"resolver": map[string]any{"static": map[string]any{"hostnames": []any{"a"}}},
	})
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())

	h := buildHTTPExporterConfig(cfg, "10.0.0.1:4318")
	assert.Equal(t, "http://10.0.0.1:4318", h.ClientConfig.Endpoint)
	assert.Equal(t, "http://10.0.0.1:4318/t", h.TracesEndpoint)
	assert.Equal(t, "http://10.0.0.1:4318/m", h.MetricsEndpoint)
	assert.Equal(t, "http://10.0.0.1:4318/l", h.LogsEndpoint)
	assert.Equal(t, otlphttpexporter.EncodingJSON, h.Encoding)
	assert.Equal(t, 7*time.Second, h.ClientConfig.Timeout)
	assert.Equal(t, "zstd", string(h.ClientConfig.Compression))
	assert.Equal(t, "http://proxy:3128", h.ClientConfig.ProxyURL)
	assert.Equal(t, 1024, h.ClientConfig.ReadBufferSize)
	assert.Equal(t, 2048, h.ClientConfig.WriteBufferSize)
	assert.True(t, h.ClientConfig.TLS.InsecureSkipVerify)
	assert.Equal(t, "1.2", h.ClientConfig.TLS.MinVersion)
	require.True(t, h.ClientConfig.Auth.HasValue())
	assert.Equal(t, component.MustNewIDWithName("basicauth", "client"), h.ClientConfig.Auth.Get().AuthenticatorID)
	require.Len(t, h.ClientConfig.Middlewares, 1)
	require.True(t, h.ClientConfig.Keepalive.HasValue())
	assert.Equal(t, 11, h.ClientConfig.Keepalive.Get().MaxIdleConns)
	assert.Equal(t, 12, h.ClientConfig.Keepalive.Get().MaxIdleConnsPerHost)
	assert.Equal(t, 14*time.Second, h.ClientConfig.Keepalive.Get().IdleConnTimeout)
	assert.Equal(t, 13, h.ClientConfig.MaxConnsPerHost)
	assert.True(t, h.ClientConfig.ForceAttemptHTTP2)
	assert.Equal(t, 15*time.Second, h.ClientConfig.HTTP2ReadIdleTimeout)
	assert.Equal(t, 16*time.Second, h.ClientConfig.HTTP2PingTimeout)
	assert.True(t, h.ClientConfig.Cookies.HasValue())
	require.True(t, h.QueueConfig.HasValue())
	assert.EqualValues(t, 17, h.QueueConfig.Get().QueueSize)
	assert.Equal(t, 3*time.Second, h.RetryConfig.InitialInterval)
	assert.Equal(t, time.Minute, h.RetryConfig.MaxElapsedTime)
}

// TestOTLPHTTPTLSBackend sends over https, which checks that the scheme of the endpoint is kept.
func TestOTLPHTTPTLSBackend(t *testing.T) {
	var hits int
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{u.Host}})
	h := httpDefaults()
	h.ClientConfig.Endpoint = "https://placeholder"
	h.ClientConfig.TLS.InsecureSkipVerify = true
	h.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	h.RetryConfig.Enabled = false
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	require.NoError(t, cfg.Validate())

	ctx := t.Context()
	me, err := createMetricsExporter(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, me.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, me.Shutdown(t.Context())) })

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric")
	require.NoError(t, me.ConsumeMetrics(ctx, md))
	assert.Equal(t, 1, hits)
}
