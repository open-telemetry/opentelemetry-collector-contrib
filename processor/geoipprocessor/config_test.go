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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	maxmind "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id                    component.ID
		expected              component.Config
		validateErrorMessage  string
		unmarshalErrorMessage string
	}{
		{
			id:                   component.NewID(metadata.Type),
			validateErrorMessage: "must specify at least one geo IP data provider when using the geoip processor",
		},
		{
			id: component.NewIDWithName(metadata.Type, "maxmind"),
			expected: &Config{
				Context: resource,
				Providers: map[string]provider.Config{
					"maxmind": &maxmind.Config{DatabasePath: "/tmp/db"},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "maxmind_record_context"),
			expected: &Config{
				Context: record,
				Providers: map[string]provider.Config{
					"maxmind": &maxmind.Config{DatabasePath: "/tmp/db"},
				},
			},
		},
		{
			id:                    component.NewIDWithName(metadata.Type, "invalid_providers_config"),
			unmarshalErrorMessage: "unexpected sub-config value kind for key:providers value:this should be a map kind:string",
		},
		{
			id:                    component.NewIDWithName(metadata.Type, "invalid_source"),
			unmarshalErrorMessage: "unknown context not.an.otlp.context, available values: resource, record",
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

			if tt.unmarshalErrorMessage != "" {
				assert.ErrorContains(t, sub.Unmarshal(cfg), tt.unmarshalErrorMessage)
				return
			}
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.validateErrorMessage != "" {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.validateErrorMessage)
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
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalidProviderKey.yaml"), factories)

	require.ErrorContains(t, err, "error reading configuration for \"geoip\": invalid provider key: invalidProviderKey")
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
	collectorConfig, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-mockProvider.yaml"), factories)

	require.NoError(t, err)
	actualDbMockConfig := collectorConfig.Processors[component.NewID(metadata.Type)].(*Config).Providers["mock"].(*dbMockConfig)
	require.Equal(t, "/tmp/geodata.csv", actualDbMockConfig.Database)

	// assert provider unmarshall configuration error by removing the database fieldfrom the configuration struct
	baseMockFactory.CreateDefaultConfigF = func() provider.Config {
		return &providerConfigMock{func() error { return nil }}
	}
	providerFactories["mock"] = &baseMockFactory

	factories.Processors[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-mockProvider.yaml"), factories)

	require.ErrorContains(t, err, "has invalid keys: database")
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
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-mockProvider.yaml"), factories)

	require.ErrorContains(t, err, "error validating provider mock")
}
