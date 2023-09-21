// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/assert"
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
	scanAndTrimLines := WithFunc(bufio.ScanLines, Config{
		PreserveLeading:  false,
		PreserveTrailing: false,
	}.Func())

	input := []byte(" hello \n world \n extra ")

	advance, token, err := scanAndTrimLines(input, false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte("hello"), token)

	advance, token, err = scanAndTrimLines(input[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte("world"), token)

	advance, token, err = scanAndTrimLines(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Nil(t, token)
}

func TestWithNilTrimFunc(t *testing.T) {
	// Same test as above, but pass nil instead of a trim func
	// In other words, we should expect exactly the behavior of bufio.ScanLines.

	scanLines := WithFunc(bufio.ScanLines, nil)

	input := []byte(" hello \n world \n extra ")

	advance, token, err := scanLines(input, false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte(" hello "), token)

	advance, token, err = scanLines(input[8:], false)
	assert.NoError(t, err)
	assert.Equal(t, 8, advance)
	assert.Equal(t, []byte(" world "), token)

	advance, token, err = scanLines(input[16:], false)
	assert.NoError(t, err)
	assert.Equal(t, 0, advance)
	assert.Nil(t, token)
}
