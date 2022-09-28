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

package opencensusreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Receiver
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "customname"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				GRPCServerSettings: configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "0.0.0.0:9090",
						Transport: "tcp",
					},
					ReadBufferSize: 512 * 1024,
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "keepalive"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				GRPCServerSettings: configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "0.0.0.0:55678",
						Transport: "tcp",
					},
					ReadBufferSize: 512 * 1024,
					Keepalive: &configgrpc.KeepaliveServerConfig{
						ServerParameters: &configgrpc.KeepaliveServerParameters{
							MaxConnectionIdle:     11 * time.Second,
							MaxConnectionAge:      12 * time.Second,
							MaxConnectionAgeGrace: 13 * time.Second,
							Time:                  30 * time.Second,
							Timeout:               5 * time.Second,
						},
						EnforcementPolicy: &configgrpc.KeepaliveEnforcementPolicy{
							MinTime:             10 * time.Second,
							PermitWithoutStream: true,
						},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "msg-size-conc-connect-max-idle"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				GRPCServerSettings: configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "0.0.0.0:55678",
						Transport: "tcp",
					},
					MaxRecvMsgSizeMiB:    32,
					MaxConcurrentStreams: 16,
					ReadBufferSize:       1024,
					WriteBufferSize:      1024,
					Keepalive: &configgrpc.KeepaliveServerConfig{
						ServerParameters: &configgrpc.KeepaliveServerParameters{
							MaxConnectionIdle: 10 * time.Second,
						},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "tlscredentials"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				GRPCServerSettings: configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "0.0.0.0:55678",
						Transport: "tcp",
					},
					ReadBufferSize: 512 * 1024,
					TLSSetting: &configtls.TLSServerSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "test.crt",
							KeyFile:  "test.key",
						},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "cors"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				GRPCServerSettings: configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "0.0.0.0:55678",
						Transport: "tcp",
					},
					ReadBufferSize: 512 * 1024,
				},
				CorsOrigins: []string{"https://*.test.com", "https://test.com"},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "uds"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				GRPCServerSettings: configgrpc.GRPCServerSettings{
					NetAddr: confignet.NetAddr{
						Endpoint:  "/tmp/opencensus.sock",
						Transport: "unix",
					},
					ReadBufferSize: 512 * 1024,
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
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
