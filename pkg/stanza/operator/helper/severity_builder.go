// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// map[string or int input]sev-level
func getBuiltinMapping(name string) severityMap {
	switch name {
	case "none":
		return map[string]entry.Severity{}
	case "otel", "aliases":
		return map[string]entry.Severity{
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
	default:
		// Add some additional values that are automatically recognized
		mapping := getBuiltinMapping("aliases")
		mapping.add(entry.Warn, "warning")
		mapping.add(entry.Warn2, "warning2")
		mapping.add(entry.Warn3, "warning3")
		mapping.add(entry.Warn4, "warning4")
		mapping.add(entry.Error, "err")
		mapping.add(entry.Error2, "err2")
		mapping.add(entry.Error3, "err3")
		mapping.add(entry.Error4, "err4")
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

// NewSeverityConfig creates a new severity parser config
func NewSeverityConfig() SeverityConfig {
	return SeverityConfig{}
}

// SeverityConfig allows users to specify how to parse a severity from a field.
type SeverityConfig struct {
	ParseFrom *entry.Field           `mapstructure:"parse_from,omitempty"`
	Preset    string                 `mapstructure:"preset,omitempty"`
	Mapping   map[string]interface{} `mapstructure:"mapping,omitempty"`
}

// Build builds a SeverityParser from a SeverityConfig
func (c *SeverityConfig) Build(logger *zap.SugaredLogger) (SeverityParser, error) {
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
		ParseFrom: *c.ParseFrom,
		Mapping:   operatorMapping,
	}

	return p, nil
}

func validateSeverity(severity interface{}) (entry.Severity, error) {
	sev, _, err := getBuiltinMapping("aliases").find(severity)
	return sev, err
}

func isRange(value interface{}) (int, int, bool) {
	rawMap, ok := value.(map[string]interface{})
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

	var rangeOfStrings []string
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
