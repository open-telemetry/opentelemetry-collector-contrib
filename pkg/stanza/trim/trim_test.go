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
	}

	for _, tc := range testCases {
		splitFunc := ToLength(tc.baseFunc, tc.maxLength)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func TestToLength_TruncationBehavior(t *testing.T) {
	splitFunc := ToLength(bufio.ScanLines, 10)

	// Test with a single long line
	input := []byte("12345678901234567890\n")
	scanner := bufio.NewScanner(bufio.NewReader(bytes.NewReader(input)))
	scanner.Split(splitFunc)

	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}

	assert.NoError(t, scanner.Err())
	// Should only get ONE token (truncated), not multiple
	assert.Len(t, tokens, 1, "Expected only one token for oversized line, got %d", len(tokens))
	assert.Equal(t, "1234567890", tokens[0], "Token should be truncated to maxLength")
}

func TestToLength_SkipRemainder(t *testing.T) {
	splitFunc := ToLength(bufio.ScanLines, 10)

	// Test with long line followed by normal line
	input := []byte("Very long line here that exceeds limit.\nShort.\n")
	scanner := bufio.NewScanner(bufio.NewReader(bytes.NewReader(input)))
	scanner.Split(splitFunc)

	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}

	assert.NoError(t, scanner.Err())
	// Should get truncated long line + short line, not the remainder of long line
	assert.Len(t, tokens, 2, "Expected two tokens (truncated long + short), got %d", len(tokens))
	assert.Equal(t, "Very long ", tokens[0], "First token should be truncated")
	assert.Equal(t, "Short.", tokens[1], "Second token should be the short line, not remainder of first")
}

func TestToLength_MultipleOversizedLines(t *testing.T) {
	splitFunc := ToLength(bufio.ScanLines, 10)

	input := []byte("First very long line here.\nSecond also very long line.\n")
	scanner := bufio.NewScanner(bufio.NewReader(bytes.NewReader(input)))
	scanner.Split(splitFunc)

	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}

	assert.NoError(t, scanner.Err())
	// Key assertion: each oversized line produces only ONE token (truncated), not multiple
	// We should get 2 tokens (one per line), not 4+ (which would indicate splitting)
	assert.Len(t, tokens, 2, "Expected 2 tokens (one per oversized line), got %d: %v", len(tokens), tokens)
	if len(tokens) >= 1 {
		assert.Equal(t, "First very", tokens[0], "First line should be truncated to 10 bytes")
	}
	if len(tokens) >= 2 {
		assert.Equal(t, "Second als", tokens[1], "Second line should be truncated to 10 bytes")
	}
}

func TestToLength_EmptyLineAfterOversized(t *testing.T) {
	splitFunc := ToLength(bufio.ScanLines, 10)

	input := []byte("Very long line here.\n\nShort.\n")
	scanner := bufio.NewScanner(bufio.NewReader(bytes.NewReader(input)))
	scanner.Split(splitFunc)

	var tokens []string
	for scanner.Scan() {
		tokens = append(tokens, scanner.Text())
	}

	assert.NoError(t, scanner.Err())
	// Should get truncated line, empty line, and short line
	assert.Len(t, tokens, 3, "Expected 3 tokens, got %d", len(tokens))
	assert.Equal(t, "Very long ", tokens[0], "First line should be truncated")
	assert.Empty(t, tokens[1], "Empty line should be preserved")
	assert.Equal(t, "Short.", tokens[2], "Short line should pass through")
}
