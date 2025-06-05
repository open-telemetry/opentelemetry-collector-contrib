// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver/internal/metadata"
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
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:9876",
							Transport: confignet.TransportTypeTCP,
						},
					},
					ThriftHTTP: &confighttp.ServerConfig{
						Endpoint: ":3456",
					},
					ThriftCompactUDP: &ProtocolUDP{
						Endpoint: "0.0.0.0:456",
						ServerConfigUDP: ServerConfigUDP{
							QueueSize:        100_000,
							MaxPacketSize:    131_072,
							Workers:          100,
							SocketBufferSize: 65_536,
						},
					},
					ThriftBinaryUDP: &ProtocolUDP{
						Endpoint: "0.0.0.0:789",
						ServerConfigUDP: ServerConfigUDP{
							QueueSize:        1_000,
							MaxPacketSize:    65_536,
							Workers:          5,
							SocketBufferSize: 0,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "defaults"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:14250",
							Transport: confignet.TransportTypeTCP,
						},
					},
					ThriftHTTP: &confighttp.ServerConfig{
						Endpoint: "localhost:14268",
					},
					ThriftCompactUDP: &ProtocolUDP{
						Endpoint:        "localhost:6831",
						ServerConfigUDP: defaultServerConfigUDP(),
					},
					ThriftBinaryUDP: &ProtocolUDP{
						Endpoint:        "localhost:6832",
						ServerConfigUDP: defaultServerConfigUDP(),
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "mixed"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:9876",
							Transport: confignet.TransportTypeTCP,
						},
					},
					ThriftCompactUDP: &ProtocolUDP{
						Endpoint:        "localhost:6831",
						ServerConfigUDP: defaultServerConfigUDP(),
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "tls"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:9876",
							Transport: confignet.TransportTypeTCP,
						},
						TLS: &configtls.ServerConfig{
							Config: configtls.Config{
								CertFile: "/test.crt",
								KeyFile:  "/test.key",
							},
						},
					},
					ThriftHTTP: &confighttp.ServerConfig{
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestFailedLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "typo_default_proto_config").String())
	require.NoError(t, err)
	err = sub.Unmarshal(cfg)
	assert.ErrorContains(t, err, "'protocols' has invalid keys: thrift_htttp")

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "bad_proto_config").String())
	require.NoError(t, err)
	err = sub.Unmarshal(cfg)
	assert.ErrorContains(t, err, "'protocols' has invalid keys: thrift_htttp")

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "empty").String())
	require.NoError(t, err)
	err = sub.Unmarshal(cfg)
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
				cfg.ThriftHTTP = &confighttp.ServerConfig{
					Endpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for Thrift HTTP must fail",
		},
		{
			desc: "thrift-udp-compact-no-port",
			apply: func(cfg *Config) {
				cfg.ThriftCompactUDP = &ProtocolUDP{
					Endpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for Thrift UDP - Compact must fail",
		},
		{
			desc: "thrift-udp-binary-no-port",
			apply: func(cfg *Config) {
				cfg.ThriftBinaryUDP = &ProtocolUDP{
					Endpoint: "localhost:",
				}
			},
			err: "receiver creation with no port number for Thrift UDP - Binary must fail",
		},
		{
			desc: "grpc-invalid-host",
			apply: func(cfg *Config) {
				cfg.GRPC = &configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "1234",
						Transport: confignet.TransportTypeTCP,
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
				cfg.ThriftBinaryUDP = &ProtocolUDP{
					Endpoint: "localhost:65536",
				}
			},
			err: "receiver creation with too large port number must fail",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)

			tC.apply(cfg)

			err := xconfmap.Validate(cfg)
			assert.Error(t, err, tC.err)
		})
	}
}
