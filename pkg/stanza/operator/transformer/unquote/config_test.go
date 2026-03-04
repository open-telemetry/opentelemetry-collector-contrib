// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package unquote

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
				Name: "unquote_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("nested")
					return cfg
				}(),
			},
			{
				Name: "unquote_single_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("key")
					return cfg
				}(),
			},
			{
				Name: "unquote_single_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("key")
					return cfg
				}(),
			},
			{
				Name: "unquote_nested_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("one", "two")
					return cfg
				}(),
			},
			{
				Name: "unquote_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("one", "two")
					return cfg
				}(),
			},
			{
				Name: "unquote_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("one", "two")
					return cfg
				}(),
			},
		},
	}.Run(t)
}
