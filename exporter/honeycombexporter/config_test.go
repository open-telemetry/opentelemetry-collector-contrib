// Copyright 2019 OpenTelemetry Authors
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

package honeycombexporter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 3)

	r0 := cfg.Exporters["honeycomb"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["honeycomb/customname"].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "honeycomb/customname"},
		APIKey:           "test-apikey",
		Dataset:          "test-dataset",
		APIURL:           "https://api.testhost.io",
		SampleRate:       1,
	})

	r2 := cfg.Exporters["honeycomb/sample_rate"].(*Config)
	assert.Equal(t, r2, &Config{
		ExporterSettings:    configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "honeycomb/sample_rate"},
		APIURL:              "https://api.honeycomb.io",
		SampleRate:          5,
		SampleRateAttribute: "custom.sample_rate",
	})
}
