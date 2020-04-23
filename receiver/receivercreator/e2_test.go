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

package receivercreator

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/service"
	"github.com/open-telemetry/opentelemetry-collector/service/defaultcomponents"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestK8sEndToEnd(t *testing.T) {
	components, err := defaultcomponents.Components()
	require.NoError(t, err)
	components.Receivers[typeStr] = &Factory{}
	exampleExporter := &config.ExampleExporterFactory{}
	components.Exporters[exampleExporter.Type()] = exampleExporter
	app, err := service.New(service.Parameters{
		Factories:            components,
		ApplicationStartInfo: service.ApplicationStartInfo{},
		ConfigFactory: func(v *viper.Viper, factories config.Factories) (*configmodels.Config, error) {
			return config.LoadConfigFile(t, "testdata/e2e.yaml", components)
		},
	})
	require.NoError(t, err)
	require.NotNil(t, app)
	require.NoError(t, app.Start())
}
