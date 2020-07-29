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

package metricstransformprocessor

import (
	"context"
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestType(t *testing.T) {
	factory := Factory{}
	pType := factory.Type()
	assert.Equal(t, pType, configmodels.Type("metricstransform"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			NameVal: typeStr,
			TypeVal: typeStr,
		},
	})
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateProcessors(t *testing.T) {
	tests := []struct {
		configName   string
		succeed      bool
		errorMessage string
	}{
		{
			configName: "config_full.yaml",
			succeed:    true,
		}, {
			configName:   "config_invalid_newname.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q while %q is %v", NewNameFieldName, ActionFieldName, Insert),
		}, {
			configName:   "config_invalid_action.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("unsupported %q: %v, the supported actions are %q and %q", ActionFieldName, "invalid", Insert, Update),
		}, {
			configName:   "config_invalid_metricname.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", MetricNameFieldName),
		}, {
			configName:   "config_invalid_label.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q while %q is %v in the %vth operation", LabelFieldName, ActionFieldName, UpdateLabel, 0),
		},
	}

	for _, test := range tests {
		factories, err := config.ExampleComponents()
		assert.NoError(t, err)

		factory := &Factory{}
		factories.Processors[typeStr] = factory
		config, err := config.LoadConfigFile(t, path.Join(".", "testdata", test.configName), factories)
		assert.NoError(t, err)

		for name, cfg := range config.Processors {
			t.Run(fmt.Sprintf("%s/%s", test.configName, name), func(t *testing.T) {
				factory := &Factory{}

				tp, tErr := factory.CreateTraceProcessor(
					context.Background(),
					component.ProcessorCreateParams{Logger: zap.NewNop()},
					nil,
					cfg)
				// Not implemented error
				assert.Error(t, tErr)
				assert.Nil(t, tp)

				mp, mErr := factory.CreateMetricsProcessor(
					context.Background(),
					component.ProcessorCreateParams{Logger: zap.NewNop()},
					nil,
					cfg)
				if test.succeed {
					assert.NotNil(t, mp)
					assert.NoError(t, mErr)
				} else {
					assert.EqualError(t, mErr, test.errorMessage)
				}
			})
		}
	}
}
