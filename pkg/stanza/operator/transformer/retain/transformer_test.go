// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package retain

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
	expectErr bool
	op        *Config
	input     func() *entry.Entry
	output    func() *entry.Entry
}

func TestBuildAndProcess(t *testing.T) {
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
			"retain_single",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
				}
				return e
			},
		},
		{
			"retain_unrelated_fields",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()

				e.Severity = entry.Debug3
				e.SeverityText = "debug"
				e.Timestamp = time.Unix(1000, 1000)
				e.ObservedTimestamp = time.Unix(2000, 2000)
				e.TraceID = []byte{0x01}
				e.SpanID = []byte{0x01}
				e.TraceFlags = []byte{0x01}
				e.ScopeName = "scope"

				return e
			},
			func() *entry.Entry {
				e := newTestEntry()

				e.Severity = entry.Debug3
				e.SeverityText = "debug"
				e.Timestamp = time.Unix(1000, 1000)
				e.ObservedTimestamp = time.Unix(2000, 2000)
				e.TraceID = []byte{0x01}
				e.SpanID = []byte{0x01}
				e.TraceFlags = []byte{0x01}
				e.ScopeName = "scope"

				e.Body = map[string]any{
					"key": "val",
				}
				return e
			},
		},
		{
			"retain_multi",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("nested2"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"nested2": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested2": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"retain_multilevel",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("foo"))
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("one", "two"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("foo"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("one", "two"))
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("foo"))
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("one", "two"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"foo": "bar",
					"one": map[string]any{
						"two": map[string]any{
							"keepme": 1,
						},
						"deleteme": "yes",
					},
					"hello": "world",
				}
				e.Attributes = map[string]any{
					"foo": "bar",
					"one": map[string]any{
						"two": map[string]any{
							"keepme": 1,
						},
						"deleteme": "yes",
					},
					"hello": "world",
				}
				e.Resource = map[string]any{
					"foo": "bar",
					"one": map[string]any{
						"two": map[string]any{
							"keepme": 1,
						},
						"deleteme": "yes",
					},
					"hello": "world",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"foo": "bar",
					"one": map[string]any{
						"two": map[string]any{
							"keepme": 1,
						},
					},
				}
				e.Attributes = map[string]any{
					"foo": "bar",
					"one": map[string]any{
						"two": map[string]any{
							"keepme": 1,
						},
					},
				}
				e.Resource = map[string]any{
					"foo": "bar",
					"one": map[string]any{
						"two": map[string]any{
							"keepme": 1,
						},
					},
				}
				return e
			},
		},
		{
			"retain_nest",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("nested2"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"nested2": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"nested2": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"retain_nested_value",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("nested2", "nestedkey2"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
					"nested2": map[string]any{
						"nestedkey2": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"nested2": map[string]any{
						"nestedkey2": "nestedval",
					},
				}
				return e
			},
		},
		{
			"retain_single_attribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"key": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"key": "val",
				}
				return e
			},
		},
		{
			"retain_multi_attribute",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key1"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key2"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"key1": "val",
					"key2": "val",
					"key3": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"key1": "val",
					"key2": "val",
				}
				return e
			},
		},
		{
			"retain_single_resource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key": "val",
				}
				return e
			},
		},
		{
			"retain_multi_resource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key1"))
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key2"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key1": "val",
					"key2": "val",
					"key3": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key1": "val",
					"key2": "val",
				}
				return e
			},
		},
		{
			"retain_one_of_each",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewResourceField("key1"))
				cfg.Fields = append(cfg.Fields, entry.NewAttributeField("key3"))
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("key"))
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key1": "val",
					"key2": "val",
				}
				e.Attributes = map[string]any{
					"key3": "val",
					"key4": "val",
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key1": "val",
				}
				e.Attributes = map[string]any{
					"key3": "val",
				}
				e.Body = map[string]any{
					"key": "val",
				}
				return e
			},
		},
		{
			"retain_a_non_existent_key",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Fields = append(cfg.Fields, entry.NewBodyField("aNonExsistentKey"))
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = nil
				return e
			},
		},
	}
	for _, tc := range cases {
		t.Run("BuildandProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = op.ProcessBatch(context.Background(), []*entry.Entry{val})
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
