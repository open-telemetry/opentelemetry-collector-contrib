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
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"

	ddconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Note: the default configuration created by CreateDefaultConfig
	// still has the unresolved environment variables.
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.ExporterSettings{
			TypeVal: config.Type(typeStr),
			NameVal: typeStr,
		},

		API: ddconfig.APIConfig{
			Key:  "$DD_API_KEY",
			Site: "$DD_SITE",
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "$DD_URL",
			},
			DeltaTTL:      3600,
			SendMonotonic: true,
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "$DD_APM_URL",
			},
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "$DD_HOST",
			Env:        "$DD_ENV",
			Service:    "$DD_SERVICE",
			Version:    "$DD_VERSION",
			EnvVarTags: "$DD_TAGS",
		},

		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, cfg, "failed to create default config")

	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters["datadog/api"].(*ddconfig.Config)
	err = apiConfig.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.ExporterSettings{
			NameVal: "datadog/api",
			TypeVal: typeStr,
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "customhostname",
			Env:        "prod",
			Service:    "myservice",
			Version:    "myversion",
			EnvVarTags: "",
			Tags:       []string{"example:tag"},
		},

		API: ddconfig.APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.eu",
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.eu",
			},
			DeltaTTL:      3600,
			SendMonotonic: true,
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.eu",
			},
		},
		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, apiConfig)

	defaultConfig := cfg.Exporters["datadog/default"].(*ddconfig.Config)
	err = defaultConfig.Sanitize()

	require.NoError(t, err)
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.ExporterSettings{
			NameVal: "datadog/default",
			TypeVal: typeStr,
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "",
			Env:        "none",
			Service:    "",
			Version:    "",
			EnvVarTags: "",
		},

		API: ddconfig.APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.com",
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
		},
		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, defaultConfig)

	invalidConfig := cfg.Exporters["datadog/invalid"].(*ddconfig.Config)
	err = invalidConfig.Sanitize()
	require.Error(t, err)
}

// TestLoadConfigEnvVariables tests that the loading configuration takes into account
// environment variables for default values
func TestLoadConfigEnvVariables(t *testing.T) {
	assert.NoError(t, os.Setenv("DD_API_KEY", "replacedapikey"))
	assert.NoError(t, os.Setenv("DD_HOST", "testhost"))
	assert.NoError(t, os.Setenv("DD_ENV", "testenv"))
	assert.NoError(t, os.Setenv("DD_SERVICE", "testservice"))
	assert.NoError(t, os.Setenv("DD_VERSION", "testversion"))
	assert.NoError(t, os.Setenv("DD_SITE", "datadoghq.test"))
	assert.NoError(t, os.Setenv("DD_TAGS", "envexample:tag envexample2:tag"))
	assert.NoError(t, os.Setenv("DD_URL", "https://api.datadoghq.com"))
	assert.NoError(t, os.Setenv("DD_APM_URL", "https://trace.agent.datadoghq.com"))

	defer func() {
		assert.NoError(t, os.Unsetenv("DD_API_KEY"))
		assert.NoError(t, os.Unsetenv("DD_HOST"))
		assert.NoError(t, os.Unsetenv("DD_ENV"))
		assert.NoError(t, os.Unsetenv("DD_SERVICE"))
		assert.NoError(t, os.Unsetenv("DD_VERSION"))
		assert.NoError(t, os.Unsetenv("DD_SITE"))
		assert.NoError(t, os.Unsetenv("DD_TAGS"))
		assert.NoError(t, os.Unsetenv("DD_URL"))
		assert.NoError(t, os.Unsetenv("DD_APM_URL"))
	}()

	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters["datadog/api2"].(*ddconfig.Config)
	err = apiConfig.Sanitize()

	// Check that settings with env variables get overridden when explicitly set in config
	require.NoError(t, err)
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.ExporterSettings{
			NameVal: "datadog/api2",
			TypeVal: typeStr,
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "customhostname",
			Env:        "prod",
			Service:    "myservice",
			Version:    "myversion",
			EnvVarTags: "envexample:tag envexample2:tag",
			Tags:       []string{"example:tag"},
		},

		API: ddconfig.APIConfig{
			Key:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site: "datadoghq.eu",
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.test",
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.test",
			},
		},
		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, apiConfig)

	defaultConfig := cfg.Exporters["datadog/default2"].(*ddconfig.Config)
	err = defaultConfig.Sanitize()

	require.NoError(t, err)

	// Check that settings with env variables get taken into account when
	// no settings are given.
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.ExporterSettings{
			NameVal: "datadog/default2",
			TypeVal: typeStr,
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "testhost",
			Env:        "testenv",
			Service:    "testservice",
			Version:    "testversion",
			EnvVarTags: "envexample:tag envexample2:tag",
		},

		API: ddconfig.APIConfig{
			Key:  "replacedapikey",
			Site: "datadoghq.test",
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
		},

		Traces: ddconfig.TracesConfig{
			SampleRate: 1,
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
		},
		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, defaultConfig)
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	logger := zap.NewNop()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters["datadog/api"]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.SendMetadata = false
	cfg.Exporters["datadog/api"] = c

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/api"],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPITracesExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	logger := zap.NewNop()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters["datadog/api"]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.SendMetadata = false

	ctx := context.Background()
	exp, err := factory.CreateTracesExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/api"],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestOnlyMetadata(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()
	logger := zap.NewNop()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[config.Type(typeStr)] = factory

	ctx := context.Background()
	cfg := &ddconfig.Config{
		API:     ddconfig.APIConfig{Key: "notnull"},
		Metrics: ddconfig.MetricsConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		Traces:  ddconfig.TracesConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},

		SendMetadata:        true,
		OnlyMetadata:        true,
		UseResourceMetadata: true,
	}

	expTraces, err := factory.CreateTracesExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)

	expMetrics, err := factory.CreateMetricsExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.ConsumeTraces(ctx, testutils.TestTraces.Clone())
	require.NoError(t, err)

	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")

}
