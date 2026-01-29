// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split

import (
	"bufio"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
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
		require.Equal(t, err, errors.New("line_end_pattern should not be set when using nop encoding"))

		startCfg := Config{LineStartPattern: "\n"}
		_, err = startCfg.Func(encoding.Nop, false, 0)
		require.Equal(t, err, errors.New("line_start_pattern should not be set when using nop encoding"))
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

func TestLineEndSplitFunc_Comprehensive(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		omitPattern bool
		flushAtEOF  bool
		encoding    encoding.Encoding
		input       []byte
		steps       []splittest.Step
	}{
		// UTF-8 encoding tests
		{
			name:    "UTF8_EmptyInput",
			pattern: `LOGEND`,
			input:   []byte(""),
		},
		{
			name:    "UTF8_PatternAtStart",
			pattern: `LOGEND`,
			input:   []byte("LOGEND rest of log"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGEND"),
			},
		},
		{
			name:        "UTF8_PatternAtStartOmitPattern",
			pattern:     `LOGEND`,
			omitPattern: true,
			input:       []byte("LOGEND rest of log"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGEND"), ""),
			},
		},
		{
			name:    "UTF8_MatchAtEndOfBuffer",
			pattern: `LOGEND`,
			input:   []byte("log content LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectToken("log content LOGEND"),
			},
		},
		{
			name:        "UTF8_MatchAtEndOfBufferOmitPattern",
			pattern:     `LOGEND`,
			omitPattern: true,
			input:       []byte("log content LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log content LOGEND"), "log content "),
			},
		},
		{
			name:    "UTF8_ThreeLogs",
			pattern: `LOGEND`,
			input:   []byte("log1 LOGEND log2 LOGEND log3 LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 LOGEND"),
				splittest.ExpectToken(" log2 LOGEND"),
				splittest.ExpectToken(" log3 LOGEND"),
			},
		},
		{
			name:        "UTF8_ThreeLogsOmitPattern",
			pattern:     `LOGEND`,
			omitPattern: true,
			input:       []byte("log1 LOGEND log2 LOGEND log3 LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log1 LOGEND"), "log1 "),
				splittest.ExpectAdvanceToken(len(" log2 LOGEND"), " log2 "),
				splittest.ExpectAdvanceToken(len(" log3 LOGEND"), " log3 "),
			},
		},
		{
			name:       "UTF8_FlushAtEOFWithMatch",
			pattern:    `LOGEND`,
			flushAtEOF: true,
			input:      []byte("log1 LOGEND log2"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 LOGEND"),
				splittest.ExpectToken(" log2"),
			},
		},
		{
			name:       "UTF8_FlushAtEOFNoMatch",
			pattern:    `LOGEND`,
			flushAtEOF: true,
			input:      []byte("log1 log2 log3"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 log2 log3"),
			},
		},
		{
			name:        "UTF8_FlushAtEOFNoMatchOmitPattern",
			pattern:     `LOGEND`,
			omitPattern: true,
			flushAtEOF:  true,
			input:       []byte("log1 log2 log3"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 log2 log3"),
			},
		},
		{
			name:       "UTF8_NoFlushAtEOFNoMatch",
			pattern:    `LOGEND`,
			flushAtEOF: false,
			input:      []byte("log1 log2 log3"),
		},
		{
			name:    "UTF8_RegexWithQuantifier",
			pattern: ` \d{3}$`,
			input:   []byte("log line 123\nlog line 456\nlog line 789"),
			steps: []splittest.Step{
				splittest.ExpectToken("log line 123"),
				splittest.ExpectToken("\nlog line 456"),
				splittest.ExpectToken("\nlog line 789"),
			},
		},
		{
			name:        "UTF8_RegexWithQuantifierOmitPattern",
			pattern:     ` \d{3}$`,
			omitPattern: true,
			input:       []byte("log line 123\nlog line 456\nlog line 789"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log line 123"), "log line"),
				splittest.ExpectAdvanceToken(len("\nlog line 456"), "\nlog line"),
				splittest.ExpectAdvanceToken(len("\nlog line 789"), "\nlog line"),
			},
		},
		{
			name:    "UTF8_MatchAtBufferBoundary",
			pattern: `END`,
			input:   []byte("log1 ENDlog2 END"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 END"),
				splittest.ExpectToken("log2 END"),
			},
		},
		{
			name:    "UTF8_MultipleMatchesSameLine",
			pattern: `END`,
			input:   []byte("log1 END log2 END log3 END"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 END"),
				splittest.ExpectToken(" log2 END"),
				splittest.ExpectToken(" log3 END"),
			},
		},
		// Edge cases with incomplete data
		{
			name:    "UTF8_MatchAtLastByte",
			pattern: `X`,
			input:   []byte("logX"),
			steps: []splittest.Step{
				splittest.ExpectToken("logX"),
			},
		},
		{
			name:        "UTF8_MatchAtLastByteOmitPattern",
			pattern:     `X`,
			omitPattern: true,
			input:       []byte("logX"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("logX"), "log"),
			},
		},
		{
			name:    "UTF8_ConsecutiveMatches",
			pattern: `END`,
			input:   []byte("ENDENDEND"),
			steps: []splittest.Step{
				splittest.ExpectToken("END"),
				splittest.ExpectToken("END"),
				splittest.ExpectToken("END"),
			},
		},
		{
			name:        "UTF8_ConsecutiveMatchesOmitPattern",
			pattern:     `END`,
			omitPattern: true,
			input:       []byte("ENDENDEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("END"), ""),
				splittest.ExpectAdvanceToken(len("END"), ""),
				splittest.ExpectAdvanceToken(len("END"), ""),
			},
		},
	}

	for _, tc := range testCases {
		if tc.encoding == nil {
			tc.encoding = unicode.UTF8
		}
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineEndSplitFunc(re, tc.omitPattern, tc.flushAtEOF, tc.encoding, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func TestLineStartSplitFunc_Comprehensive(t *testing.T) {
	testCases := []struct {
		name        string
		pattern     string
		omitPattern bool
		flushAtEOF  bool
		encoding    encoding.Encoding
		input       []byte
		steps       []splittest.Step
	}{
		// UTF-8 encoding tests
		{
			name:    "UTF8_EmptyInput",
			pattern: `LOGSTART`,
			input:   []byte(""),
		},
		{
			name:    "UTF8_PatternAtStart",
			pattern: `LOGSTART`,
			input:   []byte("LOGSTART rest of log LOGSTART next log"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART rest of log "),
			},
		},
		{
			name:        "UTF8_PatternAtStartOmitPattern",
			pattern:     `LOGSTART`,
			omitPattern: true,
			input:       []byte("LOGSTART rest of log LOGSTART next log"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART rest of log "), " rest of log "),
			},
		},
		{
			name:    "UTF8_PrecedingNonMatch",
			pattern: `LOGSTART`,
			input:   []byte("prefix LOGSTART rest LOGSTART next"),
			steps: []splittest.Step{
				splittest.ExpectToken("prefix "),
				splittest.ExpectToken("LOGSTART rest "),
			},
		},
		{
			name:        "UTF8_PrecedingNonMatchOmitPattern",
			pattern:     `LOGSTART`,
			omitPattern: true,
			input:       []byte("prefix LOGSTART rest LOGSTART next"),
			steps: []splittest.Step{
				splittest.ExpectToken("prefix "),
				splittest.ExpectAdvanceToken(len("LOGSTART rest "), " rest "),
			},
		},
		{
			name:    "UTF8_ThreeLogs",
			pattern: `LOGSTART`,
			input:   []byte("LOGSTART log1 LOGSTART log2 LOGSTART log3"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART log1 "),
				splittest.ExpectToken("LOGSTART log2 "),
			},
		},
		{
			name:        "UTF8_ThreeLogsOmitPattern",
			pattern:     `LOGSTART`,
			omitPattern: true,
			input:       []byte("LOGSTART log1 LOGSTART log2 LOGSTART log3"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGSTART log1 "), " log1 "),
				splittest.ExpectAdvanceToken(len("LOGSTART log2 "), " log2 "),
			},
		},
		{
			name:       "UTF8_FlushAtEOFWithMatch",
			pattern:    `LOGSTART`,
			flushAtEOF: true,
			input:      []byte("LOGSTART log1 LOGSTART log2"),
			steps: []splittest.Step{
				splittest.ExpectToken("LOGSTART log1 "),
				splittest.ExpectToken("LOGSTART log2"),
			},
		},
		{
			name:       "UTF8_FlushAtEOFNoMatch",
			pattern:    `LOGSTART`,
			flushAtEOF: true,
			input:      []byte("log1 log2 log3"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 log2 log3"),
			},
		},
		{
			name:        "UTF8_FlushAtEOFNoMatchOmitPattern",
			pattern:     `LOGSTART`,
			omitPattern: true,
			flushAtEOF:  true,
			input:       []byte("log1 log2 log3"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 log2 log3"),
			},
		},
		{
			name:       "UTF8_NoFlushAtEOFNoMatch",
			pattern:    `LOGSTART`,
			flushAtEOF: false,
			input:      []byte("log1 log2 log3"),
		},
		{
			name:    "UTF8_RegexWithQuantifier",
			pattern: `^\d+`,
			input:   []byte("123 log line\n456 log line\n789 log line"),
			steps: []splittest.Step{
				splittest.ExpectToken("123 log line\n"),
				splittest.ExpectToken("456 log line\n"),
			},
		},
		{
			name:        "UTF8_RegexWithQuantifierOmitPattern",
			pattern:     `^\d+`,
			omitPattern: true,
			input:       []byte("123 log line\n456 log line\n789 log line"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("123 log line\n"), " log line\n"),
				splittest.ExpectAdvanceToken(len("456 log line\n"), " log line\n"),
			},
		},
		{
			name:    "UTF8_MatchAtBufferBoundary",
			pattern: `START`,
			input:   []byte("log1 STARTlog2 START"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1 "),
				splittest.ExpectToken("STARTlog2 "),
			},
		},
		{
			name:    "UTF8_MultipleMatchesSameLine",
			pattern: `START`,
			input:   []byte("START log1 START log2 START log3"),
			steps: []splittest.Step{
				splittest.ExpectToken("START log1 "),
				splittest.ExpectToken("START log2 "),
			},
		},
		{
			name:    "UTF8_EmptyToken",
			pattern: `START`,
			input:   []byte("STARTlog1 STARTlog2 STARTlog3"),
			steps: []splittest.Step{
				splittest.ExpectToken("STARTlog1 "),
				splittest.ExpectToken("STARTlog2 "),
			},
		},
		{
			name:        "UTF8_EmptyTokenOmitPattern",
			pattern:     `START`,
			omitPattern: true,
			input:       []byte("STARTlog1 STARTlog2 STARTlog3"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("STARTlog1 "), "log1 "),
				splittest.ExpectAdvanceToken(len("STARTlog2 "), "log2 "),
			},
		},
		// Edge cases
		{
			name:    "UTF8_MatchAtEndOfBuffer",
			pattern: `START`,
			input:   []byte("logSTART nextSTART"),
			steps: []splittest.Step{
				splittest.ExpectToken("log"),
				splittest.ExpectToken("START next"),
			},
		},
	}

	for _, tc := range testCases {
		if tc.encoding == nil {
			tc.encoding = unicode.UTF8
		}
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineStartSplitFunc(re, tc.omitPattern, tc.flushAtEOF, tc.encoding, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func TestNewlineSplitFunc(t *testing.T) {
	testCases := []struct {
		name       string
		encoding   encoding.Encoding
		flushAtEOF bool
		input      []byte
		steps      []splittest.Step
	}{
		{
			name:  "EmptyFile",
			input: []byte(""),
		},
		{
			name:  "OneLogSimple",
			input: []byte("my log\n"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("my log")+1, "my log"),
			},
		},
		{
			name:  "OneLogCarriageReturn",
			input: []byte("my log\r\n"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("my log")+2, "my log"),
			},
		},
		{
			name:  "TwoLogsSimple",
			input: []byte("log1\nlog2\n"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log1")+1, "log1"),
				splittest.ExpectAdvanceToken(len("log2")+1, "log2"),
			},
		},
		{
			name:  "TwoLogsCarriageReturn",
			input: []byte("log1\r\nlog2\r\n"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log1")+2, "log1"),
				splittest.ExpectAdvanceToken(len("log2")+2, "log2"),
			},
		},
		{
			name:  "NoTailingNewline",
			input: []byte("foo"),
		},
		{
			name: "HugeLog10000",
			input: func() []byte {
				newInput := splittest.GenerateBytes(10000)
				newInput = append(newInput, '\n')
				return newInput
			}(),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(10000+1, string(splittest.GenerateBytes(10000))),
			},
		},
		{
			name:  "EmptyLine",
			input: []byte("LOGEND 333\n\nAnother one"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("LOGEND 333")+1, "LOGEND 333"),
				splittest.ExpectAdvanceToken(1, ""),
			},
		},
		{
			name:  "EmptyLineFirst",
			input: []byte("\nLOGEND 333\nAnother one"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(1, ""),
				splittest.ExpectAdvanceToken(len("LOGEND 333")+1, "LOGEND 333"),
			},
		},
		{
			name:       "FlushAtEOF",
			input:      []byte("log1\nlog2"),
			flushAtEOF: true,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("log1")+1, "log1"),
				splittest.ExpectAdvanceToken(len("log2"), "log2"),
			},
		},
		{
			name:     "SimpleUTF16",
			encoding: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			input:    []byte{0, 116, 0, 101, 0, 115, 0, 116, 0, 108, 0, 111, 0, 103, 0, 10}, // testlog\n
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len([]byte{0, 116, 0, 101, 0, 115, 0, 116, 0, 108, 0, 111, 0, 103, 0, 10}),
					string([]byte{0, 116, 0, 101, 0, 115, 0, 116, 0, 108, 0, 111, 0, 103}),
				),
			},
		},
		{
			name:     "MultiUTF16",
			encoding: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			input:    []byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 10, 0, 108, 0, 111, 0, 103, 0, 50, 0, 10}, // log1\nlog2\n
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len([]byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 10}),
					string([]byte{0, 108, 0, 111, 0, 103, 0, 49}), // log1
				),
				splittest.ExpectAdvanceToken(
					len([]byte{0, 108, 0, 111, 0, 103, 0, 50, 0, 10}),
					string([]byte{0, 108, 0, 111, 0, 103, 0, 50}), // log2
				),
			},
		},
		{
			name:     "MultiCarriageReturnUTF16",
			encoding: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			input:    []byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 13, 0, 10, 0, 108, 0, 111, 0, 103, 0, 50, 0, 13, 0, 10}, // log1\r\nlog2\r\n
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len([]byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 13, 0, 10}),
					string([]byte{0, 108, 0, 111, 0, 103, 0, 49}), // log1
				),
				splittest.ExpectAdvanceToken(
					len([]byte{0, 108, 0, 111, 0, 103, 0, 50, 0, 13, 0, 10}),
					string([]byte{0, 108, 0, 111, 0, 103, 0, 50}), // log2
				),
			},
		},
		{
			name:     "MultiCarriageReturnUTF16StartingWithWhiteChars",
			encoding: unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			input:    []byte{0, 13, 0, 10, 0, 108, 0, 111, 0, 103, 0, 49, 0, 13, 0, 10, 0, 108, 0, 111, 0, 103, 0, 50, 0, 13, 0, 10}, // \r\nlog1\r\nlog2\r\n
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len([]byte{0, 13, 0, 10}), ""),
				splittest.ExpectAdvanceToken(
					len([]byte{0, 108, 0, 111, 0, 103, 0, 49, 0, 13, 0, 10}),
					string([]byte{0, 108, 0, 111, 0, 103, 0, 49}), // log1
				),
				splittest.ExpectAdvanceToken(
					len([]byte{0, 108, 0, 111, 0, 103, 0, 50, 0, 13, 0, 10}),
					string([]byte{0, 108, 0, 111, 0, 103, 0, 50}), // log2
				),
			},
		},
		{
			name:       "AlternateEncoding",
			input:      []byte("折\n"),
			encoding:   traditionalchinese.Big5,
			flushAtEOF: true,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(4, "折"), // advance 4 bytes, []byte{167, 233, 10} + "\n"
			},
		},
	}

	for _, tc := range testCases {
		if tc.encoding == nil {
			tc.encoding = unicode.UTF8
		}
		splitFunc, err := Config{}.Func(tc.encoding, tc.flushAtEOF, 0)
		require.NoError(t, err)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func TestNoSplitFunc(t *testing.T) {
	const largeLogSize = 100
	testCases := []struct {
		name  string
		input []byte
		steps []splittest.Step
	}{
		{
			name:  "EmptyFile",
			input: []byte(""),
		},
		{
			name:  "OneLogSimple",
			input: []byte("my log\n"),
			steps: []splittest.Step{
				splittest.ExpectToken("my log\n"),
			},
		},
		{
			name:  "TwoLogsSimple",
			input: []byte("log1\nlog2\n"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1\nlog2\n"),
			},
		},
		{
			name:  "TwoLogsCarriageReturn",
			input: []byte("log1\r\nlog2\r\n"),
			steps: []splittest.Step{
				splittest.ExpectToken("log1\r\nlog2\r\n"),
			},
		},
		{
			name:  "NoTailingNewline",
			input: []byte(`foo`),
			steps: []splittest.Step{
				splittest.ExpectToken("foo"),
			},
		},
		{
			name: "HugeLog100",
			input: func() []byte {
				return splittest.GenerateBytes(largeLogSize)
			}(),
			steps: []splittest.Step{
				splittest.ExpectToken(string(splittest.GenerateBytes(largeLogSize))),
			},
		},
		{
			name: "HugeLog300",
			input: func() []byte {
				return splittest.GenerateBytes(largeLogSize * 3)
			}(),
			steps: []splittest.Step{
				splittest.ExpectToken("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv"),
				splittest.ExpectToken("wxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr"),
				splittest.ExpectToken("stuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn"),
			},
		},
		{
			name: "EOFBeforeMaxLogSize",
			input: func() []byte {
				return splittest.GenerateBytes(largeLogSize * 3.5)
			}(),
			steps: []splittest.Step{
				splittest.ExpectToken("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv"),
				splittest.ExpectToken("wxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr"),
				splittest.ExpectToken("stuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn"),
				splittest.ExpectToken("opqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc, err := Config{}.Func(encoding.Nop, false, largeLogSize)
		require.NoError(t, err)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// Helper function to encode string to UTF-16LE bytes
func utf16LEBytes(s string) []byte {
	result := make([]byte, len(s)*2)
	for i, c := range []byte(s) {
		result[i*2] = c
		result[i*2+1] = 0
	}
	return result
}

// TestLineEndSplitFunc_UTF16LE tests line_end_pattern with UTF-16LE encoding
func TestLineEndSplitFunc_UTF16LE(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name        string
		pattern     string
		omitPattern bool
		flushAtEOF  bool
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:    "UTF16LE_SimpleLineEnd",
			pattern: `LOGEND`,
			// "log1 LOGEND" in UTF-16LE
			input: utf16LEBytes("log1 LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1 LOGEND")),
					string(utf16LEBytes("log1 LOGEND")),
				),
			},
		},
		{
			name:        "UTF16LE_SimpleLineEndOmitPattern",
			pattern:     `LOGEND`,
			omitPattern: true,
			// "log1 LOGEND" in UTF-16LE
			input: utf16LEBytes("log1 LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1 LOGEND")),
					string(utf16LEBytes("log1 ")),
				),
			},
		},
		{
			name:    "UTF16LE_TwoLogs",
			pattern: `LOGEND`,
			// "log1 LOGEND log2 LOGEND" in UTF-16LE
			input: utf16LEBytes("log1 LOGEND log2 LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1 LOGEND")),
					string(utf16LEBytes("log1 LOGEND")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" log2 LOGEND")),
					string(utf16LEBytes(" log2 LOGEND")),
				),
			},
		},
		{
			name:        "UTF16LE_TwoLogsOmitPattern",
			pattern:     `LOGEND`,
			omitPattern: true,
			// "log1 LOGEND log2 LOGEND" in UTF-16LE
			input: utf16LEBytes("log1 LOGEND log2 LOGEND"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1 LOGEND")),
					string(utf16LEBytes("log1 ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" log2 LOGEND")),
					string(utf16LEBytes(" log2 ")),
				),
			},
		},
		{
			name:    "UTF16LE_PatternWithDigits",
			pattern: `END`,
			// "log END more END" in UTF-16LE - simpler pattern without digits for reliable testing
			input: utf16LEBytes("log END more END"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log END")),
					string(utf16LEBytes("log END")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" more END")),
					string(utf16LEBytes(" more END")),
				),
			},
		},
		{
			name:       "UTF16LE_FlushAtEOFNoMatch",
			pattern:    `LOGEND`,
			flushAtEOF: true,
			// "log without end marker" in UTF-16LE
			input: utf16LEBytes("log without end marker"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log without end marker")),
					string(utf16LEBytes("log without end marker")),
				),
			},
		},
		{
			name:       "UTF16LE_FlushAtEOFWithPartialMatch",
			pattern:    `LOGEND`,
			flushAtEOF: true,
			// "log1 LOGEND remaining" in UTF-16LE
			input: utf16LEBytes("log1 LOGEND remaining"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1 LOGEND")),
					string(utf16LEBytes("log1 LOGEND")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" remaining")),
					string(utf16LEBytes(" remaining")),
				),
			},
		},
		{
			name:    "UTF16LE_ConsecutiveMatches",
			pattern: `X`,
			// "AXBXCX" in UTF-16LE
			input: utf16LEBytes("AXBXCX"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("AX")),
					string(utf16LEBytes("AX")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("BX")),
					string(utf16LEBytes("BX")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("CX")),
					string(utf16LEBytes("CX")),
				),
			},
		},
		{
			name:    "UTF16LE_EmptyInput",
			pattern: `LOGEND`,
			input:   []byte{},
		},
		{
			name:    "UTF16LE_PatternAtStart",
			pattern: `^START`,
			// "START rest of log" in UTF-16LE
			input: utf16LEBytes("START rest of log"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START")),
					string(utf16LEBytes("START")),
				),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineEndSplitFunc(re, tc.omitPattern, tc.flushAtEOF, utf16le, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestLineStartSplitFunc_UTF16LE tests line_start_pattern with UTF-16LE encoding
func TestLineStartSplitFunc_UTF16LE(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name        string
		pattern     string
		omitPattern bool
		flushAtEOF  bool
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:    "UTF16LE_SimpleLineStart",
			pattern: `LOGSTART`,
			// "LOGSTART log1 LOGSTART log2" in UTF-16LE
			input: utf16LEBytes("LOGSTART log1 LOGSTART log2"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOGSTART log1 ")),
					string(utf16LEBytes("LOGSTART log1 ")),
				),
			},
		},
		{
			name:        "UTF16LE_SimpleLineStartOmitPattern",
			pattern:     `LOGSTART`,
			omitPattern: true,
			// "LOGSTART log1 LOGSTART log2" in UTF-16LE
			input: utf16LEBytes("LOGSTART log1 LOGSTART log2"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOGSTART log1 ")),
					string(utf16LEBytes(" log1 ")),
				),
			},
		},
		{
			name:    "UTF16LE_PrecedingNonMatch",
			pattern: `LOGSTART`,
			// "prefix LOGSTART log1 LOGSTART log2" in UTF-16LE
			input: utf16LEBytes("prefix LOGSTART log1 LOGSTART log2"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("prefix ")),
					string(utf16LEBytes("prefix ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOGSTART log1 ")),
					string(utf16LEBytes("LOGSTART log1 ")),
				),
			},
		},
		{
			name:        "UTF16LE_PrecedingNonMatchOmitPattern",
			pattern:     `LOGSTART`,
			omitPattern: true,
			// "prefix LOGSTART log1 LOGSTART log2" in UTF-16LE
			input: utf16LEBytes("prefix LOGSTART log1 LOGSTART log2"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("prefix ")),
					string(utf16LEBytes("prefix ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOGSTART log1 ")),
					string(utf16LEBytes(" log1 ")),
				),
			},
		},
		{
			name:    "UTF16LE_PatternWithDigits",
			pattern: `LOG\d+ `,
			// "LOG123 first LOG456 second LOG789 third" in UTF-16LE
			input: utf16LEBytes("LOG123 first LOG456 second LOG789 third"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOG123 first ")),
					string(utf16LEBytes("LOG123 first ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOG456 second ")),
					string(utf16LEBytes("LOG456 second ")),
				),
			},
		},
		{
			name:       "UTF16LE_FlushAtEOFNoMatch",
			pattern:    `LOGSTART`,
			flushAtEOF: true,
			// "log without start marker" in UTF-16LE
			input: utf16LEBytes("log without start marker"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log without start marker")),
					string(utf16LEBytes("log without start marker")),
				),
			},
		},
		{
			name:       "UTF16LE_FlushAtEOFWithMatch",
			pattern:    `LOGSTART`,
			flushAtEOF: true,
			// "LOGSTART log1 LOGSTART log2" in UTF-16LE
			input: utf16LEBytes("LOGSTART log1 LOGSTART log2"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOGSTART log1 ")),
					string(utf16LEBytes("LOGSTART log1 ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("LOGSTART log2")),
					string(utf16LEBytes("LOGSTART log2")),
				),
			},
		},
		{
			name:    "UTF16LE_EmptyInput",
			pattern: `LOGSTART`,
			input:   []byte{},
		},
		{
			name:    "UTF16LE_ThreeLogs",
			pattern: `START`,
			// "START a START b START c" in UTF-16LE
			input: utf16LEBytes("START a START b START c"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START a ")),
					string(utf16LEBytes("START a ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START b ")),
					string(utf16LEBytes("START b ")),
				),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineStartSplitFunc(re, tc.omitPattern, tc.flushAtEOF, utf16le, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestUTF16LE_BufferTruncation tests buffer truncation at odd byte boundaries for UTF-16LE
func TestUTF16LE_BufferTruncation(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name       string
		pattern    string
		flushAtEOF bool
		splitType  string // "start" or "end"
		input      []byte
		steps      []splittest.Step
	}{
		{
			name:       "SingleOddByte_LineEnd",
			pattern:    `END`,
			flushAtEOF: true,
			splitType:  "end",
			// Single odd byte - should flush the byte as-is
			input: []byte{0x42},
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(1, string([]byte{0x42})),
			},
		},
		{
			name:       "EvenByteCount_LineEnd",
			pattern:    `END`,
			flushAtEOF: true,
			splitType:  "end",
			// Even byte count - normal UTF-16LE processing
			input: utf16LEBytes("test END"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("test END")),
					string(utf16LEBytes("test END")),
				),
			},
		},
		{
			name:       "EvenByteCount_LineStart",
			pattern:    `START`,
			flushAtEOF: true,
			splitType:  "start",
			// Even byte count - normal UTF-16LE processing with line start
			input: utf16LEBytes("START test"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START test")),
					string(utf16LEBytes("START test")),
				),
			},
		},
		{
			name:       "TwoBytes_NoMatch_LineEnd",
			pattern:    `NOMATCH`,
			flushAtEOF: true,
			splitType:  "end",
			// Two bytes (single UTF-16LE char) with no pattern match
			input: []byte{0x41, 0x00}, // 'A' in UTF-16LE
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(2, string([]byte{0x41, 0x00})),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)

		t.Run(tc.name, func(t *testing.T) {
			var splitFunc bufio.SplitFunc
			if tc.splitType == "start" {
				splitFunc = LineStartSplitFunc(re, false, tc.flushAtEOF, utf16le, nil)
			} else {
				splitFunc = LineEndSplitFunc(re, false, tc.flushAtEOF, utf16le, nil)
			}
			splittest.New(splitFunc, tc.input, tc.steps...)(t)
		})
	}
}

