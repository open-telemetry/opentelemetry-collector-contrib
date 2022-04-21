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

// Test building and processing a Config
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
			func() *Config {
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
			func() *Config {
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
			func() *Config {
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
			func() *Config {
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
			"body_to_nested_attribute",
			false,
			func() *Config {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField()
				cfg.To = entry.NewAttributeField("one", "two")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
							"key": "val",
							"nested": map[string]interface{}{
								"nestedkey": "nestedval",
							},
						},
					},
				}
				return e
			},
		},
		{
			"body_to_nested_resource",
			false,
			func() *Config {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField()
				cfg.To = entry.NewResourceField("one", "two")
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]interface{}{
					"one": map[string]interface{}{
						"two": map[string]interface{}{
							"key": "val",
							"nested": map[string]interface{}{
								"nestedkey": "nestedval",
							},
						},
					},
				}
				return e
			},
		},
		{
			"attribute_to_body",
			false,
			func() *Config {
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
			func() *Config {
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
			func() *Config {
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
			"invalid_copy_to_attribute_root",
			true,
			func() *Config {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewAttributeField()
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"invalid_copy_to_resource_root",
			true,
			func() *Config {
				cfg := defaultCfg()
				cfg.From = entry.NewBodyField("key")
				cfg.To = entry.NewResourceField()
				return cfg
			}(),
			newTestEntry,
			nil,
		},
		{
			"invalid_key",
			true,
			func() *Config {
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

			copy := op.(*Transformer)
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
