// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GRPC.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)

	creationSet := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := factory.CreateTraces(t.Context(), creationSet, cfg, consumertest.NewNop())
	assert.NotNil(t, tReceiver)
	assert.NoError(t, err)

	mReceiver, err := factory.CreateMetrics(t.Context(), creationSet, cfg, consumertest.NewNop())
	assert.NotNil(t, mReceiver)
	assert.NoError(t, err)
}

func TestCreateTraces(t *testing.T) {
	factory := NewFactory()
	defaultGRPCSettings := configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  testutil.GetAvailableLocalAddress(t),
			Transport: confignet.TransportTypeTCP,
		},
	}

	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "default",
			cfg: &Config{
				Protocols: Protocols{
					GRPC: defaultGRPCSettings,
				},
			},
		},
		{
			name: "invalid_grpc_port",
			cfg: &Config{
				Protocols: Protocols{
					GRPC: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:112233",
							Transport: confignet.TransportTypeTCP,
						},
					},
				},
			},
			wantErr: true,
		},
	}
	ctx := t.Context()
	creationSet := receivertest.NewNopSettings(metadata.Type)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.TracesSink)
			tr, err := factory.CreateTraces(ctx, creationSet, tt.cfg, sink)
			assert.NoError(t, err)
			require.NotNil(t, tr)
			if tt.wantErr {
				assert.Error(t, tr.Start(t.Context(), componenttest.NewNopHost()))
				assert.NoError(t, tr.Shutdown(t.Context()))
			} else {
				assert.NoError(t, tr.Start(t.Context(), componenttest.NewNopHost()))
				assert.NoError(t, tr.Shutdown(t.Context()))
			}
		})
	}
}

func TestCreateMetricReceiver(t *testing.T) {
	factory := NewFactory()
	defaultGRPCSettings := configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  testutil.GetAvailableLocalAddress(t),
			Transport: confignet.TransportTypeTCP,
		},
	}

	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "default",
			cfg: &Config{
				Protocols: Protocols{
					GRPC: defaultGRPCSettings,
				},
			},
		},
		{
			name: "invalid_grpc_address",
			cfg: &Config{
				Protocols: Protocols{
					GRPC: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "327.0.0.1:1122",
							Transport: confignet.TransportTypeTCP,
						},
					},
				},
			},
			wantErr: true,
		},
	}
	ctx := t.Context()
	creationSet := receivertest.NewNopSettings(metadata.Type)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			mr, err := factory.CreateMetrics(ctx, creationSet, tt.cfg, sink)
			assert.NoError(t, err)
			require.NotNil(t, mr)
			if tt.wantErr {
				assert.Error(t, mr.Start(t.Context(), componenttest.NewNopHost()))
			} else {
				require.NoError(t, mr.Start(t.Context(), componenttest.NewNopHost()))
				assert.NoError(t, mr.Shutdown(t.Context()))
			}
		})
	}
}

func TestCreateLogReceiver(t *testing.T) {
	factory := NewFactory()
	defaultGRPCSettings := configgrpc.ServerConfig{
		NetAddr: confignet.AddrConfig{
			Endpoint:  testutil.GetAvailableLocalAddress(t),
			Transport: confignet.TransportTypeTCP,
		},
	}

	tests := []struct {
		name         string
		cfg          *Config
		wantStartErr bool
		wantErr      bool
		sink         consumer.Logs
	}{
		{
			name: "default",
			cfg: &Config{
				Protocols: Protocols{
					GRPC: defaultGRPCSettings,
				},
			},
			sink: new(consumertest.LogsSink),
		},
		{
			name: "invalid_grpc_address",
			cfg: &Config{
				Protocols: Protocols{
					GRPC: configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "327.0.0.1:1122",
							Transport: confignet.TransportTypeTCP,
						},
					},
				},
			},
			wantStartErr: true,
			sink:         new(consumertest.LogsSink),
		},
	}
	ctx := t.Context()
	creationSet := receivertest.NewNopSettings(metadata.Type)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, err := factory.CreateLogs(ctx, creationSet, tt.cfg, tt.sink)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.NotNil(t, mr)

			if tt.wantStartErr {
				assert.Error(t, mr.Start(t.Context(), componenttest.NewNopHost()))
				assert.NoError(t, mr.Shutdown(t.Context()))
			} else {
				require.NoError(t, mr.Start(t.Context(), componenttest.NewNopHost()))
				assert.NoError(t, mr.Shutdown(t.Context()))
			}
		})
	}
}
