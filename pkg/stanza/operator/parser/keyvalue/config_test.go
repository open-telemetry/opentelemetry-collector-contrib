// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package keyvalue

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
				Name: "parse_from_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseFrom = entry.NewBodyField("from")
					return cfg
				}(),
			},
			{
				Name: "parse_to_simple",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField("log")}
					return cfg
				}(),
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
				Name: "timestamp",
				Expect: func() *Config {
					cfg := NewConfig()
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
					parseField := entry.NewBodyField("severity_field")
					severityField := helper.NewSeverityConfig()
					severityField.ParseFrom = &parseField
					mapping := map[string]interface{}{
						"critical": "5xx",
						"error":    "4xx",
						"info":     "3xx",
						"debug":    "2xx",
					}
					severityField.Mapping = mapping
					cfg.SeverityConfig = &severityField
					return cfg
				}(),
			},
			{
				Name: "delimiter",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Delimiter = ";"
					return cfg
				}(),
			},
			{
				Name: "pair_delimiter",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PairDelimiter = ";"
					return cfg
				}(),
			},
			{
				Name: "pair_delimiter_multiline",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.PairDelimiter = "^\n"
					return cfg
				}(),
			},
			{
				Name: "parse_to_attributes",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
					return p
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
					return p
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewResourceField()}
					return p
				}(),
			},
		},
	}.Run(t)
}
