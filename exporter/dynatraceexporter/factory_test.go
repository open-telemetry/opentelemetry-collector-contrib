// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynatraceexporter

import (
	"path"
	"testing"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	dtconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &dtconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: false,
		},

		Tags:              []string{},
		DefaultDimensions: make(map[string]string),
	}, cfg, "failed to create default config")

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	t.Run("defaults", func(t *testing.T) {
		defaultConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "defaults")].(*dtconfig.Config)
		err = defaultConfig.ValidateAndConfigureHTTPClientSettings()
		require.NoError(t, err)
		assert.Equal(t, &dtconfig.Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "defaults")),
			RetrySettings:    exporterhelper.DefaultRetrySettings(),
			QueueSettings:    exporterhelper.DefaultQueueSettings(),

			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: apiconstants.GetDefaultOneAgentEndpoint(),
				Headers: map[string]string{
					"Content-Type": "text/plain; charset=UTF-8",
					"User-Agent":   "opentelemetry-collector"},
			},
			Tags:              []string{},
			DefaultDimensions: make(map[string]string),
		}, defaultConfig)
	})
	t.Run("valid config", func(t *testing.T) {
		validConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "valid")].(*dtconfig.Config)
		err = validConfig.ValidateAndConfigureHTTPClientSettings()

		require.NoError(t, err)
		assert.Equal(t, &dtconfig.Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "valid")),
			RetrySettings:    exporterhelper.DefaultRetrySettings(),
			QueueSettings:    exporterhelper.DefaultQueueSettings(),

			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "http://example.com/api/v2/metrics/ingest",
				Headers: map[string]string{
					"Authorization": "Api-Token token",
					"Content-Type":  "text/plain; charset=UTF-8",
					"User-Agent":    "opentelemetry-collector"},
			},
			APIToken: "token",

			Prefix: "myprefix",

			Tags: []string{},
			DefaultDimensions: map[string]string{
				"dimension_example": "dimension_value",
			},
		}, validConfig)
	})
	t.Run("valid config with tags", func(t *testing.T) {
		validConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "valid_tags")].(*dtconfig.Config)
		err = validConfig.ValidateAndConfigureHTTPClientSettings()

		require.NoError(t, err)
		assert.Equal(t, &dtconfig.Config{
			ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "valid_tags")),
			RetrySettings:    exporterhelper.DefaultRetrySettings(),
			QueueSettings:    exporterhelper.DefaultQueueSettings(),

			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "http://example.com/api/v2/metrics/ingest",
				Headers: map[string]string{
					"Authorization": "Api-Token token",
					"Content-Type":  "text/plain; charset=UTF-8",
					"User-Agent":    "opentelemetry-collector"},
			},
			APIToken: "token",

			Prefix: "myprefix",

			Tags:              []string{"tag_example=tag_value"},
			DefaultDimensions: make(map[string]string),
		}, validConfig)
	})
	t.Run("bad endpoint", func(t *testing.T) {
		badEndpointConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "bad_endpoint")].(*dtconfig.Config)
		err = badEndpointConfig.ValidateAndConfigureHTTPClientSettings()
		require.Error(t, err)
	})

	t.Run("missing api token", func(t *testing.T) {
		missingTokenConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "missing_token")].(*dtconfig.Config)
		err = missingTokenConfig.ValidateAndConfigureHTTPClientSettings()
		require.Error(t, err)
	})
}
