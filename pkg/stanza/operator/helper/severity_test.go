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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type severityTestCase struct {
	name       string
	sample     interface{}
	mappingSet string
	mapping    map[interface{}]interface{}
	buildErr   bool
	parseErr   bool
	expected   entry.Severity
}

// These tests ensure that users may build a mapping that
// maps values into any of the predefined keys.
// For example, this ensures that users can do this:
//   mapping:
//     warn3: warn_three
func validMappingKeyCases() []severityTestCase {
	aliasedMapping := map[string]entry.Severity{
		"trace":  entry.Trace,
		"trace2": entry.Trace2,
		"trace3": entry.Trace3,
		"trace4": entry.Trace4,
		"debug":  entry.Debug,
		"debug2": entry.Debug2,
		"debug3": entry.Debug3,
		"debug4": entry.Debug4,
		"info":   entry.Info,
		"info2":  entry.Info2,
		"info3":  entry.Info3,
		"info4":  entry.Info4,
		"warn":   entry.Warn,
		"warn2":  entry.Warn2,
		"warn3":  entry.Warn3,
		"warn4":  entry.Warn4,
		"error":  entry.Error,
		"error2": entry.Error2,
		"error3": entry.Error3,
		"error4": entry.Error4,
		"fatal":  entry.Fatal,
		"fatal2": entry.Fatal2,
		"fatal3": entry.Fatal3,
		"fatal4": entry.Fatal4,
	}

	cases := []severityTestCase{}
	for k, v := range aliasedMapping {
		cases = append(cases,
			severityTestCase{
				name:     k,
				sample:   "my_custom_value",
				mapping:  map[interface{}]interface{}{k: "my_custom_value"},
				expected: v,
			})
	}

	return cases
}

// These cases ensure that text representing OTLP severity
// levels are automatically recognized by the default mapping.
func otlpSevCases() []severityTestCase {
	mustParse := map[string]entry.Severity{
		"tRaCe":  entry.Trace,
		"tRaCe2": entry.Trace2,
		"tRaCe3": entry.Trace3,
		"tRaCe4": entry.Trace4,
		"dEbUg":  entry.Debug,
		"dEbUg2": entry.Debug2,
		"dEbUg3": entry.Debug3,
		"dEbUg4": entry.Debug4,
		"iNFo":   entry.Info,
		"iNFo2":  entry.Info2,
		"iNFo3":  entry.Info3,
		"iNFo4":  entry.Info4,
		"wARn":   entry.Warn,
		"wARn2":  entry.Warn2,
		"wARn3":  entry.Warn3,
		"wARn4":  entry.Warn4,
		"eRrOr":  entry.Error,
		"eRrOr2": entry.Error2,
		"eRrOr3": entry.Error3,
		"eRrOr4": entry.Error4,
		"fAtAl":  entry.Fatal,
		"fAtAl2": entry.Fatal2,
		"fAtAl3": entry.Fatal3,
		"fAtAl4": entry.Fatal4,
	}

	cases := []severityTestCase{}
	for k, v := range mustParse {
		cases = append(cases,
			severityTestCase{
				name:     fmt.Sprintf("otlp-sev-%s-mIxEd", k),
				sample:   k,
				expected: v,
			},
			severityTestCase{
				name:     fmt.Sprintf("otlp-sev-%s-lower", k),
				sample:   strings.ToLower(k),
				expected: v,
			},
			severityTestCase{
				name:     fmt.Sprintf("otlp-sev-%s-upper", k),
				sample:   strings.ToUpper(k),
				expected: v,
			},
			severityTestCase{
				name:     fmt.Sprintf("otlp-sev-%s-title", k),
				sample:   strings.ToTitle(k),
				expected: v,
			})
	}
	return cases
}

