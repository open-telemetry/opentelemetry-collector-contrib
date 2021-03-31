// Copyright The OpenTelemetry Authors
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
package metadata

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

type testCase struct {
	name      string
	expectErr bool
	expect    *MetadataOperatorConfig
}

func TestMetaDataGoldenConfig(t *testing.T) {
	cases := []testCase{
		{
			"default",
			false,
			defaultCfg(),
		},
		{
			"id",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.OperatorID = "newName"
				return cfg
			}(),
		},
		{
			"output",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.OutputIDs = append(cfg.OutputIDs, "nextOperator")
				return cfg
			}(),
		},
		{
			"attributes_single",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"key1": `val1`,
				}
				return cfg
			}(),
		},
		{
			"attributes_multi",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Attributes = map[string]helper.ExprStringConfig{
					"key1": `val1`,
					"key2": `val2`,
					"key3": `val3`,
				}
				return cfg
			}(),
		},
		{
			"resource_single",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": `val1`,
				}
				return cfg
			}(),
		},
		{
			"resource_multi",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.Resource = map[string]helper.ExprStringConfig{
					"key1": `val1`,
					"key2": `val2`,
					"key3": `val3`,
				}
				return cfg
			}(),
		},
		{
			"on_error",
			false,
			func() *MetadataOperatorConfig {
				cfg := defaultCfg()
				cfg.OnError = "drop"
				return cfg
			}(),
		},
	}

	for _, tc := range cases {
		t.Run("yaml/"+tc.name, func(t *testing.T) {
			cfgFromYaml, yamlErr := configFromFileViaYaml(path.Join(".", "testdata", fmt.Sprintf("%s.yaml", tc.name)))
			if tc.expectErr {
				require.Error(t, yamlErr)
			} else {
				require.NoError(t, yamlErr)
				require.Equal(t, tc.expect, cfgFromYaml)
			}
		})
		t.Run("mapstructure/"+tc.name, func(t *testing.T) {
			cfgFromMapstructure := defaultCfg()
			mapErr := configFromFileViaMapstructure(
				path.Join(".", "testdata", fmt.Sprintf("%s.yaml", tc.name)),
				cfgFromMapstructure,
			)
			if tc.expectErr {
				require.Error(t, mapErr)
			} else {
				require.NoError(t, mapErr)
				require.Equal(t, tc.expect, cfgFromMapstructure)
			}
		})
	}
}

func configFromFileViaYaml(file string) (*MetadataOperatorConfig, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not find config file: %s", err)
	}

	config := defaultCfg()
	if err := yaml.Unmarshal(bytes, config); err != nil {
		return nil, fmt.Errorf("failed to read config file as yaml: %s", err)
	}

	return config, nil
}

func configFromFileViaMapstructure(file string, result *MetadataOperatorConfig) error {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not find config file: %s", err)
	}

	raw := map[string]interface{}{}

	if err := yaml.Unmarshal(bytes, raw); err != nil {
		return fmt.Errorf("failed to read data from yaml: %s", err)
	}

	err = helper.UnmarshalMapstructure(raw, result)
	return err
}

func defaultCfg() *MetadataOperatorConfig {
	return NewMetadataOperatorConfig("metadata")
}
