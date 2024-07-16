// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver

import (
	"context"
	"testing"

	"github.com/open-telemetry/otel-arrow/collector/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
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

	creationSet := receivertest.NewNopSettings()
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.NotNil(t, tReceiver)
	assert.NoError(t, err)

	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), creationSet, cfg, consumertest.NewNop())
	assert.NotNil(t, mReceiver)
	assert.NoError(t, err)
}

func TestCreateTracesReceiver(t *testing.T) {
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
	ctx := context.Background()
	creationSet := receivertest.NewNopSettings()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.TracesSink)
			tr, err := factory.CreateTracesReceiver(ctx, creationSet, tt.cfg, sink)
			assert.NoError(t, err)
			require.NotNil(t, tr)
			if tt.wantErr {
				assert.Error(t, tr.Start(context.Background(), componenttest.NewNopHost()))
				assert.NoError(t, tr.Shutdown(context.Background()))
			} else {
				assert.NoError(t, tr.Start(context.Background(), componenttest.NewNopHost()))
				assert.NoError(t, tr.Shutdown(context.Background()))
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
	ctx := context.Background()
	creationSet := receivertest.NewNopSettings()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			mr, err := factory.CreateMetricsReceiver(ctx, creationSet, tt.cfg, sink)
			assert.NoError(t, err)
			require.NotNil(t, mr)
			if tt.wantErr {
				assert.Error(t, mr.Start(context.Background(), componenttest.NewNopHost()))
			} else {
				require.NoError(t, mr.Start(context.Background(), componenttest.NewNopHost()))
				assert.NoError(t, mr.Shutdown(context.Background()))
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
	ctx := context.Background()
	creationSet := receivertest.NewNopSettings()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, err := factory.CreateLogsReceiver(ctx, creationSet, tt.cfg, tt.sink)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.NotNil(t, mr)

			if tt.wantStartErr {
				assert.Error(t, mr.Start(context.Background(), componenttest.NewNopHost()))
				assert.NoError(t, mr.Shutdown(context.Background()))
			} else {
				require.NoError(t, mr.Start(context.Background(), componenttest.NewNopHost()))
				assert.NoError(t, mr.Shutdown(context.Background()))
			}
		})
	}
}