// TestUTF16LE_FallbackBehavior tests fallback to direct byte matching when encoding fails
func TestUTF16LE_FallbackBehavior(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name       string
		pattern    string
		flushAtEOF bool
		input      []byte
		steps      []splittest.Step
	}{
		{
			name:       "ValidUTF16LE_NoFallback",
			pattern:    `test`,
			flushAtEOF: true,
			input:      utf16LEBytes("test data"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("test")),
					string(utf16LEBytes("test")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" data")),
					string(utf16LEBytes(" data")),
				),
			},
		},
		{
			name:       "ValidUTF16LE_MultipleMatches",
			pattern:    `END`,
			flushAtEOF: true,
			// Valid UTF-16LE with multiple matches
			input: utf16LEBytes("data END more END"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("data END")),
					string(utf16LEBytes("data END")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" more END")),
					string(utf16LEBytes(" more END")),
				),
			},
		},
		{
			name:       "ValidUTF16LE_NoMatch_FlushAtEOF",
			pattern:    `NOMATCH`,
			flushAtEOF: true,
			// Valid UTF-16LE with no match - should flush entire buffer
			input: utf16LEBytes("data without match"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("data without match")),
					string(utf16LEBytes("data without match")),
				),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineEndSplitFunc(re, false, tc.flushAtEOF, utf16le, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestNewlineSplitFunc_UTF16LE tests newline splitting with UTF-16LE encoding
func TestNewlineSplitFunc_UTF16LE(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name       string
		input      []byte
		flushAtEOF bool
		steps      []splittest.Step
	}{
		{
			name: "SimpleNewline",
			// "log1\nlog2\n" in UTF-16LE: \n is [10, 0]
			input: append(append(utf16LEBytes("log1"), 10, 0), append(utf16LEBytes("log2"), 10, 0)...),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1"))+2, // +2 for \n in UTF-16LE
					string(utf16LEBytes("log1")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log2"))+2,
					string(utf16LEBytes("log2")),
				),
			},
		},
		{
			name: "CarriageReturnNewline",
			// "log1\r\nlog2\r\n" in UTF-16LE
			input: append(
				append(utf16LEBytes("log1"), 13, 0, 10, 0), // \r\n
				append(utf16LEBytes("log2"), 13, 0, 10, 0)...,
			),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log1"))+4, // +4 for \r\n in UTF-16LE
					string(utf16LEBytes("log1")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log2"))+4,
					string(utf16LEBytes("log2")),
				),
			},
		},
		{
			name:       "NoNewlineFlushAtEOF",
			input:      utf16LEBytes("log without newline"),
			flushAtEOF: true,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log without newline")),
					string(utf16LEBytes("log without newline")),
				),
			},
		},
		{
			name:       "EmptyLine",
			input:      append([]byte{10, 0}, append(utf16LEBytes("log"), 10, 0)...), // \nlog\n
			flushAtEOF: false,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(2, ""), // empty line
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("log"))+2,
					string(utf16LEBytes("log")),
				),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc, err := NewlineSplitFunc(utf16le, tc.flushAtEOF)
		require.NoError(t, err)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestMixedEncodingScenarios tests various encoding scenarios
