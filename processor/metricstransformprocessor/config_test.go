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
	"testing"
)

// TestLoadingConfigStrict tests loading testdata/config.yaml
func TestLoadingFullConfig(t *testing.T) {
	// testDataOperations := []Operation{
	// 	{
	// 		Action:   "update_label",
	// 		Label:    "label",
	// 		NewLabel: "new_label",
	// 	},
	// }

	// factories, err := config.ExampleComponents()
	// assert.Nil(t, err)

	// factory := &Factory{}
	// factories.Processors[configmodels.Type(typeStr)] = factory
	// config, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	// assert.Nil(t, err)
	// require.NotNil(t, config)

	// tests := []struct {
	// 	filterName string
	// 	expCfg     *Config
	// }{
	// 	{
	// 		filterName: "transform",
	// 		expCfg: &Config{
	// 			ProcessorSettings: configmodels.ProcessorSettings{
	// 				NameVal: "transform",
	// 				TypeVal: typeStr,
	// 			},
	// 			MetricName: "old_name",
	// 			Action:     "update",
	// 			NewName:    "new_name",
	// 			Operations: testDataOperations,
	// 		},
	// 	},
	// }

	// for _, test := range tests {
	// 	t.Run(test.filterName, func(t *testing.T) {
	// 		cfg := config.Processors[test.filterName]
	// 		assert.Equal(t, test.expCfg, cfg)
	// 	})
	// }
}

func TestLoadingInvalidConfig(t *testing.T) {

}
