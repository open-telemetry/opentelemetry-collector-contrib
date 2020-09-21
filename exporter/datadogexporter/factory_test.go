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
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		API: APIConfig{Site: "datadoghq.com"},
		Metrics: MetricsConfig{
			Mode:        DogStatsDMode,
			Percentiles: true,
			DogStatsD: DogStatsDConfig{
				Endpoint:  "127.0.0.1:8125",
				Telemetry: true,
			},
		},
	}, cfg, "failed to create default config")

	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateAgentMetricsExporter(t *testing.T) {
	logger := zap.NewNop()

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/dogstatsd"],
	)
	assert.Nil(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	logger := zap.NewNop()

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/api"],
	)

	// Not implemented
	assert.NotNil(t, err)
	assert.Nil(t, exp)
}

func TestCreateInvalidMetricsExporter(t *testing.T) {
	logger := zap.NewNop()

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ctx := context.Background()
	exp, err := factory.CreateMetricsExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/dogstatsd/invalid"],
	)

	// The address is invalid
	assert.NotNil(t, err)
	assert.Nil(t, exp)
}

func TestCreateAPITraceExporter(t *testing.T) {
	logger := zap.NewNop()

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	ctx := context.Background()
	exp, err := factory.CreateTraceExporter(
		ctx,
		component.ExporterCreateParams{Logger: logger},
		cfg.Exporters["datadog/api"],
	)

	// Not implemented
	assert.NotNil(t, err)
	assert.Nil(t, exp)
}
