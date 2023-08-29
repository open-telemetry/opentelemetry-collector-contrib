// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package remove

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

// Test building and processing a given remove config
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

	cases := []testCase{
		{
			"remove_one",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newBodyField("key")
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
				return e
			},
			false,
		},
		{
			"remove_nestedkey",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newBodyField("nested", "nestedkey")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key":    "val",
					"nested": map[string]interface{}{},
				}
				return e
			},
			false,
		},
		{
			"remove_nested_attribute",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newAttributeField("nested", "nestedkey")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"key":    "val",
					"nested": map[string]interface{}{},
				}
				return e
			},
			false,
		},
		{
			"remove_nested_resource",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newResourceField("nested", "nestedkey")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{
					"key":    "val",
					"nested": map[string]interface{}{},
				}
				return e
			},
			false,
		},
		{
			"remove_obj",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newBodyField("nested")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
				}
				return e
			},
			false,
		},
		{
			"remove_single_attribute",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newAttributeField("key")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"key": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{}
				return e
			},
			false,
		},
		{
			"remove_single_resource",
			func() *Config {
				cfg := NewConfig()
				cfg.Field = newResourceField("key")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{
					"key": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{}
				return e
			},
			false,
		},
		{
			"remove_body",
			func() *Config {
				cfg := NewConfig()
				cfg.Field.Field = entry.NewBodyField()
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = nil
				return e
			},
			false,
		},
		{
			"remove_resource",
			func() *Config {
				cfg := NewConfig()
				cfg.Field.allResource = true
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{
					"key": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = nil
				return e
			},
			false,
		},
		{
			"remove_attributes",
			func() *Config {
				cfg := NewConfig()
				cfg.Field.allAttributes = true
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"key": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = nil
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

			remove := op.(*Transformer)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, remove.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = remove.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
