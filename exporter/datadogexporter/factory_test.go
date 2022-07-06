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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"

	ddconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	t.Setenv("DD_API_KEY", "API_KEY")
	t.Setenv("DD_SITE", "SITE")
	t.Setenv("DD_URL", "URL")
	t.Setenv("DD_APM_URL", "APM_URL")
	t.Setenv("DD_HOST", "HOST")
	t.Setenv("DD_ENV", "ENV")
	t.Setenv("DD_SERVICE", "SERVICE")
	t.Setenv("DD_VERSION", "VERSION")
	t.Setenv("DD_TAGS", "TAGS")

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Note: the default configuration created by CreateDefaultConfig
	// still has the unresolved environment variables.
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		API: ddconfig.APIConfig{
			Key:              "API_KEY",
			Site:             "SITE",
			FailOnInvalidKey: false,
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "URL",
			},
			DeltaTTL:      3600,
			SendMonotonic: true,
			Quantiles:     true,
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: ddconfig.SumConfig{
				CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: ddconfig.SummaryConfig{
				Mode: ddconfig.SummaryModeGauges,
			},
		},

		Traces: ddconfig.TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "APM_URL",
			},
			IgnoreResources: []string{},
		},

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "HOST",
			Env:        "ENV",
			Service:    "SERVICE",
			Version:    "VERSION",
			EnvVarTags: "TAGS",
		},

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
		},

		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, cfg, "failed to create default config")

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

// TestLoadConfig tests that the configuration is loaded correctly
func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")].(*ddconfig.Config)
	assert.Equal(t, config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "api")), apiConfig.ExporterSettings)
	assert.Equal(t, defaulttimeoutSettings(), apiConfig.TimeoutSettings)
	assert.Equal(t, exporterhelper.NewDefaultRetrySettings(), apiConfig.RetrySettings)
	assert.Equal(t, exporterhelper.NewDefaultQueueSettings(), apiConfig.QueueSettings)
	assert.Equal(t, ddconfig.TagsConfig{
		Hostname:   "customhostname",
		Env:        "prod",
		Service:    "myservice",
		Version:    "myversion",
		EnvVarTags: "",
		Tags:       []string{"example:tag"},
	}, apiConfig.TagsConfig)
	assert.Equal(t, ddconfig.APIConfig{
		Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Site:             "datadoghq.eu",
		FailOnInvalidKey: true,
	}, apiConfig.API)
	assert.Equal(t, ddconfig.MetricsConfig{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://api.datadoghq.eu",
		},
		DeltaTTL:      3600,
		SendMonotonic: true,
		Quantiles:     true,
		HistConfig: ddconfig.HistogramConfig{
			Mode:         "distributions",
			SendCountSum: false,
		},
		SumConfig: ddconfig.SumConfig{
			CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
		},
		SummaryConfig: ddconfig.SummaryConfig{
			Mode: ddconfig.SummaryModeGauges,
		},
	}, apiConfig.Metrics)
	assert.Equal(t, ddconfig.TracesConfig{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://trace.agent.datadoghq.eu",
		},
		SpanNameRemappings: map[string]string{
			"old_name1": "new_name1",
			"old_name2": "new_name2",
		},
		SpanNameAsResourceName: true,
		IgnoreResources:        []string{},
	}, apiConfig.Traces)
	assert.True(t, apiConfig.SendMetadata)
	assert.False(t, apiConfig.OnlyMetadata)
	assert.True(t, apiConfig.UseResourceMetadata)

	defaultConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "default")].(*ddconfig.Config)
	assert.Equal(t, &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "default")),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		TagsConfig: ddconfig.TagsConfig{
			Hostname:   "",
			Env:        "none",
			Service:    "",
			Version:    "",
			EnvVarTags: "",
		},

		API: ddconfig.APIConfig{
			Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site:             "datadoghq.com",
			FailOnInvalidKey: false,
		},

		Metrics: ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			SendMonotonic: true,
			DeltaTTL:      3600,
			Quantiles:     true,
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: ddconfig.SumConfig{
				CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: ddconfig.SummaryConfig{
				Mode: ddconfig.SummaryModeGauges,
			},
		},

		Traces: ddconfig.TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
		},

		SendMetadata:        true,
		OnlyMetadata:        false,
		UseResourceMetadata: true,
	}, defaultConfig)

	hostMetadataConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "hostmetadata")].(*ddconfig.Config)

	assert.Equal(t, ddconfig.HostMetadataConfig{
		Enabled:        true,
		HostnameSource: ddconfig.HostnameSourceConfigOrSystem,
		Tags:           []string{"example:one"},
	}, hostMetadataConfig.HostMetadata)
}

