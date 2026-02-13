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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, confignet.TransportTypeTCP, cfg.ServerConfig.NetAddr.Transport)
	assert.Equal(t, defaultEndpoint, cfg.ServerConfig.NetAddr.Endpoint)
}

func TestConfigApplyDefaultsSetsValues(t *testing.T) {
	var cfg Config

	cfg.ApplyDefaults()

	assert.Equal(t, confignet.TransportTypeTCP, cfg.ServerConfig.NetAddr.Transport)
	assert.Equal(t, defaultEndpoint, cfg.ServerConfig.NetAddr.Endpoint)
}

func TestConfigApplyDefaultsPreservesExistingValues(t *testing.T) {
	cfg := Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Transport: confignet.TransportTypeUnix,
				Endpoint:  "/tmp/prometheus.sock",
			},
		},
	}

	cfg.ApplyDefaults()

	assert.Equal(t, confignet.TransportTypeUnix, cfg.ServerConfig.NetAddr.Transport)
	assert.Equal(t, "/tmp/prometheus.sock", cfg.ServerConfig.NetAddr.Endpoint)
}

func TestConfigValidate(t *testing.T) {
	t.Run("valid endpoint", func(t *testing.T) {
		cfg := Config{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: confignet.TransportTypeTCP,
					Endpoint:  "127.0.0.1:4317",
				},
			},
		}

		require.NoError(t, cfg.Validate())
	})

	t.Run("invalid transport", func(t *testing.T) {
		cfg := Config{
			ServerConfig: confighttp.ServerConfig{
				NetAddr: confignet.AddrConfig{
					Transport: confignet.TransportType("bogus"),
					Endpoint:  "127.0.0.1:4317",
				},
			},
		}

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid transport type")
	})
}
