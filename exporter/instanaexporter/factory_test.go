// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instanaexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/metadata"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint:        "",
			Timeout:         30 * time.Second,
			Headers:         map[string]configopaque.String{},
			WriteBufferSize: 512 * 1024,
		},
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yml"))
	require.NoError(t, err)
	factory := NewFactory()

	t.Run("valid config", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "valid").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		err = component.ValidateConfig(cfg)

		require.NoError(t, err)
		assert.Equal(t, &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint:        "https://example.com/api/",
				Timeout:         30 * time.Second,
				Headers:         map[string]configopaque.String{},
				WriteBufferSize: 512 * 1024,
			},
			Endpoint: "https://example.com/api/",
			AgentKey: "key1",
		}, cfg)
	})

	t.Run("valid config with ca_file", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "valid_with_ca_file").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		err = component.ValidateConfig(cfg)

		require.NoError(t, err)
		assert.Equal(t, &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint:        "https://example.com/api/",
				Timeout:         30 * time.Second,
				Headers:         map[string]configopaque.String{},
				WriteBufferSize: 512 * 1024,
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile: "ca.crt",
					},
				},
			},
			Endpoint: "https://example.com/api/",
			AgentKey: "key1",
		}, cfg)
	})

	t.Run("valid config without ca_file", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "valid_no_ca_file").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		err = component.ValidateConfig(cfg)

		require.NoError(t, err)
		assert.Equal(t, &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint:        "https://example.com/api/",
				Timeout:         30 * time.Second,
				Headers:         map[string]configopaque.String{},
				WriteBufferSize: 512 * 1024,
			},
			Endpoint: "https://example.com/api/",
			AgentKey: "key1",
		}, cfg)
	})

	t.Run("bad endpoint", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "bad_endpoint").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		err = component.ValidateConfig(cfg)
		require.Error(t, err)
	})

	t.Run("non https endpoint", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "non_https_endpoint").String())

		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		err = component.ValidateConfig(cfg)
		require.Error(t, err)
	})

	t.Run("missing agent key", func(t *testing.T) {
		cfg := factory.CreateDefaultConfig()
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "missing_agent_key").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		err = component.ValidateConfig(cfg)
		require.Error(t, err)
	})
}
