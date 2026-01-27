// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_traceID(t *testing.T) {
	runIDSuccessTests(t, traceID[any], []idSuccessTestCase{
		{
			name:  "create trace id from 16 bytes",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			want:  pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
		{
			name:  "create trace id from 32 hex chars",
			value: []byte("0102030405060708090a0b0c0d0e0f10"),
			want:  pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
		},
	})
}

func Test_traceID_validation(t *testing.T) {
	runIDErrorTests(t, traceID[any], traceIDFuncName, []idErrorTestCase{
		{
			name:  "byte slice less than 16 (15)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			err:   errIDInvalidLength,
		},
		{
			name:  "byte slice longer than 16 (17)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
			err:   errIDInvalidLength,
		},
		{
			name:  "byte slice longer than 32 (33)",
			value: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33},
			err:   errIDInvalidLength,
		},
		{
			name:  "invalid hex string",
			value: []byte("ZZ02030405060708090a0b0c0d0e0f10"),
			err:   errIDHexDecode,
		},
	})
}

func BenchmarkTraceID(b *testing.B) {
	// Create a span to set the trace ID on (shared across benchmarks)
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	testCases := []struct {
		name      string
		data      []byte
		isLiteral bool
	}{
		{
			name:      "literal_bytes_get_and_set",
			data:      []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			isLiteral: true,
		},
		{
			name:      "literal_hex_string_get_and_set",
			data:      []byte("0102030405060708090a0b0c0d0e0f10"),
			isLiteral: true,
		},
		{
			name:      "dynamic_bytes_get_and_set",
			data:      []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			isLiteral: false,
		},
		{
			name:      "dynamic_hex_string_get_and_set",
			data:      []byte("0102030405060708090a0b0c0d0e0f10"),
			isLiteral: false,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var getter ottl.ByteSliceLikeGetter[any]
			var err error

			if tc.isLiteral {
				getter, err = ottl.NewTestingLiteralGetter(true, makeIDGetter(tc.data))
				require.NoError(b, err)
			} else {
				getter = makeIDGetter(tc.data)
			}

			expr, err := traceID[any](getter)
			require.NoError(b, err)

			ctx := b.Context()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := expr(ctx, nil)
				if err != nil {
					b.Fatal(err)
				}
				span.SetTraceID(result.(pcommon.TraceID))
			}
		})
	}
}
