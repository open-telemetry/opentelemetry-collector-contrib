// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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
		cfg := Config{LineStartPattern: "foo", LineEndPattern: "bar"}
		_, err := cfg.Func(unicode.UTF8, false, maxLogSize)
		assert.EqualError(t, err, "only one of line_start_pattern or line_end_pattern can be set")
	})

	t.Run("NopEncoding", func(t *testing.T) {
		cfg := Config{}
		f, err := cfg.Func(encoding.Nop, false, maxLogSize)
		assert.NoError(t, err)

		raw := splittest.GenerateBytes(maxLogSize * 2)
		advance, token, err := f(raw, false)
		assert.NoError(t, err)
		assert.Equal(t, maxLogSize, advance)
		assert.Equal(t, raw[:maxLogSize], token)
	})

	t.Run("NopEncodingError", func(t *testing.T) {
		endCfg := Config{LineEndPattern: "\n"}
		_, err := endCfg.Func(encoding.Nop, false, 0)
		require.Equal(t, err, fmt.Errorf("line_end_pattern should not be set when using nop encoding"))

		startCfg := Config{LineStartPattern: "\n"}
		_, err = startCfg.Func(encoding.Nop, false, 0)
		require.Equal(t, err, fmt.Errorf("line_start_pattern should not be set when using nop encoding"))
	})

	t.Run("Newline", func(t *testing.T) {
		cfg := Config{}
		f, err := cfg.Func(unicode.UTF8, false, maxLogSize)
		assert.NoError(t, err)

		advance, token, err := f([]byte("foo\nbar\nbaz\n"), false)
		assert.NoError(t, err)
		assert.Equal(t, 4, advance)
		assert.Equal(t, []byte("foo"), token)
	})

	t.Run("InvalidStartRegex", func(t *testing.T) {
		cfg := Config{LineStartPattern: "["}
		_, err := cfg.Func(unicode.UTF8, false, maxLogSize)
		assert.EqualError(t, err, "compile line start regex: error parsing regexp: missing closing ]: `[`")
	})

	t.Run("InvalidEndRegex", func(t *testing.T) {
		cfg := Config{LineEndPattern: "["}
		_, err := cfg.Func(unicode.UTF8, false, maxLogSize)
		assert.EqualError(t, err, "compile line end regex: error parsing regexp: missing closing ]: `[`")
	})
}

