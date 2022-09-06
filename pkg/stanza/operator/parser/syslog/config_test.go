// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package syslog

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:      "default",
				Expect:    NewConfig(),
				ExpectErr: false, // missing protocol, caught later by Config.Validate()
			},
			{
				Name: "rfc3164",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC3164
					return cfg
				}(),
			},
			{
				Name: "rfc5424",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					return cfg
				}(),
			},
			{
				Name: "location",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.Location = "FOO"
					return cfg
				}(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "parse_from_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.ParseFrom = entry.NewBodyField("from")
					return cfg
				}(),
			},
			{
				Name: "parse_to_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					cfg.ParseTo = entry.NewBodyField("log")
					return cfg
				}(),
			},
			{
				Name: "timestamp",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					parseField := entry.NewBodyField("timestamp_field")
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
				Name: "severity",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					parseField := entry.NewBodyField("severity_field")
					severityParser := helper.NewSeverityConfig()
					severityParser.ParseFrom = &parseField
					mapping := map[interface{}]interface{}{
						"critical": "5xx",
						"error":    "4xx",
						"info":     "3xx",
						"debug":    "2xx",
					}
					severityParser.Mapping = mapping
					cfg.Config = &severityParser
					return cfg
				}(),
			},
			{
				Name: "scope_name",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Protocol = RFC5424
					loggerNameParser := helper.NewScopeNameParser()
					loggerNameParser.ParseFrom = entry.NewBodyField("logger_name_field")
					cfg.ScopeNameParser = &loggerNameParser
					return cfg
				}(),
			},
		},
	}.Run(t)
}

func TestParserMissingProtocol(t *testing.T) {
	_, err := NewConfig().Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing field 'protocol'")
}

func TestParserInvalidLocation(t *testing.T) {
	config := NewConfig()
	config.Location = "not_a_location"
	config.Protocol = RFC3164

	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load location "+config.Location)
}
