// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, confmap.New().Unmarshal(cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}

func TestUnmarshalConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t,
		&Config{
			Server: &OpAMPServer{
				WS: &commonFields{
					Endpoint: "wss://127.0.0.1:4320/v1/opamp",
				},
			},
			InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			Capabilities: Capabilities{
				ReportsEffectiveConfig: true,
				ReportsHealth:          true,
			},
			PPIDPollInterval: 5 * time.Second,
		}, cfg)
}

func TestUnmarshalHttpConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_http.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t,
		&Config{
			Server: &OpAMPServer{
				HTTP: &httpFields{
					commonFields: commonFields{
						Endpoint: "https://127.0.0.1:4320/v1/opamp",
					},
					PollingInterval: 1 * time.Minute,
				},
			},
			InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			Capabilities: Capabilities{
				ReportsEffectiveConfig: true,
				ReportsHealth:          true,
			},
			PPIDPollInterval: 5 * time.Second,
		}, cfg)
}

func TestConfig_Getters(t *testing.T) {
	type fields struct {
		Server *OpAMPServer
	}
	type expected struct {
		headers  assert.ValueAssertionFunc
		tls      assert.ValueAssertionFunc
		endpoint assert.ValueAssertionFunc
	}
	tests := []struct {
		name     string
		fields   fields
		expected expected
	}{
		{
			name: "nothing set",
			fields: fields{
				Server: &OpAMPServer{},
			},
			expected: expected{
				headers:  assert.Empty,
				tls:      assert.Empty,
				endpoint: assert.Empty,
			},
		},
		{
			name: "WS valid endpoint, headers, tls",
			fields: fields{
				Server: &OpAMPServer{
					WS: &commonFields{
						Endpoint: "wss://127.0.0.1:4320/v1/opamp",
						Headers: map[string]configopaque.String{
							"test": configopaque.String("test"),
						},
						TLS: configtls.ClientConfig{
							Insecure: true,
						},
					},
				},
			},
			expected: expected{
				headers:  assert.NotEmpty,
				tls:      assert.NotEmpty,
				endpoint: assert.NotEmpty,
			},
		},
		{
			name: "HTTP valid endpoint and valid instance id",
			fields: fields{
				Server: &OpAMPServer{
					HTTP: &httpFields{
						commonFields: commonFields{
							Endpoint: "https://127.0.0.1:4320/v1/opamp",
							Headers: map[string]configopaque.String{
								"test": configopaque.String("test"),
							},
							TLS: configtls.ClientConfig{
								Insecure: true,
							},
						},
					},
				},
			},
			expected: expected{
				headers:  assert.NotEmpty,
				tls:      assert.NotEmpty,
				endpoint: assert.NotEmpty,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expected.headers(t, tt.fields.Server.GetHeaders())
			tt.expected.tls(t, tt.fields.Server.getTLS())
			tt.expected.endpoint(t, tt.fields.Server.GetEndpoint())
		})
	}
}