func TestSeverityParser(t *testing.T) {
	allTheThingsMap := map[interface{}]interface{}{
		"info":   "3xx",
		"error3": "4xx",
		"debug4": "5xx",
		"trace2": []interface{}{
			"ttttttracer",
			[]byte{100, 100, 100},
			map[interface{}]interface{}{"min": 1111, "max": 1234},
		},
		"fatal2": "",
	}

	testCases := []severityTestCase{
		{
			name:     "unknown",
			sample:   "blah",
			mapping:  nil,
			expected: entry.Default,
		},
		{
			name:     "error",
			sample:   "error",
			mapping:  nil,
			expected: entry.Error,
		},
		{
			name:     "error2",
			sample:   "error2",
			mapping:  nil,
			expected: entry.Error2,
		},
		{
			name:     "error3",
			sample:   "error3",
			mapping:  nil,
			expected: entry.Error3,
		},
		{
			name:     "error4",
			sample:   "error4",
			mapping:  nil,
			expected: entry.Error4,
		},
		{
			name:     "error-capitalized",
			sample:   "Error",
			mapping:  nil,
			expected: entry.Error,
		},
		{
			name:     "error-all-caps",
			sample:   "ERROR",
			mapping:  nil,
			expected: entry.Error,
		},
		{
			name:     "custom-string",
			sample:   "NOOOOOOO",
			mapping:  map[interface{}]interface{}{"error": "NOOOOOOO"},
			expected: entry.Error,
		},
		{
			name:     "custom-string-caps-key",
			sample:   "NOOOOOOO",
			mapping:  map[interface{}]interface{}{"ErRoR": "NOOOOOOO"},
			expected: entry.Error,
		},
		{
			name:     "custom-int",
			sample:   1234,
			mapping:  map[interface{}]interface{}{"error": 1234},
			expected: entry.Error,
		},
		{
			name:     "mixed-list-string",
			sample:   "ThiS Is BaD",
			mapping:  map[interface{}]interface{}{"error": []interface{}{"NOOOOOOO", "this is bad", 1234}},
			expected: entry.Error,
		},
		{
			name:     "custom-float64",
			sample:   float64(6),
			mapping:  map[interface{}]interface{}{"error": 6},
			expected: entry.Error,
		},
		{
			name:     "mixed-list-int",
			sample:   1234,
			mapping:  map[interface{}]interface{}{"error": []interface{}{"NOOOOOOO", "this is bad", 1234}},
			expected: entry.Error,
		},
		{
			name:     "numbered-level",
			sample:   "critical",
			mapping:  map[interface{}]interface{}{"error2": "critical"},
			expected: entry.Error2,
		},
		{
			name:     "override-standard",
			sample:   "error",
			mapping:  map[interface{}]interface{}{"error3": []interface{}{"error"}},
			expected: entry.Error3,
		},
		{
			name:     "level-unfound",
			sample:   "not-in-the-list-but-thats-ok",
			mapping:  map[interface{}]interface{}{"error4": []interface{}{"hey!", 1234}},
			expected: entry.Default,
		},
		{
			name:     "in-range",
			sample:   123,
			mapping:  map[interface{}]interface{}{"error": map[interface{}]interface{}{"min": 120, "max": 125}},
			expected: entry.Error,
		},
		{
			name:     "in-range-min",
			sample:   120,
			mapping:  map[interface{}]interface{}{"error": map[interface{}]interface{}{"min": 120, "max": 125}},
			expected: entry.Error,
		},
		{
			name:     "in-range-max",
			sample:   125,
			mapping:  map[interface{}]interface{}{"error": map[interface{}]interface{}{"min": 120, "max": 125}},
			expected: entry.Error,
		},
		{
			name:     "out-of-range-min-minus",
			sample:   119,
			mapping:  map[interface{}]interface{}{"error": map[interface{}]interface{}{"min": 120, "max": 125}},
			expected: entry.Default,
		},
		{
			name:     "out-of-range-max-plus",
			sample:   126,
			mapping:  map[interface{}]interface{}{"error": map[interface{}]interface{}{"min": 120, "max": 125}},
			expected: entry.Default,
		},
		{
			name:     "range-out-of-order",
			sample:   123,
			mapping:  map[interface{}]interface{}{"error": map[interface{}]interface{}{"min": 125, "max": 120}},
			expected: entry.Error,
		},
		{
			name:     "Http2xx-hit",
			sample:   201,
			mapping:  map[interface{}]interface{}{"error": "2xx"},
			expected: entry.Error,
		},
		{
			name:     "Http2xx-miss",
			sample:   301,
			mapping:  map[interface{}]interface{}{"error": "2xx"},
			expected: entry.Default,
		},
		{
			name:     "Http3xx-hit",
			sample:   301,
			mapping:  map[interface{}]interface{}{"error": "3xx"},
			expected: entry.Error,
		},
		{
			name:     "Http4xx-hit",
			sample:   "404",
			mapping:  map[interface{}]interface{}{"error": "4xx"},
			expected: entry.Error,
		},
		{
			name:     "Http5xx-hit",
			sample:   555,
			mapping:  map[interface{}]interface{}{"error": "5xx"},
			expected: entry.Error,
		},
		{
			name:     "Http-All",
			sample:   "301",
			mapping:  map[interface{}]interface{}{"debug": "2xx", "info": "3xx", "error": "4xx", "warn": "5xx"},
			expected: entry.Info,
		},
		{
			name:     "all-the-things-midrange",
			sample:   1234,
			mapping:  allTheThingsMap,
			expected: entry.Trace2,
		},
		{
			name:     "all-the-things-bytes",
			sample:   []byte{100, 100, 100},
			mapping:  allTheThingsMap,
			expected: entry.Trace2,
		},
		{
			name:     "all-the-things-empty",
			sample:   "",
			mapping:  allTheThingsMap,
			expected: entry.Fatal2,
		},
		{
			name:     "all-the-things-3xx",
			sample:   "399",
			mapping:  allTheThingsMap,
			expected: entry.Info,
		},
		{
			name:     "all-the-things-miss",
			sample:   "miss",
			mapping:  allTheThingsMap,
			expected: entry.Default,
		},
		{
			name:       "base-mapping-none",
			sample:     "error",
			mappingSet: "none",
			mapping:    nil,
			expected:   entry.Default, // not error
		},
	}

	testCases = append(testCases, otlpSevCases()...)
	testCases = append(testCases, validMappingKeyCases()...)

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("root", tc.run(rootField))
			t.Run("non-root", tc.run(someField))
		})
	}
}

