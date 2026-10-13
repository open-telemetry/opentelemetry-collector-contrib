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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

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
		{
			id: component.NewIDWithName(metadata.Type, "with_grpc_default"),
			expected: &Config{
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:5778",
						Transport: confignet.TransportTypeTCP,
					},
				}),
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:14250",
						Transport: confignet.TransportTypeTCP,
					},
				}),
				Source: Source{
					File: "/etc/otelcol/sampling_strategies.json",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_grpc_custom"),
			expected: &Config{
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:5778",
						Transport: confignet.TransportTypeTCP,
					},
				}),
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "0.0.0.0:14260",
						Transport: confignet.TransportTypeTCP,
					},
				}),
				Source: Source{
					File: "/etc/otelcol/sampling_strategies.json",
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
			assert.NoError(t, confmap.Validate(cfg))

			actualCfg := cfg.(*Config)
			expectedCfg := tt.expected.(*Config)

			assert.Equal(t, expectedCfg.HTTPServerConfig.HasValue(), actualCfg.HTTPServerConfig.HasValue())
			require.True(t, actualCfg.HTTPServerConfig.HasValue())
			assert.Equal(t, expectedCfg.HTTPServerConfig.Get().NetAddr, actualCfg.HTTPServerConfig.Get().NetAddr)

			assert.Equal(t, expectedCfg.GRPCServerConfig.HasValue(), actualCfg.GRPCServerConfig.HasValue())
			if expectedCfg.GRPCServerConfig.HasValue() {
				require.True(t, actualCfg.GRPCServerConfig.HasValue())
				assert.Equal(t, expectedCfg.GRPCServerConfig.Get().NetAddr, actualCfg.GRPCServerConfig.Get().NetAddr)
			}
			assert.Equal(t, expectedCfg.Source, actualCfg.Source)
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
		{
			desc: "valid config with both protocols",
			cfg: Config{
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
			expected: nil,
		},
		{
			desc: "valid config with only gRPC",
			cfg: Config{
				GRPCServerConfig: configoptional.Some(configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:14250",
					},
				}),
				Source: Source{
					Remote: &configgrpc.ClientConfig{},
				},
			},
			expected: nil,
		},
		{
			desc: "valid config with only HTTP",
			cfg: Config{
				HTTPServerConfig: configoptional.Some(confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "0.0.0.0:5778",
					},
				}),
				Source: Source{
					File: "/etc/strategies.json",
				},
			},
			expected: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			res := tC.cfg.Validate()
			assert.ErrorIs(t, res, tC.expected)
		})
	}
}
