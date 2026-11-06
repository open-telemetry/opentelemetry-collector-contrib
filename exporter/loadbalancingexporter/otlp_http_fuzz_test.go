// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
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

func FuzzValidateHTTPEndpoints(f *testing.F) {
	for _, seed := range []string{
		"", "http://placeholder", "https://placeholder:4318/v1/metrics", "placeholder:4318", "http:///v1",
		"http://[::1]:4318", "ftp://host", "http://user:pw@host/x?y=1#z", "://", "http://%zz", "\x00",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, endpoint string) {
		err := validateHTTPEndpoints(&otlphttpexporter.Config{TracesEndpoint: endpoint})
		if err != nil {
			return
		}
		if endpoint == "" {
			return
		}
		u, perr := url.Parse(endpoint)
		require.NoError(t, perr)
		assert.Contains(t, []string{"http", "https"}, u.Scheme)
		assert.NotEmpty(t, u.Host)
	})
}

var wellFormedHost = regexp.MustCompile(`^[A-Za-z0-9.:-]+$`)

func FuzzBuildHTTPExporterConfig(f *testing.F) {
	for _, seed := range []struct{ endpoint, host string }{
		{"http://placeholder", "10.0.0.1:4318"},
		{"https://placeholder:1/a/b?x=1", "host:4318"},
		{"http://user:pw@placeholder/x#f", "[::1]:4318"},
		{"http://placeholder/", "a:1"},
	} {
		f.Add(seed.endpoint, seed.host)
	}
	f.Fuzz(func(t *testing.T, endpoint, hostPort string) {
		cfg := httpConfigWithEndpoint(endpoint)
		if cfg.Validate() != nil || endpoint == "" {
			return
		}
		before := cfg.Protocol.OTLPHTTP.Get().MetricsEndpoint

		out := buildHTTPExporterConfig(cfg, hostPort)

		assert.Equal(t, before, cfg.Protocol.OTLPHTTP.Get().MetricsEndpoint, "shared config must not change")
		assert.Empty(t, out.TracesEndpoint)

		// The result only has to be checked for well formed host:port backends, as returned by resolvers.
		host, port, err := net.SplitHostPort(hostPort)
		if err != nil || host == "" || !wellFormedHost.MatchString(host) {
			return
		}
		if _, convErr := strconv.ParseUint(port, 10, 16); convErr != nil {
			return
		}
		if strings.HasPrefix(hostPort, "[") {
			if addr, ipErr := netip.ParseAddr(host); ipErr != nil || !addr.Is6() {
				return
			}
		}
		orig, err := url.Parse(endpoint)
		require.NoError(t, err)
		got, err := url.Parse(out.MetricsEndpoint)
		require.NoError(t, err)
		assert.Equal(t, orig.Scheme, got.Scheme)
		assert.Equal(t, orig.Path, got.Path)
		assert.Equal(t, orig.RawQuery, got.RawQuery)
		assert.Equal(t, hostPort, got.Host)
	})
}

func FuzzEndpointWithDefaultPort(f *testing.F) {
	for _, seed := range []string{"", "host", "host:1", "[::1]:4318", "10.0.0.1", ":"} {
		f.Add(seed, "4318")
	}
	f.Fuzz(func(t *testing.T, endpoint, port string) {
		got := endpointWithDefaultPort(endpoint, port)
		bareIPv6 := strings.HasPrefix(endpoint, "[") && strings.HasSuffix(endpoint, "]")
		if strings.Contains(endpoint, ":") && !bareIPv6 {
			assert.Equal(t, endpoint, got, "an endpoint with a port is kept")
			return
		}
		assert.Equal(t, endpoint+":"+port, got)
		// Idempotent once a port was added, as the load balancer applies it repeatedly.
		if port != "" {
			assert.Equal(t, got, endpointWithDefaultPort(got, port))
		}
	})
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
