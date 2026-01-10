// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim

import (
	"bufio"
	"bytes"
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

func TestToLengthWithTruncate(t *testing.T) {
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
	}

	for _, tc := range testCases {
		skipping := false
		splitFunc := ToLengthWithTruncate(tc.baseFunc, tc.maxLength, &skipping)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestToLengthWithTruncate_Scanner tests the truncation behavior using a real bufio.Scanner
// This is important because the stateful skip mode requires multiple calls to the split function
func TestToLengthWithTruncate_Scanner(t *testing.T) {
	testCases := []struct {
		name           string
		input          []byte
		maxLength      int
		expectedTokens []string
	}{
		{
			name:           "OversizedLineWithNewline_Truncates",
			input:          []byte("Very long line here.\nShort.\n"),
			maxLength:      10,
			expectedTokens: []string{"Very long ", "Short."},
		},
		{
			name:           "MultipleOversizedLines_Truncates",
			input:          []byte("First very long line here.\nSecond also very long line.\n"),
			maxLength:      10,
			expectedTokens: []string{"First very", "Second als"},
		},
		{
			name:           "OversizedLineAtEOF_Truncates",
			input:          []byte("This is a very long line without newline"),
			maxLength:      10,
			expectedTokens: []string{"This is a "},
		},
		{
			name:           "MixedSizes",
			input:          []byte("Short.\nVery long oversized line.\nTiny\n"),
			maxLength:      10,
			expectedTokens: []string{"Short.", "Very long ", "Tiny"},
		},
		{
			name:           "EmptyLineAfterOversized",
			input:          []byte("Very long line here.\n\nShort.\n"),
			maxLength:      10,
			expectedTokens: []string{"Very long ", "", "Short."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			skipping := false
			splitFunc := ToLengthWithTruncate(bufio.ScanLines, tc.maxLength, &skipping)
			scanner := bufio.NewScanner(bytes.NewReader(tc.input))
			scanner.Split(splitFunc)

			var tokens []string
			for scanner.Scan() {
				tokens = append(tokens, scanner.Text())
			}

			assert.NoError(t, scanner.Err())
			assert.Equal(t, tc.expectedTokens, tokens, "Expected tokens to match")
		})
	}
}

// TestToLengthWithTruncate_LimitedBuffer tests with a buffer size limited to maxLength,
// simulating how the collector works where buffer size is capped at max_log_size
func TestToLengthWithTruncate_LimitedBuffer(t *testing.T) {
	testCases := []struct {
		name           string
		input          []byte
		maxLength      int
		expectedTokens []string
	}{
		{
			name:           "OversizedLineWithNewline_LimitedBuffer",
			input:          []byte("Very long line here.\nShort.\n"),
			maxLength:      10,
			expectedTokens: []string{"Very long ", "Short."},
		},
		{
			name:           "MultipleOversizedLines_LimitedBuffer",
			input:          []byte("First very long line here.\nSecond also very long line.\n"),
			maxLength:      10,
			expectedTokens: []string{"First very", "Second als"},
		},
		{
			name:           "OversizedLineAtEOF_LimitedBuffer",
			input:          []byte("This is a very long line without newline"),
			maxLength:      10,
			expectedTokens: []string{"This is a "},
		},
		{
			name:           "MixedSizes_LimitedBuffer",
			input:          []byte("Short.\nVery long oversized line.\nTiny\n"),
			maxLength:      10,
			expectedTokens: []string{"Short.", "Very long ", "Tiny"},
		},
		{
			name:           "VeryLongLine_MuchLongerThanBuffer",
			input:          append(bytes.Repeat([]byte("A"), 100), []byte("\nShort.\n")...),
			maxLength:      10,
			expectedTokens: []string{"AAAAAAAAAA", "Short."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			skipping := false
			splitFunc := ToLengthWithTruncate(bufio.ScanLines, tc.maxLength, &skipping)
			scanner := bufio.NewScanner(bytes.NewReader(tc.input))
			scanner.Split(splitFunc)
			// Set buffer to exactly maxLength to simulate collector behavior
			scanner.Buffer(make([]byte, tc.maxLength), tc.maxLength)

			var tokens []string
			for scanner.Scan() {
				tokens = append(tokens, scanner.Text())
			}

			assert.NoError(t, scanner.Err())
			assert.Equal(t, tc.expectedTokens, tokens, "Expected tokens to match")
		})
	}
}

// TestToLengthWithTruncate_PositionalScanner tests with a positional scanner wrapper like the collector uses
func TestToLengthWithTruncate_PositionalScanner(t *testing.T) {
	testCases := []struct {
		name           string
		input          []byte
		maxLength      int
		expectedTokens []string
	}{
		{
			name:           "10xBufferLine",
			input:          append(bytes.Repeat([]byte("A"), 100), []byte("\nShort.\n")...),
			maxLength:      10,
			expectedTokens: []string{"AAAAAAAAAA", "Short."},
		},
		{
			name:           "ExactlyAtMaxLength",
			input:          []byte("1234567890\nShort.\n"),
			maxLength:      10,
			expectedTokens: []string{"1234567890", "Short."},
		},
		{
			name:           "OneByteOverMaxLength",
			input:          []byte("12345678901\nShort.\n"),
			maxLength:      10,
			expectedTokens: []string{"1234567890", "Short."},
		},
		{
			name:           "MultipleOversizedWithShort",
			input:          append(append(bytes.Repeat([]byte("A"), 50), []byte("\nOK\n")...), append(bytes.Repeat([]byte("B"), 50), []byte("\nEnd\n")...)...),
			maxLength:      10,
			expectedTokens: []string{"AAAAAAAAAA", "OK", "BBBBBBBBBB", "End"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			skipping := false
			splitFunc := ToLengthWithTruncate(bufio.ScanLines, tc.maxLength, &skipping)

			// Create a positional scanner wrapper like the collector does
			reader := bytes.NewReader(tc.input)
			s := bufio.NewScanner(reader)
			s.Buffer(make([]byte, tc.maxLength), tc.maxLength)

			var pos int64
			posTrackingFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
				advance, token, err = splitFunc(data, atEOF)
				pos += int64(advance)
				return advance, token, err
			}
			s.Split(posTrackingFunc)

			var tokens []string
			for s.Scan() {
				tokens = append(tokens, s.Text())
			}

			assert.NoError(t, s.Err())
			assert.Equal(t, tc.expectedTokens, tokens, "Expected tokens to match")
		})
	}
}
