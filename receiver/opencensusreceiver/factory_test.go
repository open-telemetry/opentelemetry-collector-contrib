// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensusreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.NetAddr.Endpoint = testutil.GetAvailableLocalAddress(t)

	set := receivertest.NewNopSettings(metadata.Type)
	tReceiver, err := createTracesReceiver(context.Background(), set, cfg, nil)
	assert.NotNil(t, tReceiver)
	assert.NoError(t, err)

	mReceiver, err := createMetricsReceiver(context.Background(), set, cfg, nil)
	assert.NotNil(t, mReceiver)
	assert.NoError(t, err)
}

func TestCreateTraces(t *testing.T) {
	defaultNetAddr := confignet.AddrConfig{
		Endpoint:  testutil.GetAvailableLocalAddress(t),
		Transport: confignet.TransportTypeTCP,
	}
	defaultGRPCSettings := configgrpc.ServerConfig{
		NetAddr: defaultNetAddr,
	}
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "default",
			cfg: &Config{
				ServerConfig: defaultGRPCSettings,
			},
		},
		{
			name: "invalid_port",
			cfg: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:112233",
						Transport: confignet.TransportTypeTCP,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "max-msg-size-and-concurrent-connections",
			cfg: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr:              defaultNetAddr,
					MaxRecvMsgSizeMiB:    32,
					MaxConcurrentStreams: 16,
				},
			},
		},
	}
	ctx := context.Background()
	set := receivertest.NewNopSettings(metadata.Type)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := createTracesReceiver(ctx, set, tt.cfg, consumertest.NewNop())
			require.NoError(t, err)
			err = tr.Start(context.Background(), componenttest.NewNopHost())
			if (err != nil) != tt.wantErr {
				t.Errorf("factory.CreateTraces() error = %v, wantErr %v", err, tt.wantErr)
			}
			require.NoError(t, tr.Shutdown(context.Background()))
		})
	}
}

func TestCreateMetrics(t *testing.T) {
	defaultNetAddr := confignet.AddrConfig{
		Endpoint:  testutil.GetAvailableLocalAddress(t),
		Transport: confignet.TransportTypeTCP,
	}
	defaultGRPCSettings := configgrpc.ServerConfig{
		NetAddr: defaultNetAddr,
	}

	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "default",
			cfg: &Config{
				ServerConfig: defaultGRPCSettings,
			},
		},
		{
			name: "invalid_address",
			cfg: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "327.0.0.1:1122",
						Transport: confignet.TransportTypeTCP,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "keepalive",
			cfg: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: defaultNetAddr,
					Keepalive: &configgrpc.KeepaliveServerConfig{
						ServerParameters: &configgrpc.KeepaliveServerParameters{
							MaxConnectionAge: 60 * time.Second,
						},
						EnforcementPolicy: &configgrpc.KeepaliveEnforcementPolicy{
							MinTime:             30 * time.Second,
							PermitWithoutStream: true,
						},
					},
				},
			},
		},
	}
	set := receivertest.NewNopSettings(metadata.Type)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := createMetricsReceiver(context.Background(), set, tt.cfg, consumertest.NewNop())
			require.NoError(t, err)
			err = tc.Start(context.Background(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, tc.Shutdown(context.Background()))
			}()
			if (err != nil) != tt.wantErr {
				t.Errorf("factory.CreateMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
