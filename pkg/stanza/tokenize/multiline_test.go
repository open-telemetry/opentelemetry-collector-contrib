// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tokenize

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize/tokenizetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestLineStartSplitFunc(t *testing.T) {
	testCases := []tokenizetest.TestCase{
		{
			Name:    "OneLogSimple",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte("LOGSTART 123 log1LOGSTART 123 a"),
			ExpectedTokens: []string{
				`LOGSTART 123 log1`,
			},
		},
		{
			Name:                  "OneLogSimpleOmitPattern",
			Pattern:               `LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input:                 []byte("LOGSTART 123 log1LOGSTART 123 a"),
			ExpectedTokens: []string{
				`log1`,
			},
		},
		{
			Name:    "TwoLogsSimple",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte(`LOGSTART 123 log1 LOGSTART 234 log2 LOGSTART 345 foo`),
			ExpectedTokens: []string{
				`LOGSTART 123 log1`,
				`LOGSTART 234 log2`,
			},
		},
		{

			Name:                  "TwoLogsSimpleOmitPattern",
			Pattern:               `LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input:                 []byte(`LOGSTART 123 log1 LOGSTART 234 log2 LOGSTART 345 foo`),
			ExpectedTokens: []string{
				`log1`,
				`log2`,
			},
		},
		{
			Name:    "TwoLogsLineStart",
			Pattern: `^LOGSTART \d+ `,
			Input:   []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
			ExpectedTokens: []string{
				"LOGSTART 123 LOGSTART 345 log1",
				"LOGSTART 234 log2",
			},
		},
		{
			Name:                  "TwoLogsLineStartOmitPattern",
			Pattern:               `^LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input:                 []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
			ExpectedTokens: []string{
				"LOGSTART 345 log1",
				"log2",
			},
		},
		{
			Name:    "NoMatches",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte(`file that has no matches in it`),
		},
		{

			Name:                  "NoMatchesOmitPattern",
			Pattern:               `LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input:                 []byte(`file that has no matches in it`),
		},
		{
			Name:    "PrecedingNonMatches",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte(`part that doesn't match LOGSTART 123 part that matchesLOGSTART 123 foo`),
			ExpectedTokens: []string{
				`part that doesn't match`,
				`LOGSTART 123 part that matches`,
			},
		},
		{
			Name:                  "PrecedingNonMatchesOmitPattern",
			Pattern:               `LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input:                 []byte(`part that doesn't match LOGSTART 123 part that matchesLOGSTART 123 foo`),
			ExpectedTokens: []string{
				`part that doesn't match`,
				`part that matches`,
			},
		},
		{
			Name:    "HugeLog100",
			Pattern: `LOGSTART \d+ `,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, tokenizetest.GenerateBytes(100)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				`LOGSTART 123 ` + string(tokenizetest.GenerateBytes(100)),
			},
		},
		{

			Name:                  "HugeLog100OmitPattern",
			Pattern:               `LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, tokenizetest.GenerateBytes(100)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				string(tokenizetest.GenerateBytes(100)),
			},
		},
		{
			Name:    "HugeLog10000",
			Pattern: `LOGSTART \d+ `,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, tokenizetest.GenerateBytes(10000)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				`LOGSTART 123 ` + string(tokenizetest.GenerateBytes(10000)),
			},
		},
		{
			Name:    "ErrTooLong",
			Pattern: `LOGSTART \d+ `,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, tokenizetest.GenerateBytes(1000000)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:                  "ErrTooLongOmitPattern",
			Pattern:               `LOGSTART \d+ `,
			OmitPatternFromRecord: true,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, tokenizetest.GenerateBytes(1000000)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:    "MultipleMultilineLogs",
			Pattern: `^LOGSTART \d+`,
			Input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line\nLOGSTART 43 log5"),
			ExpectedTokens: []string{
				"LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1",
				"LOGSTART 17 log2\nLOGPART log2\nanother line",
			},
		},
		{
			Name:                  "MultipleMultilineLogsOmitPattern",
			Pattern:               `^LOGSTART \d+`,
			OmitPatternFromRecord: true,
			Input:                 []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line\nLOGSTART 43 log5"),
			ExpectedTokens: []string{
				"log1\t  \nLOGPART log1\nLOGPART log1",
				"log2\nLOGPART log2\nanother line",
			},
		},
		{

			Name:                  "LogsWithoutFlusherOmitPattern",
			Pattern:               `^LOGSTART \d+`,
			OmitPatternFromRecord: true,
			Input:                 []byte("LOGPART log1\nLOGPART log1\t   \n"),
		},
		{
			Name:    "LogsWithoutFlusher",
			Pattern: `^LOGSTART \d+`,
			Input:   []byte("LOGPART log1\nLOGPART log1\t   \n"),
		},
	}

	for _, tc := range testCases {
		cfg := &MultilineConfig{
			LineStartPattern:      tc.Pattern,
			OmitPatternFromRecord: tc.OmitPatternFromRecord,
		}

		trimFunc := trim.Config{
			PreserveLeading:  tc.PreserveLeadingWhitespaces,
			PreserveTrailing: tc.PreserveTrailingWhitespaces,
		}.Func()
		splitFunc, err := cfg.getSplitFunc(unicode.UTF8, false, 0, trimFunc)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}

	t.Run("FirstMatchHitsEndOfBuffer", func(t *testing.T) {
		splitFunc := LineStartSplitFunc(regexp.MustCompile("LOGSTART"), false, false, trim.Nop)
		data := []byte(`LOGSTART`)

		t.Run("NotAtEOF", func(t *testing.T) {
			advance, token, err := splitFunc(data, false)
			require.NoError(t, err)
			require.Equal(t, 0, advance)
			require.Nil(t, token)
		})

		t.Run("AtEOF", func(t *testing.T) {
			advance, token, err := splitFunc(data, true)
			require.NoError(t, err)
			require.Equal(t, 0, advance)
			require.Nil(t, token)
		})
	})
}

func TestLineEndSplitFunc(t *testing.T) {
	testCases := []tokenizetest.TestCase{
		{
			Name:    "OneLogSimple",
			Pattern: `LOGEND \d+`,
			Input:   []byte(`my log LOGEND 123`),
			ExpectedTokens: []string{
				`my log LOGEND 123`,
			},
		},
		{
			Name:                  "OneLogSimpleOmitPattern",
			Pattern:               `LOGEND \d+`,
			OmitPatternFromRecord: true,
			Input:                 []byte(`my log LOGEND 123`),
			ExpectedTokens: []string{
				`my log`,
			},
		},
		{
			Name:    "TwoLogsSimple",
			Pattern: `LOGEND \d+`,
			Input:   []byte(`log1 LOGEND 123log2 LOGEND 234`),
			ExpectedTokens: []string{
				`log1 LOGEND 123`,
				`log2 LOGEND 234`,
			},
		},
		{
			Name:                  "TwoLogsSimpleOmitPattern",
			Pattern:               `LOGEND \d+`,
			OmitPatternFromRecord: true,
			Input:                 []byte(`log1 LOGEND 123log2 LOGEND 234`),
			ExpectedTokens: []string{
				`log1`,
				`log2`,
			},
		},
		{
			Name:    "TwoLogsLineEndSimple",
			Pattern: `LOGEND$`,
			Input:   []byte("log1 LOGEND LOGEND\nlog2 LOGEND\n"),
			ExpectedTokens: []string{
				"log1 LOGEND LOGEND",
				"log2 LOGEND",
			},
		},
		{
			Name:                  "TwoLogsLineEndSimpleOmitPattern",
			Pattern:               `LOGEND$`,
			OmitPatternFromRecord: true,
			Input:                 []byte("log1 LOGEND LOGEND\nlog2 LOGEND\n"),
			ExpectedTokens: []string{
				"log1 LOGEND",
				"log2",
			},
		},
		{
			Name:    "NoMatches",
			Pattern: `LOGEND \d+`,
			Input:   []byte(`file that has no matches in it`),
		},
		{
			Name:                  "NoMatchesOmitPattern",
			OmitPatternFromRecord: true,
			Pattern:               `LOGEND \d+`,
			Input:                 []byte(`file that has no matches in it`),
		},
		{
			Name:    "NonMatchesAfter",
			Pattern: `LOGEND \d+`,
			Input:   []byte(`part that matches LOGEND 123 part that doesn't match`),
			ExpectedTokens: []string{
				`part that matches LOGEND 123`,
			},
		},
		{
			Name:                  "NonMatchesAfterOmitPattern",
			Pattern:               `LOGEND \d+`,
			OmitPatternFromRecord: true,
			Input:                 []byte(`part that matches LOGEND 123 part that doesn't match`),
			ExpectedTokens: []string{
				`part that matches`,
			},
		},
		{
			Name:    "HugeLog100",
			Pattern: `LOGEND \d`,
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(100)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				string(tokenizetest.GenerateBytes(100)) + `LOGEND 1`,
			},
		}, {
			Name:                  "HugeLog100OmitPattern",
			Pattern:               `LOGEND \d`,
			OmitPatternFromRecord: true,
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(100)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				string(tokenizetest.GenerateBytes(100)),
			},
		},
		{
			Name:    "HugeLog10000",
			Pattern: `LOGEND \d`,
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(10000)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				string(tokenizetest.GenerateBytes(10000)) + `LOGEND 1`,
			},
		},
		{
			Name:    "HugeLog1000000",
			Pattern: `LOGEND \d`,
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(1000000)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:                  "HugeLog1000000OmitPattern",
			Pattern:               `LOGEND \d`,
			OmitPatternFromRecord: true,
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(1000000)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:    "MultipleMultilineLogs",
			Pattern: `^LOGEND.*$`,
			Input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2\nLOGSTART 43 log5"),
			ExpectedTokens: []string{
				"LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1",
				"LOGSTART 17 log2\nLOGPART log2\nLOGEND log2",
			},
		},
		{
			Name:                  "MultipleMultilineLogsOmitPattern",
			Pattern:               `^LOGEND.*$`,
			OmitPatternFromRecord: true,
			Input:                 []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2\nLOGSTART 43 log5"),
			ExpectedTokens: []string{
				"LOGSTART 12 log1\t  \nLOGPART log1",
				"LOGSTART 17 log2\nLOGPART log2",
			},
		},
		{
			Name:    "LogsWithoutFlusher",
			Pattern: `^LOGEND.*$`,
			Input:   []byte("LOGPART log1\nLOGPART log1\t   \n"),
		},
		{
			Name:                  "LogsWithoutFlusherOmitPattern",
			OmitPatternFromRecord: true,
			Pattern:               `^LOGEND.*$`,
			Input:                 []byte("LOGPART log1\nLOGPART log1\t   \n"),
		},
	}

	for _, tc := range testCases {
		cfg := &MultilineConfig{
			LineEndPattern:        tc.Pattern,
			OmitPatternFromRecord: tc.OmitPatternFromRecord,
		}

		trimFunc := trim.Config{
			PreserveLeading:  tc.PreserveLeadingWhitespaces,
			PreserveTrailing: tc.PreserveTrailingWhitespaces,
		}.Func()
		splitFunc, err := cfg.getSplitFunc(unicode.UTF8, false, 0, trimFunc)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}
}

