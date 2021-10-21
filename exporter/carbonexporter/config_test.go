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

package carbonexporter

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e0 := cfg.Exporters[config.NewComponentID(typeStr)]

	defaultCfg := factory.CreateDefaultConfig().(*Config)
	assert.Equal(t, defaultCfg, e0)

	e1 := cfg.Exporters[config.NewComponentIDWithName(typeStr, "allsettings")]
	expectedCfg := Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentIDWithName(typeStr, "allsettings")),
		Endpoint:         "localhost:8080",
		Timeout:          10 * time.Second,
	}
	assert.Equal(t, &expectedCfg, e1)

	te, err := factory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), e1)
	require.NoError(t, err)
	require.NotNil(t, te)
}
