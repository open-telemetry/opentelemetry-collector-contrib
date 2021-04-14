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
package copy

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

type configTestCase struct {
	name   string
	expect *CopyOperatorConfig
}

// test unmarshalling of values into config struct
func TestGoldenConfig(t *testing.T) {
	cases := []configTestCase{
		{
			"body_to_body",
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
		},
		{
			"body_to_attribute",
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewAttributeField("key2")
				return cfg
			}(),
		},
		{
			"attribute_to_resource",
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewResourceField("key2")
				return cfg
			}(),
		},
		{
			"attribute_to_body",
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgFromYaml, yamlErr := configFromFileViaYaml(path.Join(".", "testdata", fmt.Sprintf("%s.yaml", tc.name)))
			cfgFromMapstructure, mapErr := configFromFileViaMapstructure(path.Join(".", "testdata", fmt.Sprintf("%s.yaml", tc.name)))
			require.NoError(t, yamlErr)
			require.Equal(t, tc.expect, cfgFromYaml)
			require.NoError(t, mapErr)
			require.Equal(t, tc.expect, cfgFromMapstructure)
		})
	}
}

func configFromFileViaYaml(file string) (*CopyOperatorConfig, error) {
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

func configFromFileViaMapstructure(file string) (*CopyOperatorConfig, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not find config file: %s", err)
	}

	raw := map[string]interface{}{}

	if err := yaml.Unmarshal(bytes, raw); err != nil {
		return nil, fmt.Errorf("failed to read data from yaml: %s", err)
	}

	cfg := defaultCfg()
	dc := &mapstructure.DecoderConfig{Result: cfg, DecodeHook: helper.JSONUnmarshalerHook()}
	ms, err := mapstructure.NewDecoder(dc)
	if err != nil {
		return nil, err
	}
	err = ms.Decode(raw)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultCfg() *CopyOperatorConfig {
	return NewCopyOperatorConfig("copy")
}
