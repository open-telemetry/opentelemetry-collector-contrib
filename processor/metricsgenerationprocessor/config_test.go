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

package metricsgenerationprocessor

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadingFullConfig(t *testing.T) {
	tests := []struct {
		configFile string
		filterName string
		expCfg     *Config
	}{
		{
			configFile: "config_full.yaml",
			filterName: "metricsgeneration",
			expCfg: &Config{
				ProcessorSettings: config.NewProcessorSettings(typeStr),
				Rules: []Rule{
					{
						NewMetricName:  "new_metric",
						Type:           "calculate",
						Operand1Metric: "metric1",
						Operand2Metric: "metric2",
						Operation:      "percent",
					},
					{
						NewMetricName:  "new_metric",
						Type:           "scale",
						Operand1Metric: "metric1",
						ScaleBy:        1000,
						Operation:      "multiply",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.filterName, func(t *testing.T) {

			factories, err := componenttest.NopFactories()
			assert.NoError(t, err)

			factory := NewFactory()
			factories.Processors[config.Type(typeStr)] = factory
			config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", test.configFile), factories)
			assert.NoError(t, err)
			require.NotNil(t, config)

			cfg := config.Processors[test.filterName]
			assert.Equal(t, test.expCfg, cfg)
		})
	}
}
