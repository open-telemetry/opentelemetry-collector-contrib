// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensusreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver/internal/metadata"
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
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "0.0.0.0:9090",
						Transport: confignet.TransportTypeTCP,
					},
					ReadBufferSize: 512 * 1024,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "keepalive"),
			expected: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:55678",
						Transport: confignet.TransportTypeTCP,
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
			id: component.NewIDWithName(metadata.Type, "msg-size-conc-connect-max-idle"),
			expected: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:55678",
						Transport: confignet.TransportTypeTCP,
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
			id: component.NewIDWithName(metadata.Type, "tlscredentials"),
			expected: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:55678",
						Transport: confignet.TransportTypeTCP,
					},
					ReadBufferSize: 512 * 1024,
					TLS: &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "test.crt",
							KeyFile:  "test.key",
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "cors"),
			expected: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "localhost:55678",
						Transport: confignet.TransportTypeTCP,
					},
					ReadBufferSize: 512 * 1024,
				},
				CorsOrigins: []string{"https://*.test.com", "https://test.com"},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "uds"),
			expected: &Config{
				ServerConfig: configgrpc.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint:  "/tmp/opencensus.sock",
						Transport: confignet.TransportTypeUnix,
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
