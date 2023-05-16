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

package unquote

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
			name: "not_quoted",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "val"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "val"
				return e
			},
			expectErr: "invalid syntax",
		},
		{
			name: "double_quoted",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "\"val\""
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "val"
				return e
			},
		},
		{
			name: "back_quoted",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "`val`"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "val"
				return e
			},
		},
		{
			name: "single_quoted_single_char",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "'v'" // Only allowed with single character
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "v"
				return e
			},
		},
		{
			name: "single_quoted_multi_char",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "'val'"
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Body = "'val'"
				return e
			},
			expectErr: "invalid syntax",
		},
		{
			name: "invalid_type",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
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
			expectErr: "type int cannot be unquoted",
		},
		{
			name: "attribute",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("foo")
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"foo": "\"val\"",
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"foo": "val",
				}
				return e
			},
		},
		{
			name: "missing_field",
			cfg: func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("bar")
				return cfg
			}(),
			input: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"foo": "\"val\"",
				}
				return e
			},
			output: func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]interface{}{
					"foo": "\"val\"",
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
			op, err := cfg.Build(testutil.Logger(t))
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