func TestNewlineSplitFunc(t *testing.T) {
	testCases := []tokenizetest.TestCase{
		{
			Name:  "OneLogSimple",
			Input: []byte("my log\n"),
			ExpectedTokens: []string{
				`my log`,
			},
		},
		{
			Name:  "OneLogCarriageReturn",
			Input: []byte("my log\r\n"),
			ExpectedTokens: []string{
				`my log`,
			},
		},
		{
			Name:  "TwoLogsSimple",
			Input: []byte("log1\nlog2\n"),
			ExpectedTokens: []string{
				`log1`,
				`log2`,
			},
		},
		{
			Name:  "TwoLogsCarriageReturn",
			Input: []byte("log1\r\nlog2\r\n"),
			ExpectedTokens: []string{
				`log1`,
				`log2`,
			},
		},
		{
			Name:  "NoTailingNewline",
			Input: []byte(`foo`),
		},
		{
			Name: "HugeLog100",
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(100)
				newInput = append(newInput, '\n')
				return newInput
			}(),
			ExpectedTokens: []string{
				string(tokenizetest.GenerateBytes(100)),
			},
		},
		{
			Name: "HugeLog10000",
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(10000)
				newInput = append(newInput, '\n')
				return newInput
			}(),
			ExpectedTokens: []string{
				string(tokenizetest.GenerateBytes(10000)),
			},
		},
		{
			Name: "HugeLog1000000",
			Input: func() []byte {
				newInput := tokenizetest.GenerateBytes(1000000)
				newInput = append(newInput, '\n')
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:  "LogsWithoutFlusher",
			Input: []byte("LOGPART log1"),
		},
		{
			Name:  "DefaultFlusherSplits",
			Input: []byte("log1\nlog2\n"),
			ExpectedTokens: []string{
				"log1",
				"log2",
			},
		},
		{
			Name:  "LogsWithLogStartingWithWhiteChars",
			Input: []byte("\nLOGEND 333\nAnother one"),
			ExpectedTokens: []string{
				"",
				"LOGEND 333",
			},
		},
		{
			Name:  "PreserveLeadingWhitespaces",
			Input: []byte("\n LOGEND 333 \nAnother one "),
			ExpectedTokens: []string{
				"",
				" LOGEND 333",
			},
			PreserveLeadingWhitespaces: true,
		},
		{
			Name:  "PreserveTrailingWhitespaces",
			Input: []byte("\n LOGEND 333 \nAnother one "),
			ExpectedTokens: []string{
				"",
				"LOGEND 333 ",
			},
			PreserveTrailingWhitespaces: true,
		},
		{
			Name:  "PreserveBothLeadingAndTrailingWhitespaces",
			Input: []byte("\n LOGEND 333 \nAnother one "),
			ExpectedTokens: []string{
				"",
				" LOGEND 333 ",
			},
			PreserveLeadingWhitespaces:  true,
			PreserveTrailingWhitespaces: true,
		},
	}

	for _, tc := range testCases {
		trimFunc := trim.Config{
			PreserveLeading:  tc.PreserveLeadingWhitespaces,
			PreserveTrailing: tc.PreserveTrailingWhitespaces,
		}.Func()
		splitFunc, err := NewlineSplitFunc(unicode.UTF8, false, trimFunc)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}
}

