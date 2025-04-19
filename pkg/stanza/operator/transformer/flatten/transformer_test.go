// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package flatten

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

// test building and processing a given config.
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
		e.Resource = map[string]any{
			"key": "val",
			"nested": map[string]any{
				"nestedkey": "nestedval",
			},
		}
		e.Attributes = map[string]any{
			"key": "val",
			"nested": map[string]any{
				"nestedkey": "nestedval",
			},
		}
		return e
	}
	cases := []testCase{
		{
			"flatten_one_level",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("nested")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key":       "val",
					"nestedkey": "nestedval",
				}
				return e
			},
		},
		{
			"flatten_one_level_multiValue",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("nested")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey1": "nestedval",
						"nestedkey2": "nestedval",
						"nestedkey3": "nestedval",
						"nestedkey4": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key":        "val",
					"nestedkey1": "nestedval",
					"nestedkey2": "nestedval",
					"nestedkey3": "nestedval",
					"nestedkey4": "nestedval",
				}
				return e
			},
		},
		{
			"flatten_second_level",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("nested", "secondlevel")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"secondlevel": map[string]any{
							"nestedkey": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"flatten_second_level_multivalue",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("nested", "secondlevel")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"secondlevel": map[string]any{
							"nestedkey1": "nestedval",
							"nestedkey2": "nestedval",
							"nestedkey3": "nestedval",
							"nestedkey4": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey1": "nestedval",
						"nestedkey2": "nestedval",
						"nestedkey3": "nestedval",
						"nestedkey4": "nestedval",
					},
				}
				return e
			},
		},
		{
			"flatten_move_nest",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("nested")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"secondlevel": map[string]any{
							"nestedkey": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"secondlevel": map[string]any{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"flatten_collision",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("nested")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"key": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"key": "nestedval",
				}
				return e
			},
		},
		{
			"flatten_invalid_field",
			true,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField("invalid")
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"flatten_resource",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewResourceField("nested", "secondlevel")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"secondlevel": map[string]any{
							"nestedkey1": "nestedval",
							"nestedkey2": "nestedval",
							"nestedkey3": "nestedval",
							"nestedkey4": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey1": "nestedval",
						"nestedkey2": "nestedval",
						"nestedkey3": "nestedval",
						"nestedkey4": "nestedval",
					},
				}
				return e
			},
		},
		{
			"flatten_attributes",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("nested", "secondlevel")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"secondlevel": map[string]any{
							"nestedkey1": "nestedval",
							"nestedkey2": "nestedval",
							"nestedkey3": "nestedval",
							"nestedkey4": "nestedval",
						},
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"key": "val",
					"nested": map[string]any{
						"nestedkey1": "nestedval",
						"nestedkey2": "nestedval",
						"nestedkey3": "nestedval",
						"nestedkey4": "nestedval",
					},
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

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			if tc.expectErr && err != nil {
				require.Error(t, err)
				t.SkipNow()
			}
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
