// Copyright 2021 OpenTelemetry Authors
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

package uptraceexporter

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	require.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory

	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Equal(t, 2, len(cfg.Exporters))

	c0 := cfg.Exporters["uptrace"].(*Config)
	require.Equal(t, configmodels.ExporterSettings{
		TypeVal: configmodels.Type(typeStr),
		NameVal: "uptrace",
	}, c0.ExporterSettings)
	require.Equal(t, "https://api.uptrace.dev@example.com/1", c0.DSN)

	c1 := cfg.Exporters["uptrace/customname"].(*Config)
	require.Equal(t, configmodels.ExporterSettings{
		TypeVal: configmodels.Type(typeStr),
		NameVal: "uptrace/customname",
	}, c1.ExporterSettings)
	require.Equal(t, "https://key@example.com/1", c1.DSN)
}
