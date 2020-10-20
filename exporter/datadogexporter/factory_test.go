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
package datadogexporter

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &config.Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},

		// These are filled when loading using the helper methods
		TagsConfig: config.TagsConfig{
			Hostname: "${DD_HOST}",
			Env:      "${DD_ENV}",
			Service:  "${DD_SERVICE}",
			Version:  "${DD_VERSION}",
		},

		API: config.APIConfig{Site: "datadoghq.com"},
		Traces: config.TracesConfig{
			SampleRate: 1,
		},
	}, cfg, "failed to create default config")

	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters["datadog/api"].(*config.Config)
	err = apiConfig.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, &config.Config{
		ExporterSettings: configmodels.ExporterSettings{
			NameVal: "datadog/api",
			TypeVal: typeStr,
		},

		TagsConfig: config.TagsConfig{
			Hostname: "customhostname",
			Env:      "prod",
			Service:  "myservice",
			Version:  "myversion",
			Tags:     []string{"example:tag"},
		},

		API: config.APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.eu",
		},

		Metrics: config.MetricsConfig{
			Namespace: "opentelemetry.",
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.eu",
			},
		},

		Traces: config.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.eu",
			},
		},
	}, apiConfig)

	invalidConfig2 := cfg.Exporters["datadog/invalid"].(*config.Config)
	err = invalidConfig2.Sanitize()
	require.Error(t, err)

}

func TestCreateAPIMetricsExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	logger := zap.NewNop()

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters["datadog/api"]).(*config.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	cfg.Exporters["datadog/api"] = c

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/api"],
	)

	assert.Nil(t, err)
	assert.NotNil(t, exp)
}
