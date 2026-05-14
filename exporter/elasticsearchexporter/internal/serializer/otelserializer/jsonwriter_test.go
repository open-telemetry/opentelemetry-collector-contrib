// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFloat64Val verifies the behavior of JSONWriter.float64Val.
// It verifies usual behavior and in addition verifies that a
// buffer corruption vulnerability happening when values requires
// the addition of ".0".
func TestFloat64Val(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		// Values with exponents that need ".0" insertion — these triggered the
		// AvailableBuffer corruption bug when the buffer has prior content.
		{name: "large exponent", input: 1e+20, expected: "1.0e+20"},
		{name: "very large exponent", input: 1e+100, expected: "1.0e+100"},
		{name: "negative value large exponent", input: -1e+20, expected: "-1.0e+20"},
		{name: "5e+18", input: 5e+18, expected: "5.0e+18"},

		// Values that already have a decimal point
		{name: "decimal value", input: 1.5, expected: "1.5"},
		{name: "small exponent with decimal", input: 1e-20, expected: "1.0e-20"},
		{name: "trailing zero", input: 42.0, expected: "42.0"},
		{name: "negative decimal", input: -3.14, expected: "-3.14"},

		// Integer-like values (no decimal, no exponent)
		{name: "zero", input: 0.0, expected: "0.0"},
		{name: "one", input: 1.0, expected: "1.0"},
		{name: "negative one", input: -1.0, expected: "-1.0"},

		// Special values
		{name: "positive infinity", input: math.Inf(1), expected: "null"},
		{name: "negative infinity", input: math.Inf(-1), expected: "null"},
		{name: "NaN", input: math.NaN(), expected: "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pre-grow and write prior content so AvailableBuffer returns a
			// slice that shares the buffer's internal backing array. Without
			// this, the corruption does not manifest because the append in
			// strconv.AppendFloat allocates a fresh slice.
			var buf bytes.Buffer
			buf.Grow(64)
			buf.WriteString(`{"v":`)

			w := newJSONWriter(&buf)
			w.float64Val(tt.input)
			buf.WriteByte('}')

			raw := buf.String()
			// Verify the resulting buffer is valid JSON.
			require.True(t, json.Valid(buf.Bytes()), "invalid JSON produced: %s", raw)

			// Strip wrapping JSON to match the expected format.
			got := raw[5 : len(raw)-1]
			assert.Equal(t, tt.expected, got)
		})
	}
}

func BenchmarkFloat64Val(b *testing.B) {
	benchmarks := []struct {
		name  string
		input float64
	}{
		{name: "integer", input: 42},
		{name: "integer with exponent", input: 1e+20},
		{name: "decimal", input: 3.14},
		{name: "decimal with exponent", input: 3.14e+20},
	}

	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			var buf bytes.Buffer
			buf.Grow(64)
			w := newJSONWriter(&buf)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				buf.Reset()
				w.float64Val(bb.input)
			}
		})
	}
}
