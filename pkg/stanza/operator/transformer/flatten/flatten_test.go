// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package flatten

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
	expectErr bool
	op        *FlattenOperatorConfig
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
			"flatten_one_level",
			false,
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"nested"},
				}
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
		{
			"flatten_one_level_multiValue",
			false,
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"nested"},
				}
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
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
				e.Body = map[string]interface{}{
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
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"nested", "secondlevel"},
				}
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"secondlevel": map[string]interface{}{
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
			"flatten_second_level_multivalue",
			false,
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"nested", "secondlevel"},
				}
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"secondlevel": map[string]interface{}{
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
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
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
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"nested"},
				}
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"secondlevel": map[string]interface{}{
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
					"secondlevel": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				return e
			},
		},
		{
			"flatten_collision",
			false,
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"nested"},
				}
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"key": "nestedval",
					},
				}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "nestedval",
				}
				return e
			},
		},
		{
			"flatten_invalid_field",
			true,
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"invalid"},
				}
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"flatten_resource",
			true,
			func() *FlattenOperatorConfig {
				cfg := defaultCfg()
				cfg.Field = entry.BodyField{
					Keys: []string{"resource", "invalid"},
				}
				return cfg
			}(),
			newTestEntry,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run("BuildandProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"

			op, err := cfg.Build(testutil.Logger(t))
			if tc.expectErr && err != nil {
				require.Error(t, err)
				t.SkipNow()
			}
			require.NoError(t, err)

			flatten := op.(*FlattenOperator)
			fake := testutil.NewFakeOutput(t)
			flatten.SetOutputs([]operator.Operator{fake})
			val := tc.input()
			err = flatten.Process(context.Background(), val)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
