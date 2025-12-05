// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
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
	// Scenario 1: Literal 16-byte slice (original use case)
	// Create a literal getter to ensure the optimization path is taken
	b.Run("literal_bytes", func(b *testing.B) {
		literalBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		literalGetter := makeLiteralIDGetter(literalBytes)
		expr := traceID[any](literalGetter)
		ctx := b.Context()
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, err := expr(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Scenario 2: Literal 32-char hex string (new feature)
	b.Run("literal_hex_string", func(b *testing.B) {
		literalHexString := []byte("0102030405060708090a0b0c0d0e0f10")
		literalGetter := makeLiteralIDGetter(literalHexString)
		expr := traceID[any](literalGetter)
		ctx := b.Context()
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, err := expr(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Scenario 3: Dynamic 16-byte slice (worst case - no optimization)
	b.Run("dynamic_bytes", func(b *testing.B) {
		dynamicBytes := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		dynamicGetter := makeIDGetter(dynamicBytes)
		expr := traceID[any](dynamicGetter)
		ctx := b.Context()
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, err := expr(ctx, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
