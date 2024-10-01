package move

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

// test unmarshalling of values into config struct
func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "move_body_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("key")
					cfg.To = entry.NewBodyField("new")
					return cfg
				}(),
			},
			{
				Name: "move_body_to_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("key")
					cfg.To = entry.NewAttributeField("new")
					return cfg
				}(),
			},
			{
				Name: "move_attribute_to_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewAttributeField("new")
					cfg.To = entry.NewBodyField("new")
					return cfg
				}(),
			},
			{
				Name: "move_attribute_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewAttributeField("new")
					cfg.To = entry.NewResourceField("new")
					return cfg
				}(),
			},
			{
				Name: "move_bracketed_attribute_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewAttributeField("dotted.field.name")
					cfg.To = entry.NewResourceField("new")
					return cfg
				}(),
			},
			{
				Name: "move_resource_to_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewResourceField("new")
					cfg.To = entry.NewAttributeField("new")
					return cfg
				}(),
			},
			{
				Name: "move_nested",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("nested")
					cfg.To = entry.NewBodyField("NewNested")
					return cfg
				}(),
			},
			{
				Name: "move_from_nested_object",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("nested", "nestedkey")
					cfg.To = entry.NewBodyField("unnestedkey")
					return cfg
				}(),
			},
			{
				Name: "move_to_nested_object",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("newnestedkey")
					cfg.To = entry.NewBodyField("nested", "newnestedkey")
					return cfg
				}(),
			},
			{
				Name: "move_double_nested_object",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("nested", "nested2")
					cfg.To = entry.NewBodyField("nested2")
					return cfg
				}(),
			},
			{
				Name: "move_nested_to_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("nested")
					cfg.To = entry.NewResourceField("NewNested")
					return cfg
				}(),
			},
			{
				Name: "move_nested_to_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("nested")
					cfg.To = entry.NewAttributeField("NewNested")
					return cfg
				}(),
			},
			{
				Name: "move_nested_body_to_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("one", "two")
					cfg.To = entry.NewAttributeField("three", "four")
					return cfg
				}(),
			},
			{
				Name: "move_nested_body_to_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("one", "two")
					cfg.To = entry.NewResourceField("three", "four")
					return cfg
				}(),
			},
			{
				Name: "replace_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.From = entry.NewBodyField("nested")
					cfg.To = entry.NewBodyField()
					return cfg
				}(),
			},
		},
	}.Run(t)
}
