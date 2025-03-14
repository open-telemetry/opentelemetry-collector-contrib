// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package regexreplace

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

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

type invalidConfigCase struct {
	name      string
	cfg       *Config
	expectErr string
}

func TestInvalidConfig(t *testing.T) {
	cases := []invalidConfigCase{
		{
			name: "neither_regex_nor_regexname",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = ""
				cfg.Regex = ""
				return cfg
			}(),
			expectErr: "either regex or regex_name must be set",
		},
		{
			name: "both_regex_and_regexname",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				cfg.Regex = ".*"
				return cfg
			}(),
			expectErr: "either regex or regex_name must be set",
		},
		{
			name: "unknown_regex_name",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "i_do_not_exist"
				return cfg
			}(),
			expectErr: "regex_name i_do_not_exist is unknown",
		},
		{
			name: "invalid_regex",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.Regex = ")"
				return cfg
			}(),
			expectErr: "error parsing regexp: unexpected ): `)`",
		},
	}
	for _, tc := range cases {
		t.Run("InvalidConfig/"+tc.name, func(t *testing.T) {
			cfg := tc.cfg
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "send"
			set := componenttest.NewNopTelemetrySettings()
			_, err := cfg.Build(set)
			require.Equal(t, tc.expectErr, err.Error())
		})
	}
}
