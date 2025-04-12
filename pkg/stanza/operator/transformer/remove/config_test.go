// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package remove

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
				Name: "remove_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newBodyField("nested")
					return cfg
				}(),
			},
			{
				Name: "remove_single_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newAttributeField("key")
					return cfg
				}(),
			},
			{
				Name: "remove_single_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newResourceField("key")
					return cfg
				}(),
			},
			{
				Name: "remove_entire_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field.allResource = true
					return cfg
				}(),
			},
			{
				Name: "remove_entire_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field.Field = entry.NewBodyField()
					return cfg
				}(),
			},
			{
				Name: "remove_entire_attributes",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field.allAttributes = true
					return cfg
				}(),
			},
			{
				Name: "remove_nested_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newBodyField("one", "two")
					return cfg
				}(),
			},
			{
				Name: "remove_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newAttributeField("one", "two")
					return cfg
				}(),
			},
			{
				Name: "remove_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = newResourceField("one", "two")
					return cfg
				}(),
			},
		},
	}.Run(t)
}

func newBodyField(keys ...string) rootableField {
	field := entry.NewBodyField(keys...)
	return rootableField{Field: field}
}

func newResourceField(keys ...string) rootableField {
	field := entry.NewResourceField(keys...)
	return rootableField{Field: field}
}

func newAttributeField(keys ...string) rootableField {
	field := entry.NewAttributeField(keys...)
	return rootableField{Field: field}
}