func TestNoSplitFunc(t *testing.T) {
	const largeLogSize = 100
	testCases := []tokenizetest.TestCase{
		{
			Name:           "OneLogSimple",
			Input:          []byte("my log\n"),
			ExpectedTokens: []string{"my log\n"},
		},
		{
			Name:           "TwoLogsSimple",
			Input:          []byte("log1\nlog2\n"),
			ExpectedTokens: []string{"log1\nlog2\n"},
		},
		{
			Name:           "TwoLogsCarriageReturn",
			Input:          []byte("log1\r\nlog2\r\n"),
			ExpectedTokens: []string{"log1\r\nlog2\r\n"},
		},
		{
			Name:           "NoTailingNewline",
			Input:          []byte(`foo`),
			ExpectedTokens: []string{"foo"},
		},
		{
			Name: "HugeLog100",
			Input: func() []byte {
				return tokenizetest.GenerateBytes(largeLogSize)
			}(),
			ExpectedTokens: []string{string(tokenizetest.GenerateBytes(largeLogSize))},
		},
		{
			Name: "HugeLog300",
			Input: func() []byte {
				return tokenizetest.GenerateBytes(largeLogSize * 3)
			}(),
			ExpectedTokens: []string{
				"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv",
				"wxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr",
				"stuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn",
			},
		},
		{
			Name: "EOFBeforeMaxLogSize",
			Input: func() []byte {
				return tokenizetest.GenerateBytes(largeLogSize * 3.5)
			}(),
			ExpectedTokens: []string{
				"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv",
				"wxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr",
				"stuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn",
				"opqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl",
			},
		},
	}

	for _, tc := range testCases {
		splitFunc := NoSplitFunc(largeLogSize)
		t.Run(tc.Name, tc.Run(splitFunc))
	}
}

