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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		API: APIConfig{
			Site: "datadoghq.com",
		},

		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
		},
		OnlyMetadata: false,
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

	apiConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")].(*Config)
	assert.Equal(t, config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "api")), apiConfig.ExporterSettings)
	assert.Equal(t, defaulttimeoutSettings(), apiConfig.TimeoutSettings)
	assert.Equal(t, exporterhelper.NewDefaultRetrySettings(), apiConfig.RetrySettings)
	assert.Equal(t, exporterhelper.NewDefaultQueueSettings(), apiConfig.QueueSettings)
	assert.Equal(t, TagsConfig{
		Hostname: "customhostname",
	}, apiConfig.TagsConfig)
	assert.Equal(t, APIConfig{
		Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Site:             "datadoghq.eu",
		FailOnInvalidKey: true,
	}, apiConfig.API)
	assert.Equal(t, MetricsConfig{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "https://api.datadoghq.eu",
		},
		DeltaTTL: 3600,
		HistConfig: HistogramConfig{
			Mode:         "distributions",
			SendCountSum: false,
		},
		SumConfig: SumConfig{
			CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
		},
		SummaryConfig: SummaryConfig{
			Mode: SummaryModeGauges,
		},
	}, apiConfig.Metrics)
	assert.Equal(t, TracesConfig{
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
	assert.False(t, apiConfig.OnlyMetadata)

	defaultConfig := cfg.Exporters[config.NewComponentIDWithName(typeStr, "default")].(*Config)
	assert.Equal(t, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "default")),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		API: APIConfig{
			Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site:             "datadoghq.com",
			FailOnInvalidKey: false,
		},

		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.com",
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},

		Traces: TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.com",
			},
			IgnoreResources: []string{},
		},

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
		},

		OnlyMetadata: false,
	}, defaultConfig)

	api2Config := cfg.Exporters[config.NewComponentIDWithName(typeStr, "api2")].(*Config)

	// Check that settings with env variables get overridden when explicitly set in config
	assert.Equal(t, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "api2")),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		TagsConfig: TagsConfig{
			Hostname: "customhostname",
		},
		API: APIConfig{
			Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Site:             "datadoghq.eu",
			FailOnInvalidKey: false,
		},
		Metrics: MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://api.datadoghq.test",
			},
			DeltaTTL: 3600,
			HistConfig: HistogramConfig{
				Mode:         "distributions",
				SendCountSum: false,
			},
			SumConfig: SumConfig{
				CumulativeMonotonicMode: CumulativeMonotonicSumModeToDelta,
			},
			SummaryConfig: SummaryConfig{
				Mode: SummaryModeGauges,
			},
		},
		Traces: TracesConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: "https://trace.agent.datadoghq.test",
			},
			SpanNameRemappings: map[string]string{
				"old_name3": "new_name3",
				"old_name4": "new_name4",
			},
			IgnoreResources: []string{},
		},
		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
			Tags:           []string{"example:tag"},
		},
	}, api2Config)
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
			componentCfg, ok := rawCfg.(*Config)
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
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*Config)
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
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*Config)
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
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*Config)
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
	c := (cfg.Exporters[config.NewComponentIDWithName(typeStr, "api")]).(*Config)
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
	cfg := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  defaulttimeoutSettings(),
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),

		API:          APIConfig{Key: "notnull"},
		Metrics:      MetricsConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		Traces:       TracesConfig{TCPAddr: confignet.TCPAddr{Endpoint: server.URL}},
		OnlyMetadata: true,

		HostMetadata: HostMetadataConfig{
			Enabled:        true,
			HostnameSource: HostnameSourceFirstResource,
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