// TestLoadConfigEnvVariables tests that the loading configuration takes into account
// environment variables for default values
func TestLoadConfigEnvVariables(t *testing.T) {
	t.Setenv("DD_API_KEY", "replacedapikey")
	t.Setenv("DD_HOST", "testhost")
	t.Setenv("DD_SITE", "datadoghq.test")
	t.Setenv("DD_TAGS", "envexample:tag envexample2:tag")
	t.Setenv("DD_URL", "https://api.datadoghq.com")
	t.Setenv("DD_APM_URL", "https://trace.agent.datadoghq.com")
	t.Setenv("DD_APM_MAX_TPS", "15")

	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	apiConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "api2")].(*ddconfig.Config)

	// Check that settings with env variables get overridden when explicitly set in config
	assert.Equal(t, config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "api2")), apiConfig.ExporterSettings)
	assert.Equal(t, defaulttimeoutSettings(), apiConfig.TimeoutSettings)
	assert.Equal(t, exporterhelper.NewDefaultRetrySettings(), apiConfig.RetrySettings)
	assert.Equal(t, exporterhelper.NewDefaultQueueSettings(), apiConfig.QueueSettings)
	assert.Equal(t, ddconfig.TagsConfig{
		Hostname:   "customhostname",
		Env:        "none",
		EnvVarTags: "envexample:tag envexample2:tag",
	}, apiConfig.TagsConfig)
	assert.Equal(t,
		ddconfig.APIConfig{
			Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site:             "datadoghq.eu",
			FailOnInvalidKey: false,
		}, apiConfig.API)
	assert.Equal(t,
		ddconfig.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.test",
			},
			SendMonotonic: true,
			Quantiles:     false,
			DeltaTTL:      3600,
			HistConfig: ddconfig.HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: ddconfig.SumConfig{
				CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: ddconfig.SummaryConfig{
				Mode: ddconfig.SummaryModeNoQuantiles,
			},
		}, apiConfig.Metrics)
	assert.Equal(t,
		ddconfig.TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.test",
			},
			SpanNameRemappings: map[string]string{
				"old_name3": "new_name3",
				"old_name4": "new_name4",
			},
			IgnoreResources: []string{},
		}, apiConfig.Traces)
	assert.Equal(t,
		ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
			Tags:           []string{"example:tag"},
		}, apiConfig.HostMetadata)

	defaultConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "default")].(*ddconfig.Config)

	// Check that settings with env variables get taken into account when
	// no settings are given.
	assert.Equal(t, config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "default")), defaultConfig.ExporterSettings)
	assert.Equal(t, defaulttimeoutSettings(), defaultConfig.TimeoutSettings)
	assert.Equal(t, exporterhelper.NewDefaultRetrySettings(), defaultConfig.RetrySettings)
	assert.Equal(t, exporterhelper.NewDefaultQueueSettings(), defaultConfig.QueueSettings)
	assert.Equal(t, ddconfig.TagsConfig{
		Hostname:   "testhost",
		Env:        "none",
		EnvVarTags: "envexample:tag envexample2:tag",
	}, defaultConfig.TagsConfig)
	assert.Equal(t, ddconfig.APIConfig{
		Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Site:             "datadoghq.test",
		FailOnInvalidKey: false,
	}, defaultConfig.API)
	assert.Equal(t, ddconfig.MetricsConfig{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://api.datadoghq.com",
		},
		SendMonotonic: true,
		DeltaTTL:      3600,
		Quantiles:     true,
		HistConfig: ddconfig.HistogramConfig{
			Mode:         "distributions",
			SendCountSum: false,
		},
		SumConfig: ddconfig.SumConfig{
			CumulativeMonotonicMode: ddconfig.CumulativeMonotonicSumModeToDelta,
		},
		SummaryConfig: ddconfig.SummaryConfig{
			Mode: ddconfig.SummaryModeGauges,
		},
	}, defaultConfig.Metrics)
	assert.Equal(t, ddconfig.TracesConfig{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://trace.agent.datadoghq.com",
		},
		IgnoreResources: []string{},
	}, defaultConfig.Traces)
}

