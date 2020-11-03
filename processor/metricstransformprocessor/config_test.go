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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
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
		configFile string
		filterName string
		expCfg     *Config
	}{
		{
			configFile: "config_full.yaml",
			filterName: "metricstransform",
			expCfg: &Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					NameVal: "metricstransform",
					TypeVal: typeStr,
				},
				Transforms: []Transform{
					{
						MetricIncludeFilter: FilterConfig{
							Include: "old_name",
						},
						Action:     Update,
						NewName:    "new_name",
						Operations: testDataOperations,
					},
				},
			},
		},
		{
			configFile: "config_full.yaml",
			filterName: "metricstransform/addlabel",
			expCfg: &Config{
				ProcessorSettings: configmodels.ProcessorSettings{
					NameVal: "metricstransform/addlabel",
					TypeVal: typeStr,
				},
				Transforms: []Transform{
					{
						MetricIncludeFilter: FilterConfig{
							Include:   "some_name",
							MatchType: "regexp",
						},
						Action: Update,
						Operations: []Operation{
							{
								Action:   AddLabel,
								NewLabel: "mylabel",
								NewValue: "myvalue",
							},
						},
					},
				},
			},
		},
		{
			configFile: "config_deprecated.yaml",
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
					},
				},
			},
		},
	}
)

// TestLoadingFullConfig tests loading testdata/config_full.yaml.
func TestLoadingFullConfig(t *testing.T) {
	for _, test := range tests {
		t.Run(test.filterName, func(t *testing.T) {

			factories, err := componenttest.ExampleComponents()
			assert.NoError(t, err)

			factory := NewFactory()
			factories.Processors[configmodels.Type(typeStr)] = factory
			config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", test.configFile), factories)
			assert.NoError(t, err)
			require.NotNil(t, config)

			cfg := config.Processors[test.filterName]
			assert.Equal(t, test.expCfg, cfg)
		})
	}
}
