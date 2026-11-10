// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
)

func TestHTTPMetricsHTTPS(t *testing.T) {
	// This test verifies HTTPS configuration is preserved
	cfg := &Config{
		Protocol: Protocol{
			OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "https://example.com:4318",
				},
			}),
		},
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{
				Hostnames: []string{"backend1:4318"},
			}),
		},
	}

	// Validate configuration
	err := cfg.Validate()
	require.NoError(t, err)

	// Verify the protocol is HTTP
	require.True(t, cfg.Protocol.OTLPHTTP.HasValue())
	require.Equal(t, defaultHTTPPort, cfg.defaultPort())
}

func TestMutualExclusionValidation(t *testing.T) {
	cfg := &Config{
		Protocol: Protocol{
			OTLP: configoptional.Some(otlpexporter.Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "localhost:4317",
				},
			}),
			OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "localhost:4318",
				},
			}),
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "only one protocol can be configured")
}

func TestRewriteHTTPEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		configured  string
		resolved    string
		expected    string
		expectError bool
	}{
		{
			name:       "simple endpoint",
			configured: "http://placeholder:4318/v1/metrics",
			resolved:   "backend1:4318",
			expected:   "http://backend1:4318/v1/metrics",
		},
		{
			name:       "https endpoint",
			configured: "https://placeholder:4318/v1/metrics",
			resolved:   "backend1:4318",
			expected:   "https://backend1:4318/v1/metrics",
		},
		{
			name:       "custom path",
			configured: "http://placeholder:8429/opentelemetry/v1/metrics",
			resolved:   "vmagent-1:8429",
			expected:   "http://vmagent-1:8429/opentelemetry/v1/metrics",
		},
		{
			name:       "with query params",
			configured: "http://placeholder:4318/v1/metrics?param=value",
			resolved:   "backend1:4318",
			expected:   "http://backend1:4318/v1/metrics?param=value",
		},
		{
			name:        "empty endpoint",
			configured:  "",
			resolved:    "backend1:4318",
			expectError: true,
		},
		{
			name:        "invalid endpoint",
			configured:  "::invalid-url",
			resolved:    "backend1:4318",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriteHTTPEndpoint(tt.configured, tt.resolved)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSelectedProtocol(t *testing.T) {
	t.Run("gRPC default", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLP: configoptional.Some(otlpexporter.Config{
					ClientConfig: configgrpc.ClientConfig{
						Endpoint: "localhost:4317",
					},
				}),
			},
		}
		require.True(t, cfg.Protocol.OTLP.HasValue())
		require.False(t, cfg.Protocol.OTLPHTTP.HasValue())
		require.Equal(t, defaultGRPCPort, cfg.defaultPort())
	})

	t.Run("HTTP explicit", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: "localhost:4318",
					},
				}),
			},
		}
		require.True(t, cfg.Protocol.OTLPHTTP.HasValue())
		require.Equal(t, defaultHTTPPort, cfg.defaultPort())
	})

	t.Run("neither configured", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLP:     configoptional.None[otlpexporter.Config](),
				OTLPHTTP: configoptional.None[otlphttpexporter.Config](),
			},
		}
		require.False(t, cfg.Protocol.OTLPHTTP.HasValue())
		require.Equal(t, defaultGRPCPort, cfg.defaultPort())
	})
}

func TestHTTPEncodingDefaults(t *testing.T) {
	t.Run("default encoding", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: "http://placeholder:4318/v1/metrics",
					},
				}),
			},
		}
		httpCfg, err := buildHTTPExporterConfig(cfg, "backend:4318")
		require.NoError(t, err)
		require.Equal(t, otlphttpexporter.EncodingProto, httpCfg.Encoding)
	})

	t.Run("explicit proto encoding", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: "http://placeholder:4318/v1/metrics",
					},
					Encoding: otlphttpexporter.EncodingProto,
				}),
			},
		}
		httpCfg, err := buildHTTPExporterConfig(cfg, "backend:4318")
		require.NoError(t, err)
		require.Equal(t, otlphttpexporter.EncodingProto, httpCfg.Encoding)
	})

	t.Run("explicit json encoding", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
					ClientConfig: confighttp.ClientConfig{
						Endpoint: "http://placeholder:4318/v1/metrics",
					},
					Encoding: otlphttpexporter.EncodingJSON,
				}),
			},
		}
		httpCfg, err := buildHTTPExporterConfig(cfg, "backend:4318")
		require.NoError(t, err)
		require.Equal(t, otlphttpexporter.EncodingJSON, httpCfg.Encoding)
	})
}

func TestHTTPConfigIsolation(t *testing.T) {
	cfg := &Config{
		Protocol: Protocol{
			OTLPHTTP: configoptional.Some(otlphttpexporter.Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://placeholder:4318/v1/metrics",
				},
			}),
		},
	}

	// Build config for first backend
	cfg1, err := buildHTTPExporterConfig(cfg, "backend1:4318")
	require.NoError(t, err)
	require.Equal(t, "http://backend1:4318/v1/metrics", cfg1.ClientConfig.Endpoint)

	// Build config for second backend
	cfg2, err := buildHTTPExporterConfig(cfg, "backend2:4318")
	require.NoError(t, err)
	require.Equal(t, "http://backend2:4318/v1/metrics", cfg2.ClientConfig.Endpoint)

	// Verify they are different pointers
	require.NotSame(t, cfg1, cfg2)

	// Verify original config was not mutated
	originalEndpoint := cfg.Protocol.OTLPHTTP.Get().ClientConfig.Endpoint
	require.Equal(t, "http://placeholder:4318/v1/metrics", originalEndpoint)
}

func TestOTLPDefaultInheritance(t *testing.T) {
	// Test that OTLP configuration builder produces a valid config
	// Note: This test uses configoptional.Some, so manually constructed values are used directly
	// Real default inheritance happens through YAML unmarshaling with configoptional.Default
	cfg := &Config{
		Protocol: Protocol{
			OTLP: configoptional.Some(otlpexporter.Config{
				ClientConfig: configgrpc.ClientConfig{
					Endpoint: "localhost:4317",
				},
			}),
		},
	}

	// Get the config through the builder
	result := buildExporterConfig(cfg, "backend:4317")

	// Verify the endpoint was set correctly
	require.Equal(t, "backend:4317", result.ClientConfig.Endpoint)
}

func TestRetryConfigurationSemantics(t *testing.T) {
	// Test that explicit retry configuration is preserved
	t.Run("explicitly disable retry", func(t *testing.T) {
		cfg := &Config{
			Protocol: Protocol{
				OTLP: configoptional.Some(otlpexporter.Config{
					RetryConfig: configretry.BackOffConfig{
						Enabled: false,
					},
				}),
			},
		}

		result := buildExporterConfig(cfg, "backend:4317")
		// Explicit false should be preserved
		require.False(t, result.RetryConfig.Enabled)
	})
}
