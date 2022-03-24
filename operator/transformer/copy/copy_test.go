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
package copy

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
	expectErr bool
	op        *CopyOperatorConfig
	input     func() *entry.Entry
	output    func() *entry.Entry
}

// Test building and processing a CopyOperatorConfig
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
			"body_to_body",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
					"key2": "val",
				}
				return e
			},
		},
		{
			"nested_to_body",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested", "nestedkey")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
					"key2": "nestedval",
				}
				return e
			},
		},
		{
			"body_to_nested",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("nested", "key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
						"key2":      "val",
					},
				}
				return e
			},
		},
		{
			"body_to_attribute",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewAttributeField("key2")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
				}
				e.Attributes = map[string]interface{}{"key2": "val"}
				return e
			},
		},
		{
			"attribute_to_body",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewBodyField("key2")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"key": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key": "val",
					"nested": map[string]interface{}{
						"nestedkey": "nestedval",
					},
					"key2": "val",
				}
				e.Attributes = map[string]interface{}{"key": "val"}
				return e
			},
		},
		{
			"attribute_to_resource",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("key")
				cfg.To = entry.NewResourceField("key2")
				return cfg
			}(),
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"key": "val"}
				return e
			},
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{"key": "val"}
				e.Resource = map[string]interface{}{"key2": "val"}
				return e
			},
		},
		{
			"overwrite",
			false,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewBodyField("nested")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]interface{}{
					"key":    "val",
					"nested": "val",
				}
				return e
			},
		},
		{
			"invalid_copy_obj_to_resource",
			true,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewResourceField("invalid")
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"invalid_copy_obj_to_attributes",
			true,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("nested")
				cfg.To = entry.NewAttributeField("invalid")
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"invalid_key",
			true,
			func() *CopyOperatorConfig {
				cfg := defaultCfg()
				cfg.From = entry.NewAttributeField("nonexistentkey")
				cfg.To = entry.NewResourceField("key2")
				return cfg
			}(),
			newTestEntry,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run("BuildAndProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			copy := op.(*CopyOperator)
			fake := testutil.NewFakeOutput(t)
			copy.SetOutputs([]operator.Operator{fake})
			val := tc.input()
			err = copy.Process(context.Background(), val)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fake.ExpectEntry(t, tc.output())
			}
		})
	}
}