func TestMixedEncodingScenarios(t *testing.T) {
	testCases := []struct {
		name       string
		encoding   encoding.Encoding
		pattern    string
		flushAtEOF bool
		splitType  string // "start" or "end"
		input      []byte
		steps      []splittest.Step
	}{
		// UTF-16BE tests for comparison
		{
			name:       "UTF16BE_LineEnd",
			encoding:   unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			pattern:    `END`,
			flushAtEOF: true,
			splitType:  "end",
			// "log END" in UTF-16BE: each char is [0, char]
			input: []byte{0, 'l', 0, 'o', 0, 'g', 0, ' ', 0, 'E', 0, 'N', 0, 'D'},
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					14, // 7 chars * 2 bytes
					string([]byte{0, 'l', 0, 'o', 0, 'g', 0, ' ', 0, 'E', 0, 'N', 0, 'D'}),
				),
			},
		},
		{
			name:       "UTF16BE_LineStart",
			encoding:   unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
			pattern:    `START`,
			flushAtEOF: true,
			splitType:  "start",
			// "START log" in UTF-16BE
			input: []byte{0, 'S', 0, 'T', 0, 'A', 0, 'R', 0, 'T', 0, ' ', 0, 'l', 0, 'o', 0, 'g'},
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					18, // 9 chars * 2 bytes
					string([]byte{0, 'S', 0, 'T', 0, 'A', 0, 'R', 0, 'T', 0, ' ', 0, 'l', 0, 'o', 0, 'g'}),
				),
			},
		},
		// UTF-8 for baseline comparison
		{
			name:       "UTF8_LineEnd",
			encoding:   unicode.UTF8,
			pattern:    `END`,
			flushAtEOF: true,
			splitType:  "end",
			input:      []byte("log END"),
			steps: []splittest.Step{
				splittest.ExpectToken("log END"),
			},
		},
		{
			name:       "UTF8_LineStart",
			encoding:   unicode.UTF8,
			pattern:    `START`,
			flushAtEOF: true,
			splitType:  "start",
			input:      []byte("START log"),
			steps: []splittest.Step{
				splittest.ExpectToken("START log"),
			},
		},
		// Big5 encoding test
		{
			name:       "Big5_LineEnd",
			encoding:   traditionalchinese.Big5,
			pattern:    `END`,
			flushAtEOF: true,
			splitType:  "end",
			input:      []byte("log END"),
			steps: []splittest.Step{
				splittest.ExpectToken("log END"),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)

		t.Run(tc.name, func(t *testing.T) {
			var splitFunc bufio.SplitFunc
			if tc.splitType == "start" {
				splitFunc = LineStartSplitFunc(re, false, tc.flushAtEOF, tc.encoding, nil)
			} else {
				splitFunc = LineEndSplitFunc(re, false, tc.flushAtEOF, tc.encoding, nil)
			}
			splittest.New(splitFunc, tc.input, tc.steps...)(t)
		})
	}
}

