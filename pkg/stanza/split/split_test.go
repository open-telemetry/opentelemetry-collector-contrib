// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestConfigFunc(t *testing.T) {
	maxLogSize := 100

	t.Run("BothStartAndEnd", func(t *testing.T) {
		cfg := &Config{
			LineStartPattern: "foo",
			LineEndPattern:   "bar",
		}

		_, err := cfg.Func(unicode.UTF8, false, maxLogSize, trim.Nop)
		assert.EqualError(t, err, "only one of line_start_pattern or line_end_pattern can be set")
	})

	t.Run("NopEncoding", func(t *testing.T) {
		cfg := &Config{}

		f, err := cfg.Func(encoding.Nop, false, maxLogSize, trim.Nop)
		assert.NoError(t, err)

		raw := splittest.GenerateBytes(maxLogSize * 2)
		advance, token, err := f(raw, false)
		assert.NoError(t, err)
		assert.Equal(t, maxLogSize, advance)
		assert.Equal(t, raw[:maxLogSize], token)
	})

	t.Run("Newline", func(t *testing.T) {
		cfg := &Config{}

		f, err := cfg.Func(unicode.UTF8, false, maxLogSize, trim.Nop)
		assert.NoError(t, err)

		advance, token, err := f([]byte("foo\nbar\nbaz\n"), false)
		assert.NoError(t, err)
		assert.Equal(t, 4, advance)
		assert.Equal(t, []byte("foo"), token)
	})

	t.Run("InvalidStartRegex", func(t *testing.T) {
		cfg := &Config{
			LineStartPattern: "[",
		}

		_, err := cfg.Func(unicode.UTF8, false, maxLogSize, trim.Nop)
		assert.EqualError(t, err, "compile line start regex: error parsing regexp: missing closing ]: `[`")
	})

	t.Run("InvalidEndRegex", func(t *testing.T) {
		cfg := &Config{
			LineEndPattern: "[",
		}

		_, err := cfg.Func(unicode.UTF8, false, maxLogSize, trim.Nop)
		assert.EqualError(t, err, "compile line end regex: error parsing regexp: missing closing ]: `[`")
	})

	t.Run("EncodeNewlineDstTooShort", func(t *testing.T) {
		cfg := &Config{}
		enc := errEncoding{}
		_, err := cfg.Func(&enc, false, maxLogSize, trim.Nop)
		assert.EqualError(t, err, "strange encoding")
	})
}

func TestLineStartSplitFunc(t *testing.T) {
	testCases := []splittest.TestCase{
		{
			Name:    "OneLogSimple",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte("LOGSTART 123 log1LOGSTART 123 a"),
			ExpectedTokens: []string{
				`LOGSTART 123 log1`,
			},
		},
		{
			Name:    "TwoLogsSimple",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte(`LOGSTART 123 log1 LOGSTART 234 log2 LOGSTART 345 foo`),
			ExpectedTokens: []string{
				`LOGSTART 123 log1 `,
				`LOGSTART 234 log2 `,
			},
		},
		{
			Name:    "TwoLogsLineStart",
			Pattern: `^LOGSTART \d+ `,
			Input:   []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
			ExpectedTokens: []string{
				"LOGSTART 123 LOGSTART 345 log1\n",
				"LOGSTART 234 log2\n",
			},
		},
		{
			Name:    "NoMatches",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte(`file that has no matches in it`),
		},
		{
			Name:    "PrecedingNonMatches",
			Pattern: `LOGSTART \d+ `,
			Input:   []byte(`part that doesn't match LOGSTART 123 part that matchesLOGSTART 123 foo`),
			ExpectedTokens: []string{
				`part that doesn't match `,
				`LOGSTART 123 part that matches`,
			},
		},
		{
			Name:    "HugeLog100",
			Pattern: `LOGSTART \d+ `,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, splittest.GenerateBytes(100)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				`LOGSTART 123 ` + string(splittest.GenerateBytes(100)),
			},
		},
		{
			Name:    "HugeLog10000",
			Pattern: `LOGSTART \d+ `,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, splittest.GenerateBytes(10000)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				`LOGSTART 123 ` + string(splittest.GenerateBytes(10000)),
			},
		},
		{
			Name:    "ErrTooLong",
			Pattern: `LOGSTART \d+ `,
			Input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, splittest.GenerateBytes(1000000)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:    "MultiplesplitLogs",
			Pattern: `^LOGSTART \d+`,
			Input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line\nLOGSTART 43 log5"),
			ExpectedTokens: []string{
				"LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \n",
				"LOGSTART 17 log2\nLOGPART log2\nanother line\n",
			},
		},
		{
			Name:    "NoMatch",
			Pattern: `^LOGSTART \d+`,
			Input:   []byte("LOGPART log1\nLOGPART log1\t   \n"),
		},
	}

	for _, tc := range testCases {
		cfg := &Config{LineStartPattern: tc.Pattern}
		splitFunc, err := cfg.Func(unicode.UTF8, false, 0, trim.Nop)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}

	t.Run("FirstMatchHitsEndOfBuffer", func(t *testing.T) {
		splitFunc := LineStartSplitFunc(regexp.MustCompile("LOGSTART"), false, trim.Nop)
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

	t.Run("FlushAtEOF", func(t *testing.T) {
		flushAtEOFCases := []splittest.TestCase{
			{
				Name:           "NoMatch",
				Pattern:        `^LOGSTART \d+`,
				Input:          []byte("LOGPART log1\nLOGPART log1\t   \n"),
				ExpectedTokens: []string{"LOGPART log1\nLOGPART log1\t   \n"},
			},
			{
				Name:    "MatchThenNoMatch",
				Pattern: `^LOGSTART \d+`,
				Input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line"),
				ExpectedTokens: []string{
					"LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \n",
					"LOGSTART 17 log2\nLOGPART log2\nanother line",
				},
			},
		}
		for _, tc := range flushAtEOFCases {
			cfg := &Config{
				LineStartPattern: `^LOGSTART \d+`,
			}
			splitFunc, err := cfg.Func(unicode.UTF8, true, 0, trim.Nop)
			require.NoError(t, err)
			t.Run(tc.Name, tc.Run(splitFunc))
		}
	})

	t.Run("ApplyTrimFunc", func(t *testing.T) {
		cfg := Config{LineStartPattern: ` LOGSTART \d+ `}
		input := []byte(" LOGSTART 123 log1  LOGSTART 234 log2  LOGSTART 345 foo")

		splitTrimLeading, err := cfg.Func(unicode.UTF8, false, 0, trim.Leading)
		require.NoError(t, err)
		t.Run("TrimLeading", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`LOGSTART 123 log1 `,
				`LOGSTART 234 log2 `,
			}}.Run(splitTrimLeading))

		splitTrimTrailing, err := cfg.Func(unicode.UTF8, false, 0, trim.Trailing)
		require.NoError(t, err)
		t.Run("TrimTrailing", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				` LOGSTART 123 log1`,
				` LOGSTART 234 log2`,
			}}.Run(splitTrimTrailing))

		splitTrimBoth, err := cfg.Func(unicode.UTF8, false, 0, trim.Whitespace)
		require.NoError(t, err)
		t.Run("TrimBoth", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`LOGSTART 123 log1`,
				`LOGSTART 234 log2`,
			}}.Run(splitTrimBoth))
	})
}

