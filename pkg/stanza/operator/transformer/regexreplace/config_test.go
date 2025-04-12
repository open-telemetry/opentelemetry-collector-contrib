// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package regexreplace

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
				Name: "some_regex_replace",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("nested")
					cfg.Regex = "a"
					cfg.ReplaceWith = "b"
					return cfg
				}(),
			},
			{
				Name: "ansi_control_sequences_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("nested")
					cfg.RegexName = "ansi_control_sequences"
					return cfg
				}(),
			},
			{
				Name: "ansi_control_sequences_single_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("key")
					cfg.RegexName = "ansi_control_sequences"
					return cfg
				}(),
			},
			{
				Name: "ansi_control_sequences_single_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("key")
					cfg.RegexName = "ansi_control_sequences"
					return cfg
				}(),
			},
			{
				Name: "ansi_control_sequences_nested_body",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField("one", "two")
					cfg.RegexName = "ansi_control_sequences"
					return cfg
				}(),
			},
			{
				Name: "ansi_control_sequences_nested_attribute",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewAttributeField("one", "two")
					cfg.RegexName = "ansi_control_sequences"
					return cfg
				}(),
			},
			{
				Name: "ansi_control_sequences_nested_resource",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewResourceField("one", "two")
					cfg.RegexName = "ansi_control_sequences"
					return cfg
				}(),
			},
		},
	}.Run(t)
}

func TestBuild(t *testing.T) {
	operatortest.ConfigBuilderTests{
		Tests: []operatortest.ConfigBuilderTest{
			{
				Name: "neither_regex_nor_regexname",
				Cfg: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					cfg.RegexName = ""
					cfg.Regex = ""
					return cfg
				}(),
				BuildError: "either regex or regex_name must be set",
			},
			{
				Name: "both_regex_and_regexname",
				Cfg: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					cfg.RegexName = "ansi_control_sequences"
					cfg.Regex = ".*"
					return cfg
				}(),
				BuildError: "either regex or regex_name must be set",
			},
			{
				Name: "unknown_regex_name",
				Cfg: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					cfg.RegexName = "i_do_not_exist"
					return cfg
				}(),
				BuildError: "regex_name i_do_not_exist is unknown",
			},
			{
				Name: "invalid_regex",
				Cfg: func() *Config {
					cfg := NewConfig()
					cfg.Field = entry.NewBodyField()
					cfg.Regex = ")"
					return cfg
				}(),
				BuildError: "error parsing regexp: unexpected ): `)`",
			},
		},
	}.Run(t)
}
