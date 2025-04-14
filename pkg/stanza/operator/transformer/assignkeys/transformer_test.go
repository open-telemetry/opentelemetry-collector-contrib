// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package assignkeys

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
	keys := []string{"origin", "sev", "msg", "count", "isBool"}
	now := time.Now()
	newTestEntry := func() *entry.Entry {
		e := entry.New()
		e.ObservedTimestamp = now
		e.Timestamp = time.Unix(1586632809, 0)
		e.Body = []any{"body", "INFO", "started agent", int64(42), true}
		e.Resource = map[string]any{
			"input": []any{"resource", "INFO", "started agent", int64(42), true},
		}
		e.Attributes = map[string]any{
			"input": []any{"attribute", "INFO", "started agent", int64(42), true},
		}
		return e
	}
	cases := []testCase{
		{
			"assign_keys_body",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
				cfg.Keys = keys
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Body = map[string]any{
					"origin": "body",
					"sev":    "INFO",
					"msg":    "started agent",
					"count":  int64(42),
					"isBool": true,
				}
				return e
			},
		},
		{
			"assign_keys_attributes",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewAttributeField("input")
				cfg.Keys = keys
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Attributes = map[string]any{
					"input": map[string]any{
						"origin": "attribute",
						"sev":    "INFO",
						"msg":    "started agent",
						"count":  int64(42),
						"isBool": true,
					},
				}
				return e
			},
		},
		{
			"assign_keys_resources",
			false,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewResourceField("input")
				cfg.Keys = keys
				return cfg
			}(),
			newTestEntry,
			func() *entry.Entry {
				e := newTestEntry()
				e.Resource = map[string]any{
					"input": map[string]any{
						"origin": "resource",
						"sev":    "INFO",
						"msg":    "started agent",
						"count":  int64(42),
						"isBool": true,
					},
				}
				return e
			},
		},
		{
			"assign_keys_missing_keys",
			true,
			func() *Config {
				cfg := NewConfig()
				cfg.Field = entry.NewBodyField()
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

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			if tc.expectErr && err != nil {
				require.Error(t, err)
				return
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
