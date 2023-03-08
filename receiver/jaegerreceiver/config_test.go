// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerreceiver

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
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(typeStr, "customname"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "localhost:9876",
							Transport: "tcp",
						},
					},
					ThriftHTTP: &confighttp.HTTPServerSettings{
						Endpoint: ":3456",
					},
					ThriftCompact: &ProtocolUDP{
						Endpoint: "0.0.0.0:456",
						ServerConfigUDP: ServerConfigUDP{
							QueueSize:        100_000,
							MaxPacketSize:    131_072,
							Workers:          100,
							SocketBufferSize: 65_536,
						},
					},
					ThriftBinary: &ProtocolUDP{
						Endpoint: "0.0.0.0:789",
						ServerConfigUDP: ServerConfigUDP{
							QueueSize:        1_000,
							MaxPacketSize:    65_536,
							Workers:          5,
							SocketBufferSize: 0,
						},
					},
				},
				RemoteSampling: &RemoteSamplingConfig{
					HostEndpoint: "0.0.0.0:5778",
					GRPCClientSettings: configgrpc.GRPCClientSettings{
						Endpoint: "jaeger-collector:1234",
					},
					StrategyFile:               "/etc/strategies.json",
					StrategyFileReloadInterval: time.Second * 10,
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "defaults"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  defaultGRPCBindEndpoint,
							Transport: "tcp",
						},
					},
					ThriftHTTP: &confighttp.HTTPServerSettings{
						Endpoint: defaultHTTPBindEndpoint,
					},
					ThriftCompact: &ProtocolUDP{
						Endpoint:        defaultThriftCompactBindEndpoint,
						ServerConfigUDP: DefaultServerConfigUDP(),
					},
					ThriftBinary: &ProtocolUDP{
						Endpoint:        defaultThriftBinaryBindEndpoint,
						ServerConfigUDP: DefaultServerConfigUDP(),
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "mixed"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "localhost:9876",
							Transport: "tcp",
						},
					},
					ThriftCompact: &ProtocolUDP{
						Endpoint:        defaultThriftCompactBindEndpoint,
						ServerConfigUDP: DefaultServerConfigUDP(),
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "tls"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.GRPCServerSettings{
						NetAddr: confignet.NetAddr{
							Endpoint:  "localhost:9876",
							Transport: "tcp",
						},
						TLSSetting: &configtls.TLSServerSetting{
							TLSSetting: configtls.TLSSetting{
								CertFile: "/test.crt",
								KeyFile:  "/test.key",
							},
						},
					},
					ThriftHTTP: &confighttp.HTTPServerSettings{
						Endpoint: ":3456",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestFailedLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(typeStr, "typo_default_proto_config").String())
	require.NoError(t, err)
	err = component.UnmarshalConfig(sub, cfg)
	assert.EqualError(t, err, "1 error(s) decoding:\n\n* 'protocols' has invalid keys: thrift_htttp")

	sub, err = cm.Sub(component.NewIDWithName(typeStr, "bad_proto_config").String())
	require.NoError(t, err)
	err = component.UnmarshalConfig(sub, cfg)
	assert.EqualError(t, err, "1 error(s) decoding:\n\n* 'protocols' has invalid keys: thrift_htttp")

	sub, err = cm.Sub(component.NewIDWithName(typeStr, "empty").String())
	require.NoError(t, err)
	err = component.UnmarshalConfig(sub, cfg)
	assert.EqualError(t, err, "empty config for Jaeger receiver")
}

func TestInvalidConfig(t *testing.T) {
	testCases := []struct {
		desc  string
		apply func(*Config)
		err   string
	}{
		{
			desc: "thrift-http-no-port",
			apply: func(cfg *Config) {
				cfg.ThriftHTTP = &confighttp.HTTPServerSettings{
					Endpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for Thrift HTTP must fail",
		},
		{
			desc: "thrift-udp-compact-no-port",
			apply: func(cfg *Config) {
				cfg.ThriftCompact = &ProtocolUDP{
					Endpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for Thrift UDP - Compact must fail",
		},
		{
			desc: "thrift-udp-binary-no-port",
			apply: func(cfg *Config) {
				cfg.ThriftBinary = &ProtocolUDP{
					Endpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for Thrift UDP - Binary must fail",
		},
		{
			desc: "remote-sampling-http-no-port",
			apply: func(cfg *Config) {
				cfg.RemoteSampling = &RemoteSamplingConfig{
					HostEndpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for the remote sampling HTTP endpoint must fail",
		},
		{
			desc: "grpc-invalid-host",
			apply: func(cfg *Config) {
				cfg.GRPC = &configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "1234",
						Transport: "tcp",
					},
				}
			},
			err: "receiver creation with bad hostname must fail",
		},
		{
			desc: "no-protocols",
			apply: func(cfg *Config) {
				cfg.Protocols = Protocols{}
			},
			err: "receiver creation with no protocols must fail",
		},
		{
			desc: "port-outside-of-range",
			apply: func(cfg *Config) {
				cfg.ThriftBinary = &ProtocolUDP{
					Endpoint: "localhost:65536",
				}
			},
			err: "receiver creation with too large port number must fail",
		},
		{
			desc: "port-outside-of-range",
			apply: func(cfg *Config) {
				cfg.Protocols = Protocols{}
				cfg.ThriftCompact = &ProtocolUDP{
					Endpoint: defaultThriftCompactBindEndpoint,
				}
				cfg.RemoteSampling = &RemoteSamplingConfig{
					HostEndpoint: "localhost:5778",
					StrategyFile: "strategies.json",
				}
			},
			err: "receiver creation without gRPC and with remote sampling config",
		},
		{
			desc: "reload-interval-outside-of-range",
			apply: func(cfg *Config) {
				cfg.Protocols.GRPC = &configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "1234",
						Transport: "tcp",
					},
				}
				cfg.RemoteSampling = &RemoteSamplingConfig{
					HostEndpoint:               "localhost:5778",
					StrategyFile:               "strategies.json",
					StrategyFileReloadInterval: -time.Second,
				}
			},
			err: "strategy file reload interval should be great zero",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)

			tC.apply(cfg)

			err := component.ValidateConfig(cfg)
			assert.Error(t, err, tC.err)

		})
	}
}
