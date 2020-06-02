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

package newrelicexporter

import (
	"path"
	"testing"
	"time"

	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters["newrelic"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["newrelic/alt"].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: "newrelic/alt",
		},
		APIKey:  "a1b2c3d4",
		Timeout: time.Second * 30,
		CommonAttributes: map[string]interface{}{
			"server": "test-server",
			"prod":   true,
			"weight": 3,
		},
		MetricsURLOverride: "http://alt.metrics.newrelic.com",
		SpansURLOverride:   "http://alt.spans.newrelic.com",
	})

	nrConfig := new(telemetry.Config)
	r1.HarvestOption(nrConfig)

	assert.Equal(t, nrConfig, &telemetry.Config{
		APIKey:         "a1b2c3d4",
		HarvestTimeout: time.Second * 30,
		CommonAttributes: map[string]interface{}{
			"server": "test-server",
			"prod":   true,
			"weight": 3,
		},
		MetricsURLOverride: "http://alt.metrics.newrelic.com",
		SpansURLOverride:   "http://alt.spans.newrelic.com",
		Product:            product,
		ProductVersion:     version,
	})
}
