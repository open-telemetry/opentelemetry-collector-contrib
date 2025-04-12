// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
)

func TestTrim(t *testing.T) {
	// Test all permutations of trimming whitespace
	testCases := []struct {
		name             string
		preserveLeading  bool
		preserveTrailing bool
		input            []byte
		expect           []byte
	}{
		{
			name:             "preserve both",
			preserveLeading:  true,
			preserveTrailing: true,
			input:            []byte("  hello world  "),
			expect:           []byte("  hello world  "),
		},
		{
			name:             "preserve leading",
			preserveLeading:  true,
			preserveTrailing: false,
			input:            []byte("  hello world  "),
			expect:           []byte("  hello world"),
		},
		{
			name:             "preserve trailing",
			preserveLeading:  false,
			preserveTrailing: true,
			input:            []byte("  hello world  "),
			expect:           []byte("hello world  "),
		},
		{
			name:             "preserve neither",
			preserveLeading:  false,
			preserveTrailing: false,
			input:            []byte("  hello world  "),
			expect:           []byte("hello world"),
		},
		{
			name:             "trim leading returns nil when given nil",
			preserveLeading:  false,
			preserveTrailing: true,
			input:            nil,
			expect:           nil,
		},
		{
			name:             "trim leading returns []byte when given []byte",
			preserveLeading:  false,
			preserveTrailing: true,
			input:            []byte{},
			expect:           []byte{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			trimFunc := Config{
				PreserveLeading:  tc.preserveLeading,
				PreserveTrailing: tc.preserveTrailing,
			}.Func()
			assert.Equal(t, tc.expect, trimFunc(tc.input))
		})
	}
}

func TestWithFunc(t *testing.T) {
	testCases := []struct {
		name     string
		baseFunc bufio.SplitFunc
		trimFunc Func
		input    []byte
		steps    []splittest.Step
	}{
		{
			// nil trim func should return original split func
			name:     "NilTrimFunc",
			input:    []byte(" hello \n world \n extra "),
			baseFunc: bufio.ScanLines,
			trimFunc: nil,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
				splittest.ExpectAdvanceToken(len(" extra "), " extra "),
			},
		},
		{
			name:     "ScanLinesStrictWithTrim",
			input:    []byte(" hello \n world \n extra "),
			baseFunc: bufio.ScanLines,
			trimFunc: Whitespace,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), "hello"),
				splittest.ExpectAdvanceToken(len(" world \n"), "world"),
				splittest.ExpectAdvanceToken(len(" extra "), "extra"),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc := WithFunc(tc.baseFunc, tc.trimFunc)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func TestToLength(t *testing.T) {
	testCases := []struct {
		name      string
		baseFunc  bufio.SplitFunc
		maxLength int
		input     []byte
		steps     []splittest.Step
	}{
		{
			name:      "IgnoreZeroLength",
			input:     []byte(" hello \n world \n extra "),
			baseFunc:  bufio.ScanLines,
			maxLength: 0,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
				splittest.ExpectAdvanceToken(len(" extra "), " extra "),
			},
		},
		{
			name:      "NoLongTokens",
			input:     []byte(" hello \n world \n extra "),
			baseFunc:  bufio.ScanLines,
			maxLength: 100,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
				splittest.ExpectAdvanceToken(len(" extra "), " extra "),
			},
		},
		{
			name:      "LongButNoToken",
			input:     []byte("This is a long but incomplete token."),
			baseFunc:  splittest.ScanLinesStrict,
			maxLength: 10,
			steps: []splittest.Step{
				splittest.ExpectToken("This is a "),
				splittest.ExpectToken("long but i"),
				splittest.ExpectToken("ncomplete "),
			},
		},
		{
			name:      "OneLongToken",
			input:     []byte("This is a very long token."),
			baseFunc:  bufio.ScanLines,
			maxLength: 10,
			steps: []splittest.Step{
				splittest.ExpectToken("This is a "),
				splittest.ExpectToken("very long "),
				splittest.ExpectToken("token."),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc := ToLength(tc.baseFunc, tc.maxLength)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}
