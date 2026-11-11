// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

func httpConfigWithEndpoint(endpoint string) *Config {
	cfg := createDefaultConfig().(*Config)
	h := httpDefaults()
	h.MetricsEndpoint = endpoint
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	return cfg
}

func TestBuildHTTPExporterConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name, endpoint, hostPort, want string
	}{
		{"plain", "http://placeholder", "10.0.0.1:4318", "http://10.0.0.1:4318"},
		{"trailing slash", "http://placeholder/", "10.0.0.1:4318", "http://10.0.0.1:4318/"},
		{"base path and query", "https://placeholder:1/a/b?x=1&y=2", "h:2", "https://h:2/a/b?x=1&y=2"},
		{"fragment kept", "http://placeholder/p#f", "h:2", "http://h:2/p#f"},
		{"ipv6 backend", "http://placeholder/m", "[::1]:4318", "http://[::1]:4318/m"},
		{"ipv6 placeholder", "http://[::1]:1/m", "h:2", "http://h:2/m"},
		{"encoded path kept", "http://placeholder/a%2Fb", "h:2", "http://h:2/a%2Fb"},
		{"userinfo kept", "http://user:pw@placeholder/m", "h:2", "http://user:pw@h:2/m"},
		{"no port on placeholder", "http://placeholder", "h:2", "http://h:2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := httpConfigWithEndpoint(tt.endpoint)
			require.NoError(t, cfg.Validate())
			assert.Equal(t, tt.want, buildHTTPExporterConfig(cfg, tt.hostPort).MetricsEndpoint)
		})
	}
}

func TestValidateHTTPEndpointsOrderIsStable(t *testing.T) {
	cfg := &otlphttpexporter.Config{
		TracesEndpoint:  "bad",
		MetricsEndpoint: "worse",
	}
	for range 20 {
		require.ErrorContains(t, validateHTTPEndpoints(cfg), "traces_endpoint")
	}
}

func TestEndpointWithDefaultPortIPv6(t *testing.T) {
	assert.Equal(t, "[fd00::1]:4318", endpointWithDefaultPort("[fd00::1]", "4318"))
	assert.Equal(t, "[fd00::1]:9000", endpointWithDefaultPort("[fd00::1]:9000", "4318"))
	assert.Equal(t, "host:4317", endpointWithPort("host"), "gRPC default is unchanged")
}

func TestValidateHTTPEndpointQueryAndFragment(t *testing.T) {
	cfg := httpConfigWithEndpoint("http://placeholder/m?x=1#f")
	require.NoError(t, cfg.Validate(), "per signal endpoints may carry a query")

	for _, endpoint := range []string{"http://placeholder?x=1", "http://placeholder/#f"} {
		h := httpDefaults()
		h.ClientConfig.Endpoint = endpoint
		require.ErrorContains(t, validateHTTPEndpoints(h), "must not have a query or a fragment")
	}
}

func TestCheckHTTPSignalEndpoint(t *testing.T) {
	grpc := createDefaultConfig().(*Config)
	for _, signal := range []string{"traces", "metrics", "logs"} {
		require.NoError(t, checkHTTPSignalEndpoint(grpc, signal), "gRPC is never checked")
	}

	cfg := createDefaultConfig().(*Config)
	h := httpDefaults()
	h.MetricsEndpoint = "http://placeholder/m"
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	require.NoError(t, checkHTTPSignalEndpoint(cfg, "metrics"))
	require.ErrorContains(t, checkHTTPSignalEndpoint(cfg, "traces"), "either endpoint or traces_endpoint")
	require.ErrorContains(t, checkHTTPSignalEndpoint(cfg, "logs"), "either endpoint or logs_endpoint")

	h.ClientConfig.Endpoint = "http://placeholder"
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	for _, signal := range []string{"traces", "metrics", "logs"} {
		require.NoError(t, checkHTTPSignalEndpoint(cfg, signal))
	}
}

func TestExporterFailsAtStartupWithoutSignalEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"a"}})
	h := httpDefaults()
	h.MetricsEndpoint = "http://placeholder/m"
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	set := exportertest.NewNopSettings(metadata.Type)

	_, err := createMetricsExporter(t.Context(), set, cfg)
	require.NoError(t, err)
	_, err = createTracesExporter(t.Context(), set, cfg)
	require.ErrorContains(t, err, "traces_endpoint")
	_, err = createLogsExporter(t.Context(), set, cfg)
	require.ErrorContains(t, err, "logs_endpoint")
}

func TestConfigMarshalRoundTrip(t *testing.T) {
	// A marshaled default config has null protocol values and must load back unchanged.
	def := createDefaultConfig().(*Config)
	def.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: []string{"a"}})
	conf := confmap.New()
	require.NoError(t, conf.Marshal(def))

	loaded := createDefaultConfig().(*Config)
	require.NoError(t, conf.Unmarshal(loaded))
	assert.False(t, loaded.Protocol.OTLPHTTP.HasValue())
	assert.Equal(t, def.Protocol.OTLP.ClientConfig.Endpoint, loaded.Protocol.OTLP.ClientConfig.Endpoint)
	require.NoError(t, loaded.Validate())

	// An explicit null does not select OTLP/HTTP and does not clash with otlp.
	cfg, err := unmarshalConfig(t, map[string]any{
		"protocol": map[string]any{"otlp": map[string]any{"endpoint": "placeholder:4317"}, "otlp_http": nil},
		"resolver": map[string]any{"static": map[string]any{"hostnames": []any{"a"}}},
	})
	require.NoError(t, err)
	assert.False(t, cfg.Protocol.OTLPHTTP.HasValue())
}
