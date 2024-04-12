// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package add

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type testCase struct {
	name      string
	op        *Config
	input     func() *entry.Entry
	output    func() *entry.Entry
	expectErr bool
}

func TestProcessAndBuild(t *testing.T) {
	t.Setenv("TEST_EXPR_STRING_ENV", "val")
	now := time.Now()
	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = time.Unix(1586632809, 0)
		e.Body = map[string]any{
			"key": "val",
			"nested": map[string]any{
				"nestedkey": "nestedval",
			},
		}
		return e
	}

	cases := []testCase{
		{
			"add_value",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("new")
				cfg.Value = "randomMessage"
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body.(map[string]any)["new"] = "randomMessage"
				return e
			},
			false,
		},
		{
			"add_expr",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("new")
				cfg.Value = `EXPR(body.key + "_suffix")`
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body.(map[string]any)["new"] = "val_suffix"
				return e
			},
			false,
		},
		{
			"add_nest",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("new")
				cfg.Value = map[any]any{
					"nest": map[any]any{
						"key": "val",
					},
				}
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"new": map[any]any{
						"nest": map[any]any{
							"key": "val",
						},
					},
				}
				return e
			},
			false,
		},
		{
			"add_attribute",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("new")
				cfg.Value = "some.attribute"
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{"new": "some.attribute"}
				return e
			},
			false,
		},
		{
			"add_resource",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewResourceField("new")
				cfg.Value = "some.resource"
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{"new": "some.resource"}
				return e
			},
			false,
		},
		{
			"add_resource_expr",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewResourceField("new")
				cfg.Value = `EXPR(body.key + "_suffix")`
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{"new": "val_suffix"}
				return e
			},
			false,
		},
		{
			"add_int_to_body",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("new")
				cfg.Value = 1
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"new": 1,
				}
				return e
			},
			false,
		},
		{
			"add_array_to_body",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("new")
				cfg.Value = []int{1, 2, 3, 4}
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"new": []int{1, 2, 3, 4},
				}
				return e
			},
			false,
		},
		{
			"overwrite",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("key")
				cfg.Value = []int{1, 2, 3, 4}
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": []int{1, 2, 3, 4},
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
			false,
		},
		{
			"add_int_to_resource",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewResourceField("new")
				cfg.Value = 1
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"new": 1,
				}
				return e
			},
			false,
		},
		{
			"add_int_to_attributes",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("new")
				cfg.Value = 1
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"new": 1,
				}
				return e
			},
			false,
		},
		{
			"add_nested_to_attributes",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("one", "two")
				cfg.Value = map[string]any{
					"new": 1,
				}
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"new": 1,
						},
					},
				}
				return e
			},
			false,
		},
		{
			"add_nested_to_resource",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewResourceField("one", "two")
				cfg.Value = map[string]any{
					"new": 1,
				}
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"one": map[string]any{
						"two": map[string]any{
							"new": 1,
						},
					},
				}
				return e
			},
			false,
		},
		{
			"add_expr",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("fookey")
				cfg.Value = "EXPR('foo_' + body.key)"
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"fookey": "foo_val",
				}
				return e
			},
			false,
		},
		{
			"add_expr_env",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("fookey")
				cfg.Value = "EXPR('foo_' + env('TEST_EXPR_STRING_ENV'))"
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"fookey": "foo_val",
				}
				return e
			},
			false,
		},
	}
	for _, tc := range cases {
		t.Run("BuildandProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			add := op.(*Transformer)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, add.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = add.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
