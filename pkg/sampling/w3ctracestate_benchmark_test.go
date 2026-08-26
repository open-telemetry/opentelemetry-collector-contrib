// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"strings"
	"testing"
)

func BenchmarkNewW3CTraceState(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{
			name:  "Empty",
			input: "",
		},
		{
			name:  "OTelWithExtraFields",
			input: "ot=th:c;rv:d29d6a7215ced0;pn:abc",
		},
		{
			name:  "OTelPlusMultipleVendors",
			input: "ot=th:c;rv:d29d6a7215ced0;pn:abc,zz=vendorcontent,aa=value1,bb=value2",
		},
		{
			name:  "LongValue",
			input: "ot=th:c,vendor=" + strings.Repeat("x", 200),
		},
		{
			name:  "MaxPairs",
			input: generateManyPairs(32),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = NewW3CTraceState(bm.input)
			}
		})
	}
}

// generateManyPairs creates a tracestate string with the specified number of key-value pairs.
func generateManyPairs(n int) string {
	var sb strings.Builder
	sb.WriteString("ot=th:c")
	for i := 1; i < n; i++ {
		sb.WriteString(",v")
		sb.WriteString(strings.Repeat("x", i%10))
		sb.WriteString("=val")
	}
	return sb.String()
}

func BenchmarkNewW3CTraceStateParallel(b *testing.B) {
	input := "ot=th:c;rv:d29d6a7215ced0;pn:abc,zz=vendorcontent"

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = NewW3CTraceState(input)
		}
	})
}

// =============================================================================
// Benchmark: Hand-written Validator vs Regex
// =============================================================================

func BenchmarkW3CTracestateValidation(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"Simple", "ot=th:c"},
		{"Complex", "ot=th:c;rv:d29d6a7215ced0;pn:abc,zz=vendorcontent,aa=value1,bb=value2"},
		{"LongValue", "ot=th:c,vendor=" + strings.Repeat("x", 200)},
		{"MaxPairs", generateManyPairs(32)},
	}

	for _, bm := range benchmarks {
		b.Run("Regex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = w3cTracestateRe.MatchString(bm.input)
			}
		})

		b.Run("NoRegex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = isValidW3CTraceState(bm.input)
			}
		})
	}
}