func TestOpAMPServer_GetTLSConfig(t *testing.T) {
	tests := []struct {
		name              string
		server            OpAMPServer
		expectedTLSConfig assert.ValueAssertionFunc
	}{
		{
			name: "wss endpoint",
			server: OpAMPServer{
				WS: &commonFields{
					Endpoint: "wss://example.com",
					TLS:      configtls.NewDefaultClientConfig(),
				},
			},
			expectedTLSConfig: assert.NotNil,
		},
		{
			name: "https endpoint",
			server: OpAMPServer{
				HTTP: &httpFields{
					commonFields: commonFields{
						Endpoint: "https://example.com",
						TLS:      configtls.NewDefaultClientConfig(),
					},
				},
			},
			expectedTLSConfig: assert.NotNil,
		},
		{
			name: "ws endpoint",
			server: OpAMPServer{
				WS: &commonFields{
					Endpoint: "ws://example.com",
				},
			},
			expectedTLSConfig: assert.Nil,
		},
		{
			name: "http endpoint",
			server: OpAMPServer{
				HTTP: &httpFields{
					commonFields: commonFields{
						Endpoint: "http://example.com",
					},
				},
			},
			expectedTLSConfig: assert.Nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tlsConfig, err := tt.server.GetTLSConfig(ctx)
			assert.NoError(t, err)
			tt.expectedTLSConfig(t, tlsConfig)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	type fields struct {
		Server       *OpAMPServer
		InstanceUID  string
		Capabilities Capabilities
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "WS must have endpoint",
			fields: fields{
				Server: &OpAMPServer{
					WS: &commonFields{},
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "opamp server endpoint must be provided", err.Error())
			},
		},
		{
			name: "WS valid endpoint and invalid instance id",
			fields: fields{
				Server: &OpAMPServer{
					WS: &commonFields{
						Endpoint: "wss://127.0.0.1:4320/v1/opamp",
					},
				},
				InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZFAIL",
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "opamp instance_uid is invalid", err.Error())
			},
		},
		{
			name: "WS valid endpoint and valid instance id",
			fields: fields{
				Server: &OpAMPServer{
					WS: &commonFields{
						Endpoint: "wss://127.0.0.1:4320/v1/opamp",
					},
				},
				InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			},
			wantErr: assert.NoError,
		},
		{
			name: "HTTP must have endpoint",
			fields: fields{
				Server: &OpAMPServer{
					HTTP: &httpFields{},
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "opamp server endpoint must be provided", err.Error())
			},
		},
		{
			name: "HTTP valid endpoint and invalid instance id",
			fields: fields{
				Server: &OpAMPServer{
					HTTP: &httpFields{
						commonFields: commonFields{
							Endpoint: "https://127.0.0.1:4320/v1/opamp",
						},
					},
				},
				InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZFAIL",
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "opamp instance_uid is invalid", err.Error())
			},
		},
		{
			name: "HTTP valid endpoint and valid instance id",
			fields: fields{
				Server: &OpAMPServer{
					HTTP: &httpFields{
						commonFields: commonFields{
							Endpoint: "https://127.0.0.1:4320/v1/opamp",
						},
					},
				},
				InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			},
			wantErr: assert.NoError,
		},
		{
			name: "HTTP invalid polling interval",
			fields: fields{
				Server: &OpAMPServer{
					HTTP: &httpFields{
						commonFields: commonFields{
							Endpoint: "https://127.0.0.1:4320/v1/opamp",
						},
						PollingInterval: -1,
					},
				},
				InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "polling interval must be 0 or greater", err.Error())
			},
		},
		{
			name: "neither config set",
			fields: fields{
				Server: &OpAMPServer{},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "opamp server must have at least ws or http set", err.Error())
			},
		},
		{
			name: "both config set",
			fields: fields{
				Server: &OpAMPServer{
					WS:   &commonFields{},
					HTTP: &httpFields{},
				},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Equal(t, "opamp server must have only ws or http set", err.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server:       tt.fields.Server,
				InstanceUID:  tt.fields.InstanceUID,
				Capabilities: tt.fields.Capabilities,
			}
			tt.wantErr(t, cfg.Validate())
		})
	}
}

func TestCapabilities_toAgentCapabilities(t *testing.T) {
	type fields struct {
		ReportsEffectiveConfig     bool
		ReportsHealth              bool
		ReportsAvailableComponents bool
	}
	tests := []struct {
		name   string
		fields fields
		want   protobufs.AgentCapabilities
	}{
		{
			name: "default capabilities",
			fields: fields{
				ReportsEffectiveConfig:     false,
				ReportsHealth:              false,
				ReportsAvailableComponents: false,
			},
			want: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus,
		},
		{
			name: "all supported capabilities enabled",
			fields: fields{
				ReportsEffectiveConfig:     true,
				ReportsHealth:              true,
				ReportsAvailableComponents: true,
			},
			want: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus | protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig | protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth | protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := Capabilities{
				ReportsEffectiveConfig:     tt.fields.ReportsEffectiveConfig,
				ReportsHealth:              tt.fields.ReportsHealth,
				ReportsAvailableComponents: tt.fields.ReportsEffectiveConfig,
			}
			assert.Equalf(t, tt.want, caps.toAgentCapabilities(), "toAgentCapabilities()")
		})
	}
}
