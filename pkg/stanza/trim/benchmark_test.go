// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim

import (
	"bufio"
	"strings"
	"testing"
)

// BenchmarkTrimFuncs tests the core trimming logic (Leading, Trailing, Whitespace, Nop)
func BenchmarkTrimFuncs(b *testing.B) {
	inputs := []struct {
		name string
		data []byte
	}{
		{
			name: "small clean",
			data: []byte("simple_log_line"),
		},
		{
			name: "small dirty",
			data: []byte("   \t\r\n simple_log_line \t\r\n   "),
		},
		{
			name: "all space",
			data: []byte("   \t\r\n   "),
		},
		{
			name: "large dirty",
			data: []byte(strings.Repeat("   \t content \r\n ", 100)),
		},
	}

	funcs := []struct {
		name string
		fn   Func
	}{
		{
			name: "leading",
			fn:   Leading,
		},
		{
			name: "trailing",
			fn:   Trailing,
		},
		{
			name: "whitespace",
			fn:   Whitespace,
		},
		{
			name: "nop",
			fn:   Nop,
		},
	}

	for _, f := range funcs {
		for _, input := range inputs {
			b.Run(f.name+"/"+input.name, func(b *testing.B) {
				b.ReportAllocs()
				data := input.data

				for b.Loop() {
					_ = f.fn(data)
				}
			})
		}
	}
}

// BenchmarkWithFunc tests the overhead of wrapping a SplitFunc
func BenchmarkWithFunc(b *testing.B) {
	data := []byte("line1\nline2\nline3\n")

	tests := []struct {
		name string
		fn   bufio.SplitFunc
	}{
		{
			name: "baseline scanlines",
			fn:   bufio.ScanLines,
		},
		{
			name: "with whitespace",
			fn:   WithFunc(bufio.ScanLines, Whitespace),
		},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			split := tc.fn

			for b.Loop() {
				_, _, _ = split(data, false)
			}
		})
	}
}

// BenchmarkToLength tests the truncation logic
func BenchmarkToLength(b *testing.B) {
	tests := []struct {
		name  string
		data  []byte
		limit int
	}{
		{
			name:  "under limit",
			data:  []byte("short"),
			limit: 10,
		},
		{
			name:  "over limit",
			data:  []byte(strings.Repeat("long", 100)),
			limit: 10,
		},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			// Initialize the split function with the specific limit
			split := ToLength(bufio.ScanLines, tc.limit)
			data := tc.data

			for b.Loop() {
				_, _, _ = split(data, false)
			}
		})
	}
}
