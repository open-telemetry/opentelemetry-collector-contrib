// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingprocessor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory

	// we don't need to use them in this test, but the config has them
	factories.Exporters["otlp"] = otlpexporter.NewFactory()
	factories.Exporters["jaeger"] = jaegerexporter.NewFactory()

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	parsed := cfg.Processors[config.NewComponentID(typeStr)]
	assert.Equal(t, parsed,
		&Config{
			ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
			DefaultExporters:  []string{"otlp"},
			FromAttribute:     "X-Tenant",
			Table: []RoutingTableItem{
				{
					Value:     "acme",
					Exporters: []string{"jaeger/acme", "otlp/acme"},
				},
				{
					Value:     "globex",
					Exporters: []string{"otlp/globex"},
				},
			},
		})
}