func TestOtelPreset(t *testing.T) {
	expected := map[string]entry.Severity{
		"trace":  entry.Trace,
		"1":      entry.Trace,
		"trace2": entry.Trace2,
		"2":      entry.Trace2,
		"trace3": entry.Trace3,
		"3":      entry.Trace3,
		"trace4": entry.Trace4,
		"4":      entry.Trace4,
		"debug":  entry.Debug,
		"5":      entry.Debug,
		"debug2": entry.Debug2,
		"6":      entry.Debug2,
		"debug3": entry.Debug3,
		"7":      entry.Debug3,
		"debug4": entry.Debug4,
		"8":      entry.Debug4,
		"info":   entry.Info,
		"9":      entry.Info,
		"info2":  entry.Info2,
		"10":     entry.Info2,
		"info3":  entry.Info3,
		"11":     entry.Info3,
		"info4":  entry.Info4,
		"12":     entry.Info4,
		"warn":   entry.Warn,
		"13":     entry.Warn,
		"warn2":  entry.Warn2,
		"14":     entry.Warn2,
		"warn3":  entry.Warn3,
		"15":     entry.Warn3,
		"warn4":  entry.Warn4,
		"16":     entry.Warn4,
		"error":  entry.Error,
		"17":     entry.Error,
		"error2": entry.Error2,
		"18":     entry.Error2,
		"error3": entry.Error3,
		"19":     entry.Error3,
		"error4": entry.Error4,
		"20":     entry.Error4,
		"fatal":  entry.Fatal,
		"21":     entry.Fatal,
		"fatal2": entry.Fatal2,
		"22":     entry.Fatal2,
		"fatal3": entry.Fatal3,
		"23":     entry.Fatal3,
		"fatal4": entry.Fatal4,
		"24":     entry.Fatal4,
	}

	testCases := make([]severityTestCase, 0, len(expected))
	for k, v := range expected {
		testCases = append(testCases, severityTestCase{
			name:       fmt.Sprintf("otel-%s", k),
			sample:     k,
			mappingSet: "otel",
			expected:   v,
		})
	}

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("root", tc.run(rootField))
			t.Run("non-root", tc.run(someField))
		})
	}
}

