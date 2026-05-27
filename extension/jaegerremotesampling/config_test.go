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
	"go.opentelemetry.io/collector/config/configoptional"
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
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:5778",
						Transport: confignet.TransportTypeTCP,
					},
				}),
				GRPCServerConfig: configoptional.None[configgrpc.ServerConfig](),
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
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:5778",
						Transport: confignet.TransportTypeTCP,
					},
				}),
				GRPCServerConfig: configoptional.None[configgrpc.ServerConfig](),
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
	
			assert.True(t, cfg.(*Config).HTTPServerConfig.HasValue())
			assert.Equal(t, "localhost:5778", cfg.(*Config).HTTPServerConfig.Get().NetAddr.Endpoint)
			assert.False(t, cfg.(*Config).GRPCServerConfig.HasValue())
			assert.Equal(t, tt.expected.(*Config).Source, cfg.(*Config).Source)
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
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{}),
			},
			expected: errNoSources,
		},
		{
			desc: "too many sources",
			cfg: Config{
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{}),
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
			name: "valid config with both protocols",
			config: Config{
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				}),
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				}),
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid config with only gRPC",
			config: Config{
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				}),
				Source: Source{
					Remote: &configgrpc.ClientConfig{},
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid config with only HTTP",
			config: Config{
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				}),
				Source: Source{
					File: "/etc/strategies.json",
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
