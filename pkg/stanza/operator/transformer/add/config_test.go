// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package add

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "add_value",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = "randomMessage"
					return cfg
				}(),
			},
			{
				Name: "add_expr",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = `EXPR(body.key + "_suffix")`
					return cfg
				}(),
			},
			{
				Name: "add_nest",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = map[string]any{
						"nest": map[string]any{"key": "val"},
					}
					return cfg
				}(),
			},
			{
				Name: "add_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("new")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("one", "two")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_obj_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("one", "two")
					cfg.Value = map[string]any{
						"nest": map[string]any{"key": "val"},
					}
					return cfg
				}(),
			},
			{
				Name: "add_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("new")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("one", "two")
					cfg.Value = "newVal"
					return cfg
				}(),
			},
			{
				Name: "add_nested_obj_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("one", "two")
					cfg.Value = map[string]any{
						"nest": map[string]any{"key": "val"},
					}
					return cfg
				}(),
			},
			{
				Name: "add_resource_expr",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("new")
					cfg.Value = `EXPR(body.key + "_suffix")`
					return cfg
				}(),
			},
			{
				Name: "add_array_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("new")
					cfg.Value = []any{1, 2, 3, 4}
					return cfg
				}(),
			},
			{
				Name: "add_array_to_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("new")
					cfg.Value = []any{1, 2, 3, 4}
					return cfg
				}(),
			},

			{
				Name: "add_array_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("new")
					cfg.Value = []any{1, 2, 3, 4}
					return cfg
				}(),
			},
		},
	}.Run(t)
}