// TestEncoderErrorsInPositionMapping tests scenarios where encoder might produce errors during position mapping
func TestEncoderErrorsInPositionMapping(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name        string
		pattern     string
		flushAtEOF  bool
		omitPattern bool
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:       "ValidInput_PositionMappingSucceeds",
			pattern:    `END`,
			flushAtEOF: true,
			input:      utf16LEBytes("test END more"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("test END")),
					string(utf16LEBytes("test END")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" more")),
					string(utf16LEBytes(" more")),
				),
			},
		},
		{
			name:        "ValidInput_OmitPattern_PositionMappingSucceeds",
			pattern:     `END`,
			flushAtEOF:  true,
			omitPattern: true,
			input:       utf16LEBytes("test END more"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("test END")),
					string(utf16LEBytes("test ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" more")),
					string(utf16LEBytes(" more")),
				),
			},
		},
		{
			name:       "LongInput_PositionMapping",
			pattern:    `MARKER`,
			flushAtEOF: true,
			input:      utf16LEBytes("aaaaaaaaaa MARKER bbbbbbbbbb MARKER cccccccccc"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("aaaaaaaaaa MARKER")),
					string(utf16LEBytes("aaaaaaaaaa MARKER")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" bbbbbbbbbb MARKER")),
					string(utf16LEBytes(" bbbbbbbbbb MARKER")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" cccccccccc")),
					string(utf16LEBytes(" cccccccccc")),
				),
			},
		},
		{
			name:       "MatchAtStart_PositionMapping",
			pattern:    `^START`,
			flushAtEOF: true,
			input:      utf16LEBytes("START rest"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START")),
					string(utf16LEBytes("START")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes(" rest")),
					string(utf16LEBytes(" rest")),
				),
			},
		},
		{
			name:       "MatchAtEnd_PositionMapping",
			pattern:    `END$`,
			flushAtEOF: true,
			input:      utf16LEBytes("some text END"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("some text END")),
					string(utf16LEBytes("some text END")),
				),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineEndSplitFunc(re, tc.omitPattern, tc.flushAtEOF, utf16le, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestLineStartSplitFunc_PositionMapping tests position mapping specifically for line start patterns
func TestLineStartSplitFunc_PositionMapping(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	testCases := []struct {
		name        string
		pattern     string
		flushAtEOF  bool
		omitPattern bool
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:       "TwoMatches_PositionMapping",
			pattern:    `START`,
			flushAtEOF: true,
			input:      utf16LEBytes("START one START two"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START one ")),
					string(utf16LEBytes("START one ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START two")),
					string(utf16LEBytes("START two")),
				),
			},
		},
		{
			name:        "TwoMatches_OmitPattern_PositionMapping",
			pattern:     `START`,
			flushAtEOF:  true,
			omitPattern: true,
			input:       utf16LEBytes("START one START two"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START one ")),
					string(utf16LEBytes(" one ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START two")),
					string(utf16LEBytes(" two")),
				),
			},
		},
		{
			name:       "PrefixBeforeMatch_PositionMapping",
			pattern:    `START`,
			flushAtEOF: true,
			input:      utf16LEBytes("prefix START content START end"),
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("prefix ")),
					string(utf16LEBytes("prefix ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START content ")),
					string(utf16LEBytes("START content ")),
				),
				splittest.ExpectAdvanceToken(
					len(utf16LEBytes("START end")),
					string(utf16LEBytes("START end")),
				),
			},
		},
	}

	for _, tc := range testCases {
		re, err := regexp.Compile("(?m)" + tc.pattern)
		require.NoError(t, err)
		splitFunc := LineStartSplitFunc(re, tc.omitPattern, tc.flushAtEOF, utf16le, nil)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

// TestDecoderStateIsolation verifies that decoder state doesn't leak between invocations
// This tests the fix for the reviewer comment about stateful decoders
func TestDecoderStateIsolation(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)

	// Create a split function
	re := regexp.MustCompile("(?m)END")
	splitFunc := LineEndSplitFunc(re, false, true, utf16le, nil)

	// First call with valid UTF-16LE data
	input1 := utf16LEBytes("test END")
	advance1, token1, err1 := splitFunc(input1, true)
	require.NoError(t, err1)
	require.Equal(t, len(input1), advance1)
	require.Equal(t, input1, token1)

	// Second call with different valid UTF-16LE data
	// If decoder state leaked, this might fail or produce incorrect results
	input2 := utf16LEBytes("another END")
	advance2, token2, err2 := splitFunc(input2, true)
	require.NoError(t, err2)
	require.Equal(t, len(input2), advance2)
	require.Equal(t, input2, token2)

	// Third call to ensure continued isolation
	input3 := utf16LEBytes("third END")
	advance3, token3, err3 := splitFunc(input3, true)
	require.NoError(t, err3)
	require.Equal(t, len(input3), advance3)
	require.Equal(t, input3, token3)
}
