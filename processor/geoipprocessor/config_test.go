// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:           component.NewID(metadata.Type),
			errorMessage: "must specify at least one geo IP data provider when using the geoip processor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.errorMessage != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadConfig_InvalidProviderKey(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidateWithSettings(factories, otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: []string{filepath.Join("testdata", "config-invalidProviderKey.yaml")},
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(),
			},
		},
	},
	)

	require.Contains(t, err.Error(), "error reading configuration for \"geoip\": invalid provider key: invalidProviderKey")
}

func TestLoadConfig_ValidProviderKey(t *testing.T) {
	type dbMockConfig struct {
		Database string `mapstructure:"database"`
		providerConfigMock
	}
	baseMockFactory.CreateDefaultConfigF = func() provider.Config {
		return &dbMockConfig{providerConfigMock: providerConfigMock{func() error { return nil }}}
	}
	providerFactories["mock"] = &baseMockFactory

	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[metadata.Type] = factory
	collectorConfig, err := otelcoltest.LoadConfigAndValidateWithSettings(factories, otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: []string{filepath.Join("testdata", "config-mockProvider.yaml")},
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(),
			},
		},
	},
	)

	require.NoError(t, err)
	actualDbMockConfig := collectorConfig.Processors[component.NewID(metadata.Type)].(*Config).Providers["mock"].(*dbMockConfig)
	require.Equal(t, "/tmp/geodata.csv", actualDbMockConfig.Database)
}

func TestLoadConfig_ProviderValidateError(t *testing.T) {
	baseMockFactory.CreateDefaultConfigF = func() provider.Config {
		sampleConfig := struct {
			Database string `mapstructure:"database"`
			providerConfigMock
		}{
			"",
			providerConfigMock{func() error { return errors.New("error validating mocked config") }},
		}
		return &sampleConfig
	}
	providerFactories["mock"] = &baseMockFactory

	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Processors[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidateWithSettings(factories, otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: []string{filepath.Join("testdata", "config-mockProvider.yaml")},
			ProviderFactories: []confmap.ProviderFactory{
				fileprovider.NewFactory(),
			},
		},
	},
	)

	require.Contains(t, err.Error(), "error validating provider mock")
}
