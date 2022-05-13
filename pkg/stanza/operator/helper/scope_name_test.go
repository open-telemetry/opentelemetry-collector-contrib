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

package helper

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

const testScopeName = "my.logger"

func TestScopeNameParser(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name      string
		parser    *ScopeNameParser
		input     *entry.Entry
		expectErr bool
		expected  *entry.Entry
	}{
		{
			name: "root_string",
			parser: &ScopeNameParser{
				ParseFrom: entry.NewBodyField(),
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = testScopeName
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "nondestructive_error",
			parser: &ScopeNameParser{
				ParseFrom: entry.NewBodyField(),
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expectErr: true,
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
		},
		{
			name: "nonroot_string",
			parser: &ScopeNameParser{
				ParseFrom: entry.NewBodyField("logger"),
			},
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ObservedTimestamp = now
				return e
			}(),
			expected: func() *entry.Entry {
				e := entry.New()
				e.Body = map[string]interface{}{"logger": testScopeName}
				e.ScopeName = testScopeName
				e.ObservedTimestamp = now
				return e
			}(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.parser.Parse(tc.input)
			if tc.expectErr {
				require.Error(t, err)
			}
			if tc.expected != nil {
				require.Equal(t, tc.expected, tc.input)
			}
		})
	}
}

func TestGoldenScopeNameParserConfig(t *testing.T) {
	cases := []struct {
		name      string
		expectErr bool
		expect    *ScopeNameParser
	}{
		{
			"parse_from",
			false,
			func() *ScopeNameParser {
				cfg := NewScopeNameParser()
				cfg.ParseFrom = entry.NewBodyField("from")
				return &cfg
			}(),
		},
	}

	for _, tc := range cases {
		t.Run("yaml/"+tc.name, func(t *testing.T) {
			cfgFromYaml, yamlErr := ScopeConfigFromFileViaYaml(path.Join(".", "testdata", "scope_name", fmt.Sprintf("%s.yaml", tc.name)))
			if tc.expectErr {
				require.Error(t, yamlErr)
			} else {
				require.NoError(t, yamlErr)
				require.Equal(t, tc.expect, cfgFromYaml)
			}
		})
		t.Run("mapstructure/"+tc.name, func(t *testing.T) {
			parser := NewScopeNameParser()
			cfgFromMapstructure := &parser
			mapErr := ScopeConfigFromFileViaMapstructure(
				path.Join(".", "testdata", "scope_name", fmt.Sprintf("%s.yaml", tc.name)),
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

func ScopeConfigFromFileViaYaml(file string) (*ScopeNameParser, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not find config file: %s", err)
	}

	parser := NewScopeNameParser()
	config := &parser
	if err := yaml.Unmarshal(bytes, config); err != nil {
		return nil, fmt.Errorf("failed to read config file as yaml: %s", err)
	}

	return config, nil
}

func ScopeConfigFromFileViaMapstructure(file string, result *ScopeNameParser) error {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not find config file: %s", err)
	}

	raw := map[string]interface{}{}

	if err := yaml.Unmarshal(bytes, raw); err != nil {
		return fmt.Errorf("failed to read data from yaml: %s", err)
	}

	err = UnmarshalMapstructure(raw, result)
	return err
}