func TestLineEndSplitFunc(t *testing.T) {
	testCases := []splittest.TestCase{
		{
			Name:    "OneLogSimple",
			Pattern: `LOGEND \d+`,
			Input:   []byte(`my log LOGEND 123`),
			ExpectedTokens: []string{
				`my log LOGEND 123`,
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
			Name:    "TwoLogsLineEndSimple",
			Pattern: `LOGEND$`,
			Input:   []byte("log1 LOGEND LOGEND\nlog2 LOGEND\n"),
			ExpectedTokens: []string{
				"log1 LOGEND LOGEND",
				"\nlog2 LOGEND",
			},
		},
		{
			Name:    "NoMatches",
			Pattern: `LOGEND \d+`,
			Input:   []byte(`file that has no matches in it`),
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
			Name:    "HugeLog100",
			Pattern: `LOGEND \d`,
			Input: func() []byte {
				newInput := splittest.GenerateBytes(100)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				string(splittest.GenerateBytes(100)) + `LOGEND 1`,
			},
		},
		{
			Name:    "HugeLog10000",
			Pattern: `LOGEND \d`,
			Input: func() []byte {
				newInput := splittest.GenerateBytes(10000)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedTokens: []string{
				string(splittest.GenerateBytes(10000)) + `LOGEND 1`,
			},
		},
		{
			Name:    "HugeLog1000000",
			Pattern: `LOGEND \d`,
			Input: func() []byte {
				newInput := splittest.GenerateBytes(1000000)
				newInput = append(newInput, []byte(`LOGEND 1 `)...)
				return newInput
			}(),
			ExpectedError: errors.New("bufio.Scanner: token too long"),
		},
		{
			Name:    "MultiplesplitLogs",
			Pattern: `^LOGEND.*$`,
			Input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2\nLOGSTART 43 log5"),
			ExpectedTokens: []string{
				"LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   ",
				"\nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2",
			},
		},
		{
			Name:    "NoMatch",
			Pattern: `^LOGEND.*$`,
			Input:   []byte("LOGPART log1\nLOGPART log1\t   \n"),
		},
	}

	for _, tc := range testCases {
		cfg := Config{LineEndPattern: tc.Pattern}
		splitFunc, err := cfg.Func(unicode.UTF8, false, 0, trim.Nop)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}

	t.Run("FlushAtEOF", func(t *testing.T) {
		cfg := &Config{
			LineEndPattern: `^LOGSTART \d+`,
		}
		splitFunc, err := cfg.Func(unicode.UTF8, true, 0, trim.Nop)
		require.NoError(t, err)
		splittest.TestCase{
			Name:           "NoMatch",
			Input:          []byte("LOGPART log1\nLOGPART log1\t   \n"),
			ExpectedTokens: []string{"LOGPART log1\nLOGPART log1\t   \n"},
		}.Run(splitFunc)(t)
	})

	t.Run("ApplyTrimFunc", func(t *testing.T) {
		cfg := Config{LineEndPattern: ` LOGEND `}
		input := []byte(" log1 LOGEND  log2 LOGEND ")

		splitTrimLeading, err := cfg.Func(unicode.UTF8, false, 0, trim.Leading)
		require.NoError(t, err)
		t.Run("TrimLeading", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`log1 LOGEND `,
				`log2 LOGEND `,
			}}.Run(splitTrimLeading))

		splitTrimTrailing, err := cfg.Func(unicode.UTF8, false, 0, trim.Trailing)
		require.NoError(t, err)
		t.Run("TrimTrailing", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				` log1 LOGEND`,
				` log2 LOGEND`,
			}}.Run(splitTrimTrailing))

		splitTrimBoth, err := cfg.Func(unicode.UTF8, false, 0, trim.Whitespace)
		require.NoError(t, err)
		t.Run("TrimBoth", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`log1 LOGEND`,
				`log2 LOGEND`,
			}}.Run(splitTrimBoth))
	})
}

