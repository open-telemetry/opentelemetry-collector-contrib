// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package move

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type processTestCase struct {
	name      string
	expectErr bool
	op        *Config
	input     func() *entry.Entry
	output    func() *entry.Entry
}

func TestProcessAndBuild(t *testing.T) {
	now := time.Now()
	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = time.Unix(1586632809, 0)
		e.Body = map[string]interface{}{
			"key": "val",
			"nested": map[string]interface{}{
				"nestedkey": "nestedval",
			},
		}
		return e
	}

	cases := []processTestCase{
		{
			"MoveBodyToBody",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("new")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"new": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"MoveBodyToAttribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewAttributeField("new")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				e.Attributes = map[string]interface{}{"new": "val"}
				return e
			},
		},
		{
			"MoveAttributeToBody",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewBodyField("new")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"new": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"new": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				e.Attributes = map[string]interface{}{}
				return e
			},
		},
		{
			"MoveAttributeToResource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewResourceField("new")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"new": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{"new": "val"}
				e.Attributes = map[string]interface{}{}
				return e
			},
		},
		{
			"MoveBracketedAttributeToResource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewAttributeField("dotted.field.name")
				cfg.To = entry.NewResourceField("new")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"dotted.field.name": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{"new": "val"}
				e.Attributes = map[string]interface{}{}
				return e
			},
		},
		{
			"MoveBracketedAttributeToBracketedResource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewAttributeField("dotted.field.name")
				cfg.To = entry.NewResourceField("dotted.field.name")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"dotted.field.name": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{"dotted.field.name": "val"}
				e.Attributes = map[string]interface{}{}
				return e
			},
		},
		{
			"MoveAttributeToBracketedResource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewAttributeField("new")
				cfg.To = entry.NewResourceField("dotted.field.name")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"new": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{"dotted.field.name": "val"}
				e.Attributes = map[string]interface{}{}
				return e
			},
		},
		{
			"MoveResourceToAttribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewResourceField("new")
				cfg.To = entry.NewAttributeField("new")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{"new": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{}
				e.Attributes = map[string]interface{}{"new": "val"}
				return e
			},
		},
		{
			"MoveNest",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewBodyField("NewNested")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"NewNested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"MoveFromNestedObj",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested", "nestedkey")
				cfg.To = entry.NewBodyField("unnestedkey")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key":         "val",
					"nested":      map[string]interface{}{},
					"unnestedkey": "nestedval",
				}
				return e
			},
		},
		{
			"MoveToNestedObj",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("newnestedkey")
				cfg.To = entry.NewBodyField("nested", "newnestedkey")

				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
					"newnestedkey": "nestedval",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey":    "nestedval",
						"newnestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"MoveDoubleNestedObj",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested", "nested2")
				cfg.To = entry.NewBodyField("nested2")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
						"nested2": map[string]interface{}{
							"nestedkey": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
					"nested2": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"MoveNestToResource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewResourceField("NewNested")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
				}
				e.Resource = map[string]interface{}{
					"NewNested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"MoveNestToAttribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewAttributeField("NewNested")

				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
				}
				e.Attributes = map[string]interface{}{
					"NewNested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"MoveNestedBodyStringToNestedAttribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested", "nestedkey")
				cfg.To = entry.NewAttributeField("one", "two", "three")

				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key":    "val",
					"nested": map[string]interface{}{},
				}
				e.Attributes = map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
							"three": "nestedval",
						},
					},
				}
				return e
			},
		},
		{
			"MoveAttributeTodBody",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewAttributeField("one", "two", "three")
				cfg.To = entry.NewBodyField()

				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
							"three": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = "nestedval"
				e.Attributes = map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{},
					},
				}
				return e
			},
		},
		{
			"ReplaceBodyObj",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("wrapper")
				cfg.To = entry.NewBodyField()
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"wrapper": map[string]interface{}{
						"key": "val",
						"nested": map[string]interface{}{
							"nestedkey": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"ReplaceBodyString",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField()
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = "val"
				return e
			},
		},
		{
			"MergeObjToBody",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewBodyField()
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key":       "val",
					"nestedkey": "nestedval",
				}
				return e
			},
		},
	}
	for _, tc := range cases {
		t.Run("BuildandProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			move := op.(*Transformer)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, move.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = move.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