func (tc severityTestCase) run(parseFrom entry.Field) func(*testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		cfg := &SeverityConfig{
			ParseFrom: &parseFrom,
			Preset:    tc.mappingSet,
			Mapping:   tc.mapping,
		}

		severityParser, err := cfg.Build(testutil.Logger(t))
		if tc.buildErr {
			require.Error(t, err, "expected error when configuring operator")
			return
		}
		require.NoError(t, err, "unexpected error when configuring operator")

		ent := entry.New()
		require.NoError(t, ent.Set(parseFrom, tc.sample))
		err = severityParser.Parse(ent)
		if tc.parseErr {
			require.Error(t, err, "expected error when parsing sample")
			return
		}
		require.NoError(t, err)

		require.Equal(t, tc.expected, ent.Severity)
	}
}

type severityConfigTestCase struct {
	name      string
	expectErr bool
	expect    *SeverityConfig
}

func TestGoldenSeverityParserConfig(t *testing.T) {
	cases := []severityConfigTestCase{
		{
			"default",
			false,
			defaultSeverityCfg(),
		},
		{
			"parse_from_simple",
			false,
			func() *SeverityConfig {
				cfg := defaultSeverityCfg()
				newParse := entry.NewBodyField("from")
				cfg.ParseFrom = &newParse
				return cfg
			}(),
		},
		{
			"mapping",
			false,
			func() *SeverityConfig {
				cfg := defaultSeverityCfg()
				cfg.Mapping = map[interface{}]interface{}{
					"critical": "5xx",
					"error":    "4xx",
					"info":     "3xx",
					"debug":    "2xx",
				}
				return cfg
			}(),
		},
		{
			"preset",
			false,
			func() *SeverityConfig {
				cfg := defaultSeverityCfg()
				cfg.Preset = "default"
				return cfg
			}(),
		},
	}

	for _, tc := range cases {
		t.Run("yaml/"+tc.name, func(t *testing.T) {
			cfgFromYaml, yamlErr := severityConfigFromFileViaYaml(path.Join(".", "testdata", "severity", fmt.Sprintf("%s.yaml", tc.name)))
			if tc.expectErr {
				require.Error(t, yamlErr)
			} else {
				require.NoError(t, yamlErr)
				require.Equal(t, tc.expect, cfgFromYaml)
			}
		})
		t.Run("mapstructure/"+tc.name, func(t *testing.T) {
			cfgFromMapstructure := defaultSeverityCfg()
			mapErr := severityConfigFromFileViaMapstructure(
				path.Join(".", "testdata", "severity", fmt.Sprintf("%s.yaml", tc.name)),
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

func severityConfigFromFileViaYaml(file string) (*SeverityConfig, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not find config file: %w", err)
	}

	config := defaultSeverityCfg()
	if err := yaml.Unmarshal(bytes, config); err != nil {
		return nil, fmt.Errorf("failed to read config file as yaml: %w", err)
	}

	return config, nil
}

func severityConfigFromFileViaMapstructure(file string, result *SeverityConfig) error {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("could not find config file: %w", err)
	}

	raw := map[string]interface{}{}

	if err = yaml.Unmarshal(bytes, raw); err != nil {
		return fmt.Errorf("failed to read data from yaml: %w", err)
	}

	err = UnmarshalMapstructure(raw, result)
	return err
}

func defaultSeverityCfg() *SeverityConfig {
	newCfg := NewSeverityConfig()
	return &newCfg
}
