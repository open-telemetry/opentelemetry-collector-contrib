// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(confmap.New(), cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
}

func TestUnmarshalConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(cm, cfg))
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
			},
		}, cfg)
}

func TestUnmarshalHttpConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_http.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(cm, cfg))
	assert.Equal(t,
		&Config{
			Server: &OpAMPServer{
				HTTP: &commonFields{
					Endpoint: "https://127.0.0.1:4320/v1/opamp",
				},
			},
			InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			Capabilities: Capabilities{
				ReportsEffectiveConfig: true,
			},
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
						TLSSetting: configtls.ClientConfig{
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
					HTTP: &commonFields{
						Endpoint: "https://127.0.0.1:4320/v1/opamp",
						Headers: map[string]configopaque.String{
							"test": configopaque.String("test"),
						},
						TLSSetting: configtls.ClientConfig{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.expected.headers(t, tt.fields.Server.GetHeaders())
			tt.expected.tls(t, tt.fields.Server.GetTLSSetting())
			tt.expected.endpoint(t, tt.fields.Server.GetEndpoint())
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
					HTTP: &commonFields{},
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
					HTTP: &commonFields{
						Endpoint: "https://127.0.0.1:4320/v1/opamp",
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
					HTTP: &commonFields{
						Endpoint: "https://127.0.0.1:4320/v1/opamp",
					},
				},
				InstanceUID: "01BX5ZZKBKACTAV9WEVGEMMVRZ",
			},
			wantErr: assert.NoError,
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
					HTTP: &commonFields{},
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
