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
package json

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

type testCase struct {
	name      string
	expectErr bool
	expect    *JSONParserConfig
}

func TestJSONParserConfig(t *testing.T) {
	cases := []testCase{
		{
			"default",
			false,
			defaultCfg(),
		},
		{
			"parse_from_simple",
			false,
			func() *JSONParserConfig {
				cfg := defaultCfg()
				cfg.ParseFrom = entry.NewRecordField("from")
				return cfg
			}(),
		},
		{
			"parse_to_simple",
			false,
			func() *JSONParserConfig {
				cfg := defaultCfg()
				cfg.ParseTo = entry.NewRecordField("log")
				return cfg
			}(),
		},
		{
			"on_error_drop",
			false,
			func() *JSONParserConfig {
				cfg := defaultCfg()
				cfg.OnError = "drop"
				return cfg
			}(),
		},
		{
			"timestamp",
			false,
			func() *JSONParserConfig {
				cfg := defaultCfg()
				parseField := entry.NewRecordField("timestamp_field")
				newTime := helper.TimeParser{
					LayoutType: "strptime",
					Layout:     "%Y-%m-%d",
					ParseFrom:  &parseField,
				}
				cfg.TimeParser = &newTime
				return cfg
			}(),
		},
		{
			"severity",
			false,
			func() *JSONParserConfig {
				cfg := defaultCfg()
				parseField := entry.NewRecordField("severity_field")
				severityField := helper.NewSeverityParserConfig()
				severityField.ParseFrom = &parseField
				mapping := map[interface{}]interface{}{
					"critical": "5xx",
					"error":    "4xx",
					"info":     "3xx",
					"debug":    "2xx",
				}
				severityField.Mapping = mapping
				cfg.SeverityParserConfig = &severityField
				return cfg
			}(),
		},
		{
			"preserve_to",
			false,
			func() *JSONParserConfig {
				cfg := defaultCfg()
				preserve := entry.NewRecordField("aField")
				cfg.PreserveTo = &preserve
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

func configFromFileViaYaml(file string) (*JSONParserConfig, error) {
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

func configFromFileViaMapstructure(file string, result *JSONParserConfig) error {
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

func defaultCfg() *JSONParserConfig {
	return NewJSONParserConfig("json_parser")
}
