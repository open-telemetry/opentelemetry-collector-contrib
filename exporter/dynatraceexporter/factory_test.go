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
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	dtconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &dtconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		RetrySettings:    exporterhelper.DefaultRetrySettings(),
		QueueSettings:    exporterhelper.DefaultQueueSettings(),
		ResourceToTelemetrySettings: exporterhelper.ResourceToTelemetrySettings{
			Enabled: false,
		},

		Tags: []string{},
	}, cfg, "failed to create default config")

	assert.NoError(t, configcheck.ValidateConfig(cfg))
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

	apiConfig := cfg.Exporters[config.NewIDWithName(typeStr, "valid")].(*dtconfig.Config)
	err = apiConfig.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, &dtconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewIDWithName(typeStr, "valid")),
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

		Tags: []string{"example=tag"},
	}, apiConfig)

	invalidConfig2 := cfg.Exporters[config.NewIDWithName(typeStr, "invalid")].(*dtconfig.Config)
	err = invalidConfig2.Sanitize()
	require.Error(t, err)
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg.Exporters[config.NewIDWithName(typeStr, "valid")],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPIMetricsExporterInvalid(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg.Exporters[config.NewIDWithName(typeStr, "invalid")],
	)

	assert.Error(t, err)
	assert.Nil(t, exp)
}
