// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerremotesampling

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:5778",
						Transport: confignet.TransportTypeTCP,
					},
				},
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:14250",
						Transport: confignet.TransportTypeTCP,
					},
				},
				Source: Source{
					Remote: &configgrpc.ClientConfig{
						Endpoint: "jaeger-collector:14250",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:5778",
						Transport: confignet.TransportTypeTCP,
					},
				},
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:14250",
						Transport: confignet.TransportTypeTCP,
					},
				},
				Source: Source{
					ReloadInterval: time.Second,
					File:           "/etc/otelcol/sampling_strategies.json",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc     string
		cfg      Config
		expected error
	}{
		{
			desc:     "no receiving protocols",
			cfg:      Config{},
			expected: errAtLeastOneProtocol,
		},
		{
			desc: "no sources",
			cfg: Config{
				GRPCServerConfig: &configgrpc.ServerConfig{},
			},
			expected: errNoSources,
		},
		{
			desc: "too many sources",
			cfg: Config{
				GRPCServerConfig: &configgrpc.ServerConfig{},
				Source: Source{
					Remote: &configgrpc.ClientConfig{},
					File:   "/tmp/some-file",
				},
			},
			expected: errTooManySources,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := tC.cfg.Validate()
			assert.Equal(t, tC.expected, res)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name: "valid config with both protocols on different ports",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				},
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				},
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid config with only HTTP",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				},
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid config with only gRPC",
			config: Config{
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				},
				Source: Source{
					Remote: &configgrpc.ClientConfig{},
				},
			},
			expectedErr: nil,
		},
		{
			name: "invalid: no protocols configured",
			config: Config{
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: errAtLeastOneProtocol,
		},
		{
			name: "invalid: same port for HTTP and gRPC",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				},
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				},
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: errSamePortConflict,
		},
		{
			name: "invalid: same port with different host formats",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "localhost:8080",
					},
				},
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:8080",
					},
				},
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: errSamePortConflict,
		},
		{
			name: "invalid: same port with IPv6 format",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "[::1]:9090",
					},
				},
				GRPCServerConfig: &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:9090",
					},
				},
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: errSamePortConflict,
		},
		{
			name: "invalid: too many sources",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				},
				Source: Source{
					File:   "/etc/strategies.json",
					Remote: &configgrpc.ClientConfig{},
				},
			},
			expectedErr: errTooManySources,
		},
		{
			name: "invalid: no sources",
			config: Config{
				HTTPServerConfig: &confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				},
				Source: Source{},
			},
			expectedErr: errNoSources,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractPort(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		expectedPort string
		expectError  bool
	}{
		{
			name:         "standard format with host and port",
			endpoint:     "0.0.0.0:8080",
			expectedPort: "8080",
			expectError:  false,
		},
		{
			name:         "port only format",
			endpoint:     ":8080",
			expectedPort: "8080",
			expectError:  false,
		},
		{
			name:         "localhost with port",
			endpoint:     "localhost:9090",
			expectedPort: "9090",
			expectError:  false,
		},
		{
			name:         "IPv6 format",
			endpoint:     "[::1]:8080",
			expectedPort: "8080",
			expectError:  false,
		},
		{
			name:         "IPv6 with zone",
			endpoint:     "[fe80::1%lo0]:8080",
			expectedPort: "8080",
			expectError:  false,
		},
		{
			name:        "empty endpoint",
			endpoint:    "",
			expectError: true,
		},
		{
			name:        "missing port",
			endpoint:    "localhost",
			expectError: true,
		},
		{
			name:        "invalid format",
			endpoint:    "not:a:valid:endpoint:8080",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := extractPort(tt.endpoint)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPort, port)
			}
		})
	}
}
