// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lightstepexporter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateTraceExporter(t *testing.T) {
	logger := zap.NewNop()

	factories, err := config.ExampleComponents()
	require.NoError(t, err)
	factory := Factory{}
	factories.Exporters[configmodels.Type(typeStr)] = &factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)

	exporter, err := factory.CreateTraceExporter(logger, cfg.Exporters["lightstep/customname"])
	assert.Nil(t, err)
	assert.NotNil(t, exporter)
}

func TestCreateMetricsExporter(t *testing.T) {
	logger := zap.NewNop()

	factories, err := config.ExampleComponents()
	require.NoError(t, err)
	factory := Factory{}
	factories.Exporters[configmodels.Type(typeStr)] = &factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)

	exporter, err := factory.CreateMetricsExporter(logger, cfg.Exporters["lightstep/customname"])
	assert.NotNil(t, err)
	assert.Nil(t, exporter)
}