func TestNewlineSplitFunc(t *testing.T) {
	testCases := []splittest.TestCase{
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
				newInput := splittest.GenerateBytes(100)
				newInput = append(newInput, '\n')
				return newInput
			}(),
			ExpectedTokens: []string{
				string(splittest.GenerateBytes(100)),
			},
		},
		{
			Name: "HugeLog10000",
			Input: func() []byte {
				newInput := splittest.GenerateBytes(10000)
				newInput = append(newInput, '\n')
				return newInput
			}(),
			ExpectedTokens: []string{
				string(splittest.GenerateBytes(10000)),
			},
		},
		{
			Name: "HugeLog1000000",
			Input: func() []byte {
				newInput := splittest.GenerateBytes(1000000)
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
			Name:  "DefaultSplits",
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
	}

	for _, tc := range testCases {
		splitFunc, err := NewlineSplitFunc(unicode.UTF8, false, trim.Nop)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}

	t.Run("FlushAtEOF", func(t *testing.T) {
		splitFunc, err := Config{}.Func(unicode.UTF8, true, 0, trim.Nop)
		require.NoError(t, err)
		splittest.TestCase{
			Name:           "FlushAtEOF",
			Input:          []byte("log1\nlog2"),
			ExpectedTokens: []string{"log1", "log2"},
		}.Run(splitFunc)(t)
	})

	t.Run("ApplyTrimFunc", func(t *testing.T) {
		cfg := &Config{}
		input := []byte(" log1 \n log2 \n")

		splitTrimLeading, err := cfg.Func(unicode.UTF8, false, 0, trim.Leading)
		require.NoError(t, err)
		t.Run("TrimLeading", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`log1 `,
				`log2 `,
			}}.Run(splitTrimLeading))

		splitTrimTrailing, err := cfg.Func(unicode.UTF8, false, 0, trim.Trailing)
		require.NoError(t, err)
		t.Run("TrimTrailing", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				` log1`,
				` log2`,
			}}.Run(splitTrimTrailing))

		splitTrimBoth, err := cfg.Func(unicode.UTF8, false, 0, trim.Whitespace)
		require.NoError(t, err)
		t.Run("TrimBoth", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`log1`,
				`log2`,
			}}.Run(splitTrimBoth))
	})
}

func TestNoSplitFunc(t *testing.T) {
	const largeLogSize = 100
	testCases := []splittest.TestCase{
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
				return splittest.GenerateBytes(largeLogSize)
			}(),
			ExpectedTokens: []string{string(splittest.GenerateBytes(largeLogSize))},
		},
		{
			Name: "HugeLog300",
			Input: func() []byte {
				return splittest.GenerateBytes(largeLogSize * 3)
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
				return splittest.GenerateBytes(largeLogSize * 3.5)
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
	endCfg := Config{LineEndPattern: "\n"}
	_, err := endCfg.Func(encoding.Nop, false, 0, trim.Nop)
	require.Equal(t, err, fmt.Errorf("line_start_pattern or line_end_pattern should not be set when using nop encoding"))

	startCfg := Config{LineStartPattern: "\n"}
	_, err = startCfg.Func(encoding.Nop, false, 0, trim.Nop)
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

// TODO the only error case is when an encoding requires more than 10
// bytes to represent a newline or carriage return. This is very unlikely to happen
// and doesn't seem to warrant error handling in more than one place.
// Therefore, move error case into decoder.LookupEncoding?
type errEncoding struct{}
type errEncoder struct{}

func (e errEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{
		Transformer: &errEncoder{},
	}
}

func (e errEncoding) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{}
}

func (e errEncoder) Transform(_, _ []byte, _ bool) (int, int, error) {
	return 0, 0, errors.New("strange encoding")
}

func (e errEncoder) Reset() {}
