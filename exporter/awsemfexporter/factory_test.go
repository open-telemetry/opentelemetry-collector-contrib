// Copyright 2020, OpenTelemetry Authors
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

package awsemfexporter

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateTracesExporter(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)

	ctx := context.Background()
	exporter, err := factory.CreateTracesExporter(ctx, componenttest.NewNopExporterCreateSettings(), cfg.Exporters[config.NewIDWithName(typeStr, "1")])
	assert.NotNil(t, err)
	assert.Nil(t, exporter)
}

func TestCreateMetricsExporter(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)

	ctx := context.Background()
	exporter, err := factory.CreateMetricsExporter(ctx, componenttest.NewNopExporterCreateSettings(), cfg.Exporters[config.NewIDWithName(typeStr, "1")])
	assert.Nil(t, err)
	assert.NotNil(t, exporter)
}
