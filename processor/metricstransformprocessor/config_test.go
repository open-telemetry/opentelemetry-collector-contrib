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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
)

var (
	testDataOperations = []Operation{
		{
			Action:   UpdateLabel,
			Label:    "label",
			NewLabel: "new_label",
			ValueActions: []ValueAction{
				{
					Value:    "current_label_value",
					NewValue: "new_label_value",
				},
			},
		},
		{
			Action: AggregateLabels,
			LabelSet: []string{
				"label1",
				"label2",
			},
			AggregationType: Sum,
		},
		{
			Action: AggregateLabelValues,
			Label:  "label",
			AggregatedValues: []string{
				"value1",
				"value2",
			},
			NewValue:        "new_value",
			AggregationType: Sum,
		},
	}

	tests = []struct {
		filterName string
		expCfg     *Config
	}{
		{
			filterName: "metricstransform",
			expCfg: &Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					NameVal: "metricstransform",
					TypeVal: typeStr,
				},
				Transforms: []Transform{
					{
						MetricName: "old_name",
						Action:     Update,
						NewName:    "new_name",
						Operations: testDataOperations,
					},
				},
			},
		},
	}
)

// TestLoadingFullConfig tests loading testdata/config_full.yaml.
func TestLoadingFullConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.NoError(t, err)

	factory := &Factory{}
	factories.Processors[configmodels.Type(typeStr)] = factory
	config, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config_full.yaml"), factories)

	assert.NoError(t, err)
	require.NotNil(t, config)

	for _, test := range tests {
		t.Run(test.filterName, func(t *testing.T) {
			cfg := config.Processors[test.filterName]
			assert.Equal(t, test.expCfg, cfg)
		})
	}
}
