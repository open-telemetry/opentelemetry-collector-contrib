// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sanitizeutf8

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type processTestCase struct {
	name   string
	op     *Config
	input  func() *entry.Entry
	output func() *entry.Entry
}

func TestSanitizeUTF8(t *testing.T) {
	now := time.Now()
	cases := []processTestCase{
		{
			name: "normal_string_body",
			op:   newCfgWithField("body"),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is a normal string"
				e.ObservedTimestamp = now
				return e
			},
			output: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is a normal string"
				e.ObservedTimestamp = now
				return e
			},
		},
		{
			name: "invalid_utf8_body",
			op:   newCfgWithField("body"),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is an invalid utf8 string \xfe"
				e.ObservedTimestamp = now
				return e
			},
			output: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is an invalid utf8 string \uFFFD"
				e.ObservedTimestamp = now
				return e
			},
		},
		{
			name: "consecutive_invalid_utf8_body",
			op:   newCfgWithField("body"),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is an invalid utf8 string \xfe\xfe"
				e.ObservedTimestamp = now
				return e
			},
			output: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is an invalid utf8 string \uFFFD"
				e.ObservedTimestamp = now
				return e
			},
		},
		{
			name: "unconsecutive_invalid_utf8_body",
			op:   newCfgWithField("body"),
			input: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is an invalid utf8 string \xfe and another \xfe"
				e.ObservedTimestamp = now
				return e
			},
			output: func() *entry.Entry {
				e := entry.New()
				e.Body = "This is an invalid utf8 string \uFFFD and another \uFFFD"
				e.ObservedTimestamp = now
				return e
			},
		},
		{
			name: "normal_string_attribute",
			op:   newCfgWithField("attributes.foo"),
			input: func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]any{
					"foo": "This is a normal string",
				}
				e.ObservedTimestamp = now
				return e
			},
			output: func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]any{
					"foo": "This is a normal string",
				}
				e.ObservedTimestamp = now
				return e
			},
		},
		{
			name: "invalid_utf8_attribute",
			op:   newCfgWithField("attributes.foo"),
			input: func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]any{
					"foo": "This is an invalid utf8 string \xfe",
				}
				e.ObservedTimestamp = now
				return e
			},
			output: func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]any{
					"foo": "This is an invalid utf8 string \uFFFD",
				}
				e.ObservedTimestamp = now
				return e
			},
		},
	}
	for _, tc := range cases {
		t.Run("BuildandProcess/"+tc.name, func(t *testing.T) {
			cfg := tc.op
			cfg.OutputIDs = []string{"fake"}
			cfg.OnError = "drop"
			op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
			require.NoError(t, err)

			sanitizeUTF8 := op.(*Transformer)
			fake := testutil.NewFakeOutput(t)
			require.NoError(t, sanitizeUTF8.SetOutputs([]operator.Operator{fake}))
			val := tc.input()
			err = sanitizeUTF8.Process(t.Context(), val)
			require.NoError(t, err)
			fake.ExpectEntry(t, tc.output())
		})
	}
}

func newCfgWithField(field string) *Config {
	entryField, err := entry.NewField(field)
	if err != nil {
		panic(fmt.Sprintf("failed to create field %s: %v", field, err))
	}
	config := NewConfigWithID("sanitize_utf8/test")
	config.Field = entryField
	return config
}
