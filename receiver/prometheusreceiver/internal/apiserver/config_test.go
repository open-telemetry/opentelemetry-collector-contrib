// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
)

func TestConfigStruct(t *testing.T) {
	require.NoError(t, componenttest.CheckConfigStruct(Config{}))
}

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, confignet.TransportTypeTCP, cfg.ServerConfig.NetAddr.Transport)
	assert.Equal(t, defaultEndpoint, cfg.ServerConfig.NetAddr.Endpoint)
	assert.Equal(t, defaultReadTimeout, cfg.ServerConfig.ReadTimeout)
	assert.Equal(t, defaultMaxConnections, cfg.MaxConnections)
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		expectErr string
		assertCfg func(*testing.T, Config)
	}{
		{
			name: "valid endpoint",
			cfg: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
						Endpoint:  "127.0.0.1:4317",
					},
				},
			},
		},
		{
			name: "invalid transport",
			cfg: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportType("bogus"),
						Endpoint:  "127.0.0.1:4317",
					},
				},
			},
			expectErr: "invalid transport type",
		},
		{
			name: "missing endpoint uses defaults",
			cfg: Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: confignet.TransportTypeTCP,
					},
				},
			},
			assertCfg: func(t *testing.T, cfg Config) {
				assert.Empty(t, cfg.ServerConfig.NetAddr.Endpoint)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectErr == "" {
				require.NoError(t, err)
				if tt.assertCfg != nil {
					tt.assertCfg(t, tt.cfg)
				}
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)
		})
	}
}
