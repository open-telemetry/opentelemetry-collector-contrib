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
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

const minSeverity = 0
const maxSeverity = 100

// map[string or int input]sev-level
func getBuiltinMapping(name string) severityMap {
	switch name {
	case "none":
		return map[string]entry.Severity{}
	case "aliases":
		return map[string]entry.Severity{
			"default":     entry.Default,
			"trace":       entry.Trace,
			"trace2":      entry.Trace2,
			"trace3":      entry.Trace3,
			"trace4":      entry.Trace4,
			"debug":       entry.Debug,
			"debug2":      entry.Debug2,
			"debug3":      entry.Debug3,
			"debug4":      entry.Debug4,
			"info":        entry.Info,
			"info2":       entry.Info2,
			"info3":       entry.Info3,
			"info4":       entry.Info4,
			"notice":      entry.Notice,
			"warning":     entry.Warning,
			"warning2":    entry.Warning2,
			"warning3":    entry.Warning3,
			"warning4":    entry.Warning4,
			"error":       entry.Error,
			"error2":      entry.Error2,
			"error3":      entry.Error3,
			"error4":      entry.Error4,
			"critical":    entry.Critical,
			"alert":       entry.Alert,
			"emergency":   entry.Emergency,
			"emergency2":  entry.Emergency2,
			"emergency3":  entry.Emergency3,
			"emergency4":  entry.Emergency4,
			"catastrophe": entry.Catastrophe,
		}
	default:
		// Add some additional values that are automatically recognized
		mapping := getBuiltinMapping("aliases")
		mapping.add(entry.Warning, "warn")
		mapping.add(entry.Warning2, "warn2")
		mapping.add(entry.Warning3, "warn3")
		mapping.add(entry.Warning4, "warn4")

		mapping.add(entry.Error, "err")
		mapping.add(entry.Error2, "err2")
		mapping.add(entry.Error3, "err3")
		mapping.add(entry.Error4, "err4")

		mapping.add(entry.Critical, "crit")

		mapping.add(entry.Emergency, "fatal")
		mapping.add(entry.Emergency2, "fatal2")
		mapping.add(entry.Emergency3, "fatal3")
		mapping.add(entry.Emergency4, "fatal4")

		return mapping
	}
}

func (m severityMap) add(severity entry.Severity, parseableValues ...string) {
	for _, str := range parseableValues {
		m[str] = severity
	}
}

const (
	// HTTP2xx is a special key that is represents a range from 200 to 299. Literal value is "2xx"
	HTTP2xx = "2xx"

	// HTTP3xx is a special key that is represents a range from 300 to 399. Literal value is "3xx"
	HTTP3xx = "3xx"

	// HTTP4xx is a special key that is represents a range from 400 to 499. Literal value is "4xx"
	HTTP4xx = "4xx"

	// HTTP5xx is a special key that is represents a range from 500 to 599. Literal value is "5xx"
	HTTP5xx = "5xx"
)

// NewSeverityParserConfig creates a new severity parser config
func NewSeverityParserConfig() SeverityParserConfig {
	return SeverityParserConfig{}
}

// SeverityParserConfig allows users to specify how to parse a severity from a field.
type SeverityParserConfig struct {
	ParseFrom  *entry.Field                `mapstructure:"parse_from,omitempty"  json:"parse_from,omitempty"  yaml:"parse_from,omitempty"`
	PreserveTo *entry.Field                `mapstructure:"preserve_to,omitempty" json:"preserve_to,omitempty" yaml:"preserve_to,omitempty"`
	Preset     string                      `mapstructure:"preset,omitempty"      json:"preset,omitempty"      yaml:"preset,omitempty"`
	Mapping    map[interface{}]interface{} `mapstructure:"mapping,omitempty"     json:"mapping,omitempty"     yaml:"mapping,omitempty"`
}

// Build builds a SeverityParser from a SeverityParserConfig
func (c *SeverityParserConfig) Build(context operator.BuildContext) (SeverityParser, error) {
	operatorMapping := getBuiltinMapping(c.Preset)

	for severity, unknown := range c.Mapping {
		sev, err := validateSeverity(severity)
		if err != nil {
			return SeverityParser{}, err
		}

		switch u := unknown.(type) {
		case []interface{}: // check before interface{}
			for _, value := range u {
				v, err := parseableValues(value)
				if err != nil {
					return SeverityParser{}, err
				}
				operatorMapping.add(sev, v...)
			}
		case interface{}:
			v, err := parseableValues(u)
			if err != nil {
				return SeverityParser{}, err
			}
			operatorMapping.add(sev, v...)
		}
	}

	if c.ParseFrom == nil {
		return SeverityParser{}, fmt.Errorf("missing required field 'parse_from'")
	}

	p := SeverityParser{
		ParseFrom:  *c.ParseFrom,
		PreserveTo: c.PreserveTo,
		Mapping:    operatorMapping,
	}

	return p, nil
}

func validateSeverity(severity interface{}) (entry.Severity, error) {
	if sev, _, err := getBuiltinMapping("aliases").find(severity); err != nil {
		return entry.Default, err
	} else if sev != entry.Default {
		return sev, nil
	}

	// If integer between 0 and 100
	var intSev int
	switch s := severity.(type) {
	case int:
		intSev = s
	case string:
		i, err := strconv.ParseInt(s, 10, 8)
		if err != nil {
			return entry.Default, fmt.Errorf("%s cannot be used as a severity", severity)
		}
		intSev = int(i)
	default:
		return entry.Default, fmt.Errorf("type %T cannot be used as a severity (%v)", severity, severity)
	}

	if intSev < minSeverity || intSev > maxSeverity {
		return entry.Default, fmt.Errorf("severity must be between %d and %d", minSeverity, maxSeverity)
	}
	return entry.Severity(intSev), nil
}

func isRange(value interface{}) (int, int, bool) {
	rawMap, ok := value.(map[interface{}]interface{})
	if !ok {
		return 0, 0, false
	}

	min, minOK := rawMap["min"]
	max, maxOK := rawMap["max"]
	if !minOK || !maxOK {
		return 0, 0, false
	}

	minInt, minOK := min.(int)
	maxInt, maxOK := max.(int)
	if !minOK || !maxOK {
		return 0, 0, false
	}

	return minInt, maxInt, true
}

func expandRange(min, max int) []string {
	if min > max {
		min, max = max, min
	}

	rangeOfStrings := []string{}
	for i := min; i <= max; i++ {
		rangeOfStrings = append(rangeOfStrings, strconv.Itoa(i))
	}
	return rangeOfStrings
}

func parseableValues(value interface{}) ([]string, error) {
	switch v := value.(type) {
	case int:
		return []string{strconv.Itoa(v)}, nil // store as string because we will compare as string
	case string:
		switch v {
		case HTTP2xx:
			return expandRange(200, 299), nil
		case HTTP3xx:
			return expandRange(300, 399), nil
		case HTTP4xx:
			return expandRange(400, 499), nil
		case HTTP5xx:
			return expandRange(500, 599), nil
		default:
			return []string{strings.ToLower(v)}, nil
		}
	case []byte:
		return []string{strings.ToLower(string(v))}, nil
	default:
		min, max, ok := isRange(v)
		if ok {
			return expandRange(min, max), nil
		}
		return nil, fmt.Errorf("type %T cannot be parsed as a severity", v)
	}
}
