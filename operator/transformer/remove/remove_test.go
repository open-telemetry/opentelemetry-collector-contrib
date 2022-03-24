// // Copyright The OpenTelemetry Authors
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
package remove

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

type testCase struct {
	name      string
	op        *RemoveOperatorConfig
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			"remove_obj",
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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
			func() *RemoveOperatorConfig {
				cfg := defaultCfg()
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

			remove := op.(*RemoveOperator)
			fake := testutil.NewFakeOutput(t)
			remove.SetOutputs([]operator.Operator{fake})
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
