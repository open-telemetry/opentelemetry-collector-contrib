// Copyright 2019, OpenTelemetry Authors
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

package azuremonitorexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
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

	exporterType := typeStr
	exporter := cfg.Exporters[exporterType]
	assert.Equal(t, factory.CreateDefaultConfig(), exporter)

	exporterType = typeStr + "/2"
	exporter = cfg.Exporters[exporterType].(*Config)
	assert.NoError(t, configcheck.ValidateConfig(exporter))
	assert.Equal(
		t,
		&Config{
			ExporterSettings:   configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: exporterType},
			Endpoint:           defaultEndpoint,
			InstrumentationKey: "abcdefg",
			MaxBatchSize:       100,
			MaxBatchInterval:   10 * time.Second,
		},
		exporter)
}
