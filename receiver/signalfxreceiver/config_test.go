// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	allSettingsServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	allSettingsServerConfig.WriteTimeout = 0
	allSettingsServerConfig.ReadHeaderTimeout = 0
	allSettingsServerConfig.IdleTimeout = 0
	allSettingsServerConfig.KeepAlivesEnabled = false
	allSettingsServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  "localhost:9943",
	}

	tlsServerConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	tlsServerConfig.WriteTimeout = 0
	tlsServerConfig.ReadHeaderTimeout = 0
	tlsServerConfig.IdleTimeout = 0
	tlsServerConfig.KeepAlivesEnabled = false
	tlsServerConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  "localhost:9943",
	}
	tlsServerConfig.TLS = configoptional.Some(configtls.ServerConfig{
		Config: configtls.Config{
			CertFile: "/test.crt",
			KeyFile:  "/test.key",
		},
	})

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				ServerConfig: allSettingsServerConfig,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "tls"),
			expected: &Config{
				ServerConfig: tlsServerConfig,
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

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = ""

	err := cfg.Validate()
	assert.EqualError(t, err, "empty endpoint")
}

func TestCreateNoPortEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = "localhost:"

	err := cfg.Validate()
	assert.EqualError(t, err, `endpoint port is not a number: strconv.ParseInt: parsing "": invalid syntax`)
}

func TestCreateLargePortEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = "localhost:65536"

	err := cfg.Validate()
	assert.EqualError(t, err, "port number must be between 1 and 65535")
}