func TestLineStartSplitFunc(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		omitPattern bool
		flushAtEOF  bool
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:    "OneLogSimple",
			pattern: `LOGSTART \d+ `,
			input:   []byte("LOGSTART 123 log1LOGSTART 123 a"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART 123 log1"),
			},
		},
		{
			name:        "OneLogSimpleOmitPattern",
			pattern:     `LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("LOGSTART 123 log1LOGSTART 123 a"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 123 log1"), "log1"),
			},
		},
		{
			name:    "TwoLogsSimple",
			pattern: `LOGSTART \d+ `,
			input:   []byte("LOGSTART 123 log1 LOGSTART 234 log2 LOGSTART 345 foo"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART 123 log1 "),
				splittest.ExpectToken("LOGSTART 234 log2 "),
			},
		},
		{
			name:        "TwoLogsSimpleOmitPattern",
			pattern:     `LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("LOGSTART 123 log1 LOGSTART 234 log2 LOGSTART 345 foo"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 123 log1 "), "log1 "),
				splittest.ExpectAdvanceToken(len("LOGSTART 234 log2 "), "log2 "),
			},
		},
		{
			name:    "TwoLogsLineStart",
			pattern: `^LOGSTART \d+ `,
			input:   []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART 123 LOGSTART 345 log1\n"),
				splittest.ExpectToken("LOGSTART 234 log2\n"),
			},
		},
		{
			name:        "TwoLogsLineStartOmitPattern",
			pattern:     `^LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 123 LOGSTART 345 log1\n"), "LOGSTART 345 log1\n"),
				splittest.ExpectAdvanceToken(len("LOGSTART 234 log2\n"), "log2\n"),
			},
		},
		{
			name:        "TwoLogsLineStartOmitPatternNoStringBeginningToken",
			pattern:     `LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 123 LOGSTART 345 log1\n"), "LOGSTART 345 log1\n"),
				splittest.ExpectAdvanceToken(len("LOGSTART 234 log2\n"), "log2\n"),
			},
		},
		{
			name:    "NoMatches",
			pattern: `LOGSTART \d+ `,
			input:   []byte("file that has no matches in it"),
		},
		{
			name:        "NoMatchesOmitPattern",
			pattern:     `LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("file that has no matches in it"),
		},
		{
			name:    "PrecedingNonMatches",
			pattern: `LOGSTART \d+ `,
			input:   []byte("part that doesn't match LOGSTART 123 part that matchesLOGSTART 123 foo"),
			steps: []splittest.Step{
				splittest.ExpectToken("part that doesn't match "),
				splittest.ExpectToken("LOGSTART 123 part that matches"),
			},
		},
		{
			name:        "PrecedingNonMatchesOmitPattern",
			pattern:     `LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("part that doesn't match LOGSTART 123 part that matchesLOGSTART 123 foo"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("part that doesn't match "), "part that doesn't match "),
				splittest.ExpectAdvanceToken(len("LOGSTART 123 part that matches"), "part that matches"),
			},
		},
		{
			name:    "HugeLog10000",
			pattern: `LOGSTART \d+ `,
			input: func() []byte {
				newInput := []byte(`LOGSTART 123 `)
				newInput = append(newInput, splittest.GenerateBytes(10000)...)
				newInput = append(newInput, []byte(`LOGSTART 234 endlog`)...)
				return newInput
			}(),
			steps: []splittest.Step{
				splittest.ExpectToken(`LOGSTART 123 ` + string(splittest.GenerateBytes(10000))),
			},
		},
		{
			name:        "HugeLog10000OmitPattern",
			pattern:     `LOGSTART \d+ `,
			omitPattern: true,
			input: func() []byte {
				newInput := []byte("LOGSTART 123 ")
				newInput = append(newInput, splittest.GenerateBytes(10000)...)
				newInput = append(newInput, []byte("LOGSTART 234 endlog")...)
				return newInput
			}(),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 123 ")+10000, string(splittest.GenerateBytes(10000))),
			},
		},
		{
			name:    "MultipleMultilineLogs",
			pattern: `^LOGSTART \d+ `,
			input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line\nLOGSTART 43 log5"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \n"),
				splittest.ExpectToken("LOGSTART 17 log2\nLOGPART log2\nanother line\n"),
			},
		},
		{
			name:        "MultipleMultilineLogsOmitPattern",
			pattern:     `^LOGSTART \d+ `,
			omitPattern: true,
			input:       []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line\nLOGSTART 43 log5"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \n"), "log1\t  \nLOGPART log1\nLOGPART log1\t   \n"),
				splittest.ExpectAdvanceToken(len("LOGSTART 17 log2\nLOGPART log2\nanother line\n"), "log2\nLOGPART log2\nanother line\n"),
			},
		},
		{
			name:       "FlushAtEOFNoMatch",
			pattern:    `^LOGSTART \d+ `,
			flushAtEOF: true,
			input:      []byte("LOGPART log1\nLOGPART log1\t   \n"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGPART log1\nLOGPART log1\t   \n"),
			},
		},
		{
			name:        "FlushAtEOFNoMatchOmitPattern",
			pattern:     `^LOGSTART \d+ `,
			omitPattern: true,
			flushAtEOF:  true,
			input:       []byte("LOGPART log1\nLOGPART log1\t   \n"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGPART log1\nLOGPART log1\t   \n"),
			},
		},
		{
			name:       "FlushAtEOFMatchThenNoMatch",
			pattern:    `^LOGSTART \d+ `,
			flushAtEOF: true,
			input:      []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \n"),
				splittest.ExpectToken("LOGSTART 17 log2\nLOGPART log2\nanother line"),
			},
		},
		{
			name:        "FlushAtEOFMatchThenNoMatchOmitPattern",
			pattern:     `^LOGSTART \d+ `,
			omitPattern: true,
			flushAtEOF:  true,
			input:       []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \n"), "log1\t  \nLOGPART log1\nLOGPART log1\t   \n"),
				splittest.ExpectAdvanceToken(len("LOGSTART 17 log2\nLOGPART log2\nanother line"), "log2\nLOGPART log2\nanother line"),
			},
		},
	}

	for _, tc := range testCases {
		cfg := Config{
			LineStartPattern: tc.pattern,
			OmitPattern:      tc.omitPattern,
		}
		splitFunc, err := cfg.Func(unicode.UTF8, tc.flushAtEOF, 0)
		require.NoError(t, err)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func TestLineEndSplitFunc_Detailed(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		omitPattern bool
		flushAtEOF  bool
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:    "OneLogSimple",
			pattern: `LOGEND \d+ `,
			input:   []byte("my log LOGEND 123 "),
			steps: []splittest.Step{
				splittest.ExpectToken("my log LOGEND 123 "),
			},
		},
		{
			name:        "OneLogSimpleOmitPattern",
			pattern:     `LOGEND \d+ `,
			omitPattern: true,
			input:       []byte("my log LOGEND 123 "),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("my log LOGEND 123 "), "my log "),
			},
		},
		{
			name:    "TwoLogsSimple",
			pattern: `LOGEND \d+ `,
			input:   []byte("log1 LOGEND 123 log2 LOGEND 234 "),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 LOGEND 123 "),
				splittest.ExpectToken("log2 LOGEND 234 "),
			},
		},
		{
			name:        "TwoLogsSimpleOmitPattern",
			pattern:     `LOGEND \d+ `,
			omitPattern: true,
			input:       []byte("log1 LOGEND 123 log2 LOGEND 234 "),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log1 LOGEND 123 "), "log1 "),
				splittest.ExpectAdvanceToken(len("log2 LOGEND 234 "), "log2 "),
			},
		},
		{
			name:    "TwoLogsLineEndSimple",
			pattern: `LOGEND$`,
			input:   []byte("log1 LOGEND LOGEND\nlog2 LOGEND\n"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 LOGEND LOGEND"),
				splittest.ExpectToken("\nlog2 LOGEND"),
			},
		},
		{
			name:        "TwoLogsLineEndSimpleOmitPattern",
			pattern:     `LOGEND$`,
			omitPattern: true,
			input:       []byte("log1 LOGEND LOGEND\nlog2 LOGEND\n"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log1 LOGEND LOGEND"), "log1 LOGEND "),
				splittest.ExpectAdvanceToken(len("\nlog2 LOGEND"), "\nlog2 "),
			},
		},
		{
			name:    "NoMatches",
			pattern: `LOGEND \d+ `,
			input:   []byte("file that has no matches in it"),
		},
		{
			name:        "NoMatchesOmitPattern",
			omitPattern: true,
			pattern:     `LOGEND \d+ `,
			input:       []byte("file that has no matches in it"),
		},
		{
			name:    "NonMatchesAfter",
			pattern: `LOGEND \d+ `,
			input:   []byte("part that matches LOGEND 123 part that doesn't match"),
			steps: []splittest.Step{
				splittest.ExpectToken("part that matches LOGEND 123 "),
			},
		},
		{
			name:        "NonMatchesAfterOmitPattern",
			pattern:     `LOGEND \d+ `,
			omitPattern: true,
			input:       []byte("part that matches LOGEND 123 part that doesn't match"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("part that matches LOGEND 123 "), "part that matches "),
			},
		},
		{
			name:    "HugeLog10000",
			pattern: `LOGEND \d `,
			input: func() []byte {
				newInput := splittest.GenerateBytes(10000)
				newInput = append(newInput, []byte("LOGEND 1 ")...)
				return newInput
			}(),
			steps: []splittest.Step{
				splittest.ExpectToken(string(splittest.GenerateBytes(10000)) + "LOGEND 1 "),
			},
		},
		{
			name:        "HugeLog10000OmitPattern",
			pattern:     `LOGEND \d `,
			omitPattern: true,
			input: func() []byte {
				newInput := splittest.GenerateBytes(10000)
				newInput = append(newInput, []byte("LOGEND 1 ")...)
				return newInput
			}(),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(10000+len("LOGEND 1 "), string(splittest.GenerateBytes(10000))),
			},
		},
		{
			name:    "MultiplesplitLogs",
			pattern: `^LOGEND.*\n`,
			input:   []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2\nLOGSTART 43 log5"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \n"),
				splittest.ExpectToken("LOGSTART 17 log2\nLOGPART log2\nLOGEND log2\n"),
			},
		},
		{
			name:        "MultipleMultilineLogsOmitPattern",
			pattern:     `^LOGEND.*\n`,
			omitPattern: true,
			input:       []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2\nLOGSTART 43 log5"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \n"), "LOGSTART 12 log1\t  \nLOGPART log1\n"),
				splittest.ExpectAdvanceToken(len("LOGSTART 17 log2\nLOGPART log2\nLOGEND log2\n"), "LOGSTART 17 log2\nLOGPART log2\n"),
			},
		},
		{
			name:       "FlushAtEOFNoMatch",
			pattern:    `^LOGSTART \d+`,
			flushAtEOF: true,
			input:      []byte("LOGPART log1\nLOGPART log1\t   \n"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGPART log1\nLOGPART log1\t   \n"),
			},
		},
		{
			name:        "FlushAtEOFNoMatchOmitPattern",
			pattern:     `^LOGSTART \d+`,
			omitPattern: true,
			flushAtEOF:  true,
			input:       []byte("LOGPART log1\nLOGPART log1\t   \n"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGPART log1\nLOGPART log1\t   \n"),
			},
		},
	}

	for _, tc := range testCases {
		cfg := Config{
			LineEndPattern: tc.pattern,
			OmitPattern:    tc.omitPattern,
		}
		splitFunc, err := cfg.Func(unicode.UTF8, tc.flushAtEOF, 0)
		require.NoError(t, err)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
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
		splitFunc, err := NewlineSplitFunc(unicode.UTF8, false)
		require.NoError(t, err)
		t.Run(tc.Name, tc.Run(splitFunc))
	}

	t.Run("FlushAtEOF", func(t *testing.T) {
		splitFunc, err := Config{}.Func(unicode.UTF8, true, 0)
		require.NoError(t, err)
		splittest.TestCase{
			Name:           "FlushAtEOF",
			Input:          []byte("log1\nlog2"),
			ExpectedTokens: []string{"log1", "log2"},
		}.Run(splitFunc)(t)
	})

	// // TODO move to internal/splitter?
	t.Run("ApplyTrimFunc", func(t *testing.T) {
		cfg := Config{}
		input := []byte(" log1 \n log2 \n")
		splitFunc, err := cfg.Func(unicode.UTF8, false, 0)
		require.NoError(t, err)

		splitTrimLeading := trim.WithFunc(splitFunc, trim.Leading)
		t.Run("TrimLeading", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				`log1 `,
				`log2 `,
			}}.Run(splitTrimLeading))

		splitTrimTrailing := trim.WithFunc(splitFunc, trim.Trailing)
		t.Run("TrimTrailing", splittest.TestCase{
			Input: input,
			ExpectedTokens: []string{
				` log1`,
				` log2`,
			}}.Run(splitTrimTrailing))

		splitTrimBoth, err := cfg.Func(unicode.UTF8, false, 0)
		require.NoError(t, err)
		splitTrimBoth = trim.WithFunc(splitTrimBoth, trim.Whitespace)
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
			splitFunc, err := NewlineSplitFunc(tc.encoding, false)
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
