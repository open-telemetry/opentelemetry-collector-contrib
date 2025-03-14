// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexreplace

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type testCase struct {
	name      string
	cfg       *Config
	input     func() *entry.Entry
	output    func() *entry.Entry
	expectErr string
}

func TestBuildAndProcess(t *testing.T) {
	now := time.Now()
	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = time.Unix(1586632809, 0)
		return e
	}

	cases := []testCase{
		{
			name: "simple_regex_replace",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.Regex = "_+"
				cfg.ReplaceWith = ","
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "a__b__c"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "a,b,c"
				return e
			},
		},
		{
			name: "group_regex_replace",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.Regex = "{(.)}"
				cfg.ReplaceWith = "${1}"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "{a}{b}{c}"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "abc"
				return e
			},
		},
		{
			name: "no_match",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "asdf"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "asdf"
				return e
			},
		},
		{
			name: "no_color_code",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "\x1b[m"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = ""
				return e
			},
		},
		{
			name: "single_color_code",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "\x1b[31m"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = ""
				return e
			},
		},
		{
			name: "multiple_color_codes",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "\x1b[31;1;4m"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = ""
				return e
			},
		},
		{
			name: "multiple_escapes",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "\x1b[31mred\x1b[0m"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "red"
				return e
			},
		},
		{
			name: "preserve_other_text",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "start \x1b[31mred\x1b[0m end"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "start red end"
				return e
			},
		},
		{
			name: "nonstandard_uppercase_m",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "\x1b[31M"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = ""
				return e
			},
		},
		{
			name: "invalid_type",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = 123
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = 123
				return e
			},
			expectErr: "type int cannot be handled",
		},
		{
			name: "attribute",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("foo")
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"foo": "\x1b[31mred\x1b[0m",
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"foo": "red",
				}
				return e
			},
		},
		{
			name: "missing_field",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("bar")
				cfg.RegexName = "ansi_control_sequences"
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"foo": "\x1b[31mred\x1b[0m",
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"foo": "\x1b[31mred\x1b[0m",
				}
				return e
			},
		},
	}
	for _, tc := range cases {
		t.Run("BuildandProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.cfg
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "send"
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			unqouteOp := op.(*Transformer)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, unqouteOp.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = unqouteOp.Process(context.Background(), val)
			if tc.expectErr != "" {
				require.Equal(t, tc.expectErr, err.Error())
			} else {
				require.NoError(t, err)
			}

			// Expect entry to pass through even if error, due to OnError = "send"
			fake.ExpectEntry(t, tc.output())
		})
	}
}
