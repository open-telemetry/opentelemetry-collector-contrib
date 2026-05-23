// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"regexp"
	"strings"
	"testing"
)

// Regex patterns for OTel tracestate validation.
// Kept here for benchmark comparison purposes.
//
// OTel tracestate syntax:
// chr        = ucalpha / lcalpha / DIGIT / "." / "_" / "-"
// ucalpha    = %x41-5A ; A-Z
// lcalpha    = %x61-7A ; a-z
// key        = lcalpha *(lcalpha / DIGIT )
// value      = *(chr)
// list-member = key ":" value
// list        = list-member *( ";" list-member )
const (
	otelKeyRegexp             = lcAlphaRegexp + lcAlphanumRegexp + `*`
	otelValueRegexp           = `[a-zA-Z0-9._\-]*`
	otelMemberRegexp          = `(?:` + otelKeyRegexp + `:` + otelValueRegexp + `)`
	otelSemicolonMemberRegexp = `(?:` + `;` + otelMemberRegexp + `)`
	otelTracestateRegexp      = `^` + otelMemberRegexp + otelSemicolonMemberRegexp + `*$`
)

var otelTracestateRe = regexp.MustCompile(otelTracestateRegexp)

func BenchmarkNewOpenTelemetryTraceState(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{
			name:  "Simple",
			input: "th:c",
		},
		{
			name:  "WithRValue",
			input: "th:c;rv:d29d6a7215ced0",
		},
		{
			name:  "WithExtraFields",
			input: "th:c;rv:d29d6a7215ced0;pn:abc",
		},
		{
			name:  "MultipleExtras",
			input: "th:c;rv:d29d6a7215ced0;pn:abc;extra1:value1;extra2:value2",
		},
		{
			name:  "LongValue",
			input: "th:c;extra:" + strings.Repeat("x", 200),
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = NewOpenTelemetryTraceState(bm.input)
			}
		})
	}
}

func BenchmarkNewOpenTelemetryTraceStateParallel(b *testing.B) {
	input := "th:c;rv:d29d6a7215ced0;pn:abc"

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = NewOpenTelemetryTraceState(input)
		}
	})
}

// =============================================================================
// Benchmark: Hand-written Validator vs Regex
// =============================================================================

func BenchmarkOTelTracestateValidation(b *testing.B) {
	benchmarks := []struct {
		name  string
		input string
	}{
		{"Simple", "th:c"},
		{"WithRValue", "th:c;rv:d29d6a7215ced0"},
		{"Complex", "th:c;rv:d29d6a7215ced0;pn:abc;extra1:value1;extra2:value2"},
		{"LongValue", "th:c;extra:" + strings.Repeat("x", 200)},
	}

	for _, bm := range benchmarks {
		b.Run("Regex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = otelTracestateRe.MatchString(bm.input)
			}
		})

		b.Run("NoRegex/"+bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = isValidOTelTraceState(bm.input)
			}
		})
	}
}