func TestNoopEncodingError(t *testing.T) {
	cfg := &MultilineConfig{
		LineEndPattern: "\n",
	}

	_, err := cfg.getSplitFunc(encoding.Nop, false, 0, trim.Nop)
	require.Equal(t, err, fmt.Errorf("line_start_pattern or line_end_pattern should not be set when using nop encoding"))

	cfg = &MultilineConfig{
		LineStartPattern: "\n",
	}

	_, err = cfg.getSplitFunc(encoding.Nop, false, 0, trim.Nop)
	require.Equal(t, err, fmt.Errorf("line_start_pattern or line_end_pattern should not be set when using nop encoding"))
}

func TestNewlineSplitFunc_Encodings(t *testing.T) {
	cases := []struct {
		name     string
		encoding encoding.Encoding
		input    []byte
		tokens   [][]byte
	}{
		{
			"Simple",
			unicode.UTF8,
			[]byte("testlog\n"),
			[][]byte{[]byte("testlog")},
		},
		{
			"CarriageReturn",
			unicode.UTF8,
			[]byte("testlog\r\n"),
			[][]byte{[]byte("testlog")},
		},
		{
			"SimpleUTF16",
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			[]byte{0, 116, 0, 101, 0, 115, 0, 116, 0, 108, 0, 111, 0, 103, 0, 10}, // testlog\n
			[][]byte{{0, 116, 0, 101, 0, 115, 0, 116, 0, 108, 0, 111, 0, 103}},
		},
		{
			"MultiUTF16",
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			[]byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 10, 0, 108, 0, 111, 0, 103, 0, 50, 0, 10}, // log1\nlog2\n
			[][]byte{
				{0, 108, 0, 111, 0, 103, 0, 49}, // log1
				{0, 108, 0, 111, 0, 103, 0, 50}, // log2
			},
		},
		{
			"MultiCarriageReturnUTF16",
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			[]byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 13, 0, 10, 0, 108, 0, 111, 0, 103, 0, 50, 0, 13, 0, 10}, // log1\r\nlog2\r\n
			[][]byte{
				{0, 108, 0, 111, 0, 103, 0, 49}, // log1
				{0, 108, 0, 111, 0, 103, 0, 50}, // log2
			},
		},
		{
			"MultiCarriageReturnUTF16StartingWithWhiteChars",
			unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			[]byte{0, 13, 0, 10, 0, 108, 0, 111, 0, 103, 0, 49, 0, 13, 0, 10, 0, 108, 0, 111, 0, 103, 0, 50, 0, 13, 0, 10}, // \r\nlog1\r\nlog2\r\n
			[][]byte{
				{},
				{0, 108, 0, 111, 0, 103, 0, 49}, // log1
				{0, 108, 0, 111, 0, 103, 0, 50}, // log2
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			splitFunc, err := NewlineSplitFunc(tc.encoding, false, trim.Nop)
			require.NoError(t, err)
			scanner := bufio.NewScanner(bytes.NewReader(tc.input))
			scanner.Split(splitFunc)

			var tokens [][]byte
			for {
				ok := scanner.Scan()
				if !ok {
					require.NoError(t, scanner.Err())
					break
				}

				tokens = append(tokens, scanner.Bytes())
			}

			require.Equal(t, tc.tokens, tokens)
		})
	}
}
