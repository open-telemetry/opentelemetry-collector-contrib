// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "format_json",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatJSON
					return cfg
				}(),
			},
			{
				Name: "format_console",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatConsole
					return cfg
				}(),
			},
			{
				Name: "format_auto",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatAuto
					return cfg
				}(),
			},
			{
				Name: "parse_from_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseFrom = entry.NewBodyField("from")
					return cfg
				}(),
			},
			{
				Name: "parse_to_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
					return cfg
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
					return cfg
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseTo = entry.RootableField{Field: entry.NewResourceField()}
					return cfg
				}(),
			},
			{
				Name: "scope_name",
				Expect: func() *Config {
					cfg := NewConfig()
					loggerNameParser := helper.NewScopeNameParser()
					loggerNameParser.ParseFrom = entry.NewBodyField("logger_name_field")
					cfg.ScopeNameParser = &loggerNameParser
					return cfg
				}(),
			},
			{
				Name: "severity",
				Expect: func() *Config {
					cfg := NewConfig()
					parseField := entry.NewBodyField("severity_field")
					severityParser := helper.NewSeverityConfig()
					severityParser.ParseFrom = &parseField
					severityParser.Mapping = map[string]any{
						"critical": "5xx",
						"error":    "4xx",
						"info":     "3xx",
						"debug":    "2xx",
					}
					cfg.SeverityConfig = &severityParser
					return cfg
				}(),
			},
			{
				Name: "timestamp",
				Expect: func() *Config {
					cfg := NewConfig()
					parseField := entry.NewBodyField("timestamp_field")
					cfg.TimeParser = &helper.TimeParser{
						LayoutType: "strptime",
						Layout:     "%Y-%m-%d",
						ParseFrom:  &parseField,
					}
					return cfg
				}(),
			},
		},
	}.Run(t)
}