func TestOverrideEndpoints(t *testing.T) {
	tests := []struct {
		componentID             string
		expectedSite            string
		expectedMetricsEndpoint string
		expectedTracesEndpoint  string
	}{
		{
			componentID:             "nositeandnoendpoints",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
		},
		{
			componentID:             "nositeandmetricsendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.com",
		},
		{
			componentID:             "nositeandtracesendpoint",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "https://api.datadoghq.com",
			expectedTracesEndpoint:  "tracesendpoint:1234",
		},
		{
			componentID:             "nositeandbothendpoints",
			expectedSite:            "datadoghq.com",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "tracesendpoint:1234",
		},

		{
			componentID:             "siteandnoendpoints",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "https://api.datadoghq.eu",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.eu",
		},
		{
			componentID:             "siteandmetricsendpoint",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "https://trace.agent.datadoghq.eu",
		},
		{
			componentID:             "siteandtracesendpoint",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "https://api.datadoghq.eu",
			expectedTracesEndpoint:  "tracesendpoint:1234",
		},
		{
			componentID:             "siteandbothendpoints",
			expectedSite:            "datadoghq.eu",
			expectedMetricsEndpoint: "metricsendpoint:1234",
			expectedTracesEndpoint:  "tracesendpoint:1234",
		},
	}

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "unmarshal.yaml"), factories)
	require.NoError(t, err)

	for _, testInstance := range tests {
		t.Run(testInstance.componentID, func(t *testing.T) {
			rawCfg := cfg.Exporters[config.NewComponentIDWithName(typeStr, testInstance.componentID)]
			componentCfg, ok := rawCfg.(*ddconfig.Config)
			require.True(t, ok, "config.Exporter is not a Datadog exporter config (wrong ID?)")
			assert.Equal(t, testInstance.expectedSite, componentCfg.API.Site)
			assert.Equal(t, testInstance.expectedMetricsEndpoint, componentCfg.Metrics.Endpoint)
			assert.Equal(t, testInstance.expectedTracesEndpoint, componentCfg.Traces.Endpoint)
		})
	}
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPIMetricsExporterFailOnInvalidkey(t *testing.T) {
	server := testutils.DatadogServerMock(testutils.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("fail_on_invalid_key is true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		exp, err := factory.CreateMetricsExporter(
			ctx,
			componenttest.NewNopExporterCreateSettings(),
			cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, exp)
	})
	t.Run("fail_on_invalid_key is false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetricsExporter(
			ctx,
			componenttest.NewNopExporterCreateSettings(),
			cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
		)
		assert.Nil(t, err)
		assert.NotNil(t, exp)
	})
}

func TestCreateAPITracesExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateTracesExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPITracesExporterFailOnInvalidkey(t *testing.T) {
	server := testutils.DatadogServerMock(testutils.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Use the mock server for API key validation
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*ddconfig.Config)
	c.Metrics.TCPAddr.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("fail_on_invalid_key is true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		exp, err := factory.CreateTracesExporter(
			ctx,
			componenttest.NewNopExporterCreateSettings(),
			cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, exp)
	})
	t.Run("fail_on_invalid_key is false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateTracesExporter(
			ctx,
			componenttest.NewNopExporterCreateSettings(),
			cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")],
		)
		assert.Nil(t, err)
		assert.NotNil(t, exp)
	})
}

func TestOnlyMetadata(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	ctx := context.Background()
	cfg := &ddconfig.Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		API:          ddconfig.APIConfig{Key: "notnull"},
		Metrics:      ddconfig.MetricsConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		Traces:       ddconfig.TracesConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		OnlyMetadata: true,

		HostMetadata: ddconfig.HostMetadataConfig{
			Enabled:        true,
			HostnameSource: ddconfig.HostnameSourceFirstResource,
		},
	}

	expTraces, err := factory.CreateTracesExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)

	expMetrics, err := factory.CreateMetricsExporter(
		ctx,
		componenttest.NewNopExporterCreateSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.Start(ctx, nil)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expTraces.Shutdown(ctx))
	}()

	err = expTraces.ConsumeTraces(ctx, testutils.TestTraces.Clone())
	require.NoError(t, err)

	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")

}
