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
		expCfg     *Config
	}{
		{
			configFile: "config_full.yaml",
			expCfg: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
				Rules: []Rule{
					{
						Name:      "new_metric",
						Unit:      "percent",
						Type:      "calculate",
						Metric1:   "metric1",
						Metric2:   "metric2",
						Operation: "percent",
					},
					{
						Name:      "new_metric",
						Unit:      "unit",
						Type:      "scale",
						Metric1:   "metric1",
						ScaleBy:   1000,
						Operation: "multiply",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.expCfg.ID().String(), func(t *testing.T) {
			factories, err := componenttest.NopFactories()
			assert.NoError(t, err)

			factory := NewFactory()
			factories.Processors[typeStr] = factory
			config, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", test.configFile), factories)
			assert.NoError(t, err)
			require.NotNil(t, config)

			cfg := config.Processors[test.expCfg.ID()]
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
			errorMessage: fmt.Sprintf("missing required field %q", nameFieldName),
		},
		{
			configName:   "config_missing_type.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", typeFieldName),
		},
		{
			configName:   "config_invalid_generation_type.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", typeFieldName, generationTypeKeys()),
		},
		{
			configName:   "config_missing_operand1.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", metric1FieldName),
		},
		{
			configName:   "config_missing_operand2.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q for generation type %q", metric2FieldName, calculate),
		},
		{
			configName:   "config_missing_scale_by.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("field %q required to be greater than 0 for generation type %q", scaleByFieldName, scale),
		},
		{
			configName:   "config_invalid_operation.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", operationFieldName, operationTypeKeys()),
		},
	}

	for _, test := range tests {
		factories, err := componenttest.NopFactories()
		assert.NoError(t, err)

		factory := NewFactory()
		factories.Processors[typeStr] = factory
		t.Run(test.configName, func(t *testing.T) {
			config, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", test.configName), factories)
			if test.succeed {
				assert.NotNil(t, config)
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, fmt.Sprintf("processor %q has invalid configuration: %s", typeStr, test.errorMessage))
			}
		})

	}
}
