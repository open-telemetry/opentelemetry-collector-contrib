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
	"fmt"
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

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		configName   string
		succeed      bool
		errorMessage string
	}{
		{
			configName: "config_full.yaml",
			succeed:    true,
		},
		{
			configName:   "config_missing_new_metric.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", NewMetricFieldName),
		},
		{
			configName:   "config_missing_type.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", GenerationTypeFieldName),
		},
		{
			configName:   "config_invalid_generation_type.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", GenerationTypeFieldName, generationTypeKeys()),
		},
		{
			configName:   "config_missing_operand1.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", Operand1MetricFieldName),
		},
		{
			configName:   "config_missing_operand2.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q for generation type %q", Operand2MetricFieldName, Calculate),
		},
		{
			configName:   "config_missing_scale_by.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("field %q required to be greater than 0 for generation type %q", ScaleByFieldName, Scale),
		},
		{
			configName:   "config_invalid_operation.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", OperationFieldName, operationTypeKeys()),
		},
	}

	for _, test := range tests {
		factories, err := componenttest.NopFactories()
		assert.NoError(t, err)

		factory := NewFactory()
		factories.Processors[typeStr] = factory
		t.Run(test.configName, func(t *testing.T) {
			config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", test.configName), factories)
			if test.succeed {
				assert.NotNil(t, config)
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, fmt.Sprintf("processor %q has invalid configuration: %s", typeStr, test.errorMessage))
			}
		})

	}
}
