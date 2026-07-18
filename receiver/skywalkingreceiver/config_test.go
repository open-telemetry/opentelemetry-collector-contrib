// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	defaultServerConfig.WriteTimeout = 0
	defaultServerConfig.ReadHeaderTimeout = 0
	defaultServerConfig.IdleTimeout = 0
	defaultServerConfig.KeepAlivesEnabled = false
	defaultServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "0.0.0.0:12800",
	}

	customServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	customServerConfig.WriteTimeout = 0
	customServerConfig.ReadHeaderTimeout = 0
	customServerConfig.IdleTimeout = 0
	customServerConfig.KeepAlivesEnabled = false
	customServerConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "0.0.0.0:12801",
	}

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Protocols: Protocols{
					HTTP: &defaultServerConfig,
					GRPC: &configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "0.0.0.0:11800",
							Transport: confignet.TransportTypeTCP,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				Protocols: Protocols{
					HTTP: &customServerConfig,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "defaults"),
			expected: &Config{
				Protocols: Protocols{
					GRPC: &configgrpc.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint:  "localhost:11800",
							Transport: confignet.TransportTypeTCP,
						},
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
