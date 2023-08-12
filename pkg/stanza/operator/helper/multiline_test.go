// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/internal"
)

const (
	// Those values has been experimentally figured out for windows
	sleepDuration time.Duration = time.Millisecond * 80
	forcePeriod   time.Duration = time.Millisecond * 40
)

type MultiLineTokenizerTestCase struct {
	internal.TokenizerTestCase
	Flusher *Flusher
}

func TestLineStartSplitFunc(t *testing.T) {
	testCases := []MultiLineTokenizerTestCase{
		{
			internal.TokenizerTestCase{
				Name:    "OneLogSimple",
				Pattern: `LOGSTART \d+ `,
				Raw:     []byte("LOGSTART 123 log1LOGSTART 123 a"),
				ExpectedTokenized: []string{
					`LOGSTART 123 log1`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "TwoLogsSimple",
				Pattern: `LOGSTART \d+ `,
				Raw:     []byte(`LOGSTART 123 log1 LOGSTART 234 log2 LOGSTART 345 foo`),
				ExpectedTokenized: []string{
					`LOGSTART 123 log1`,
					`LOGSTART 234 log2`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "TwoLogsLineStart",
				Pattern: `^LOGSTART \d+ `,
				Raw:     []byte("LOGSTART 123 LOGSTART 345 log1\nLOGSTART 234 log2\nLOGSTART 345 foo"),
				ExpectedTokenized: []string{
					"LOGSTART 123 LOGSTART 345 log1",
					"LOGSTART 234 log2",
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "NoMatches",
				Pattern: `LOGSTART \d+ `,
				Raw:     []byte(`file that has no matches in it`),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "PrecedingNonMatches",
				Pattern: `LOGSTART \d+ `,
				Raw:     []byte(`part that doesn't match LOGSTART 123 part that matchesLOGSTART 123 foo`),
				ExpectedTokenized: []string{
					`part that doesn't match`,
					`LOGSTART 123 part that matches`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "HugeLog100",
				Pattern: `LOGSTART \d+ `,
				Raw: func() []byte {
					newRaw := []byte(`LOGSTART 123 `)
					newRaw = append(newRaw, internal.GeneratedByteSliceOfLength(100)...)
					newRaw = append(newRaw, []byte(`LOGSTART 234 endlog`)...)
					return newRaw
				}(),
				ExpectedTokenized: []string{
					`LOGSTART 123 ` + string(internal.GeneratedByteSliceOfLength(100)),
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "HugeLog10000",
				Pattern: `LOGSTART \d+ `,
				Raw: func() []byte {
					newRaw := []byte(`LOGSTART 123 `)
					newRaw = append(newRaw, internal.GeneratedByteSliceOfLength(10000)...)
					newRaw = append(newRaw, []byte(`LOGSTART 234 endlog`)...)
					return newRaw
				}(),
				ExpectedTokenized: []string{
					`LOGSTART 123 ` + string(internal.GeneratedByteSliceOfLength(10000)),
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "ErrTooLong",
				Pattern: `LOGSTART \d+ `,
				Raw: func() []byte {
					newRaw := []byte(`LOGSTART 123 `)
					newRaw = append(newRaw, internal.GeneratedByteSliceOfLength(1000000)...)
					newRaw = append(newRaw, []byte(`LOGSTART 234 endlog`)...)
					return newRaw
				}(),
				ExpectedError: errors.New("bufio.Scanner: token too long"),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "MultipleMultilineLogs",
				Pattern: `^LOGSTART \d+`,
				Raw:     []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1\t   \nLOGSTART 17 log2\nLOGPART log2\nanother line\nLOGSTART 43 log5"),
				ExpectedTokenized: []string{
					"LOGSTART 12 log1\t  \nLOGPART log1\nLOGPART log1",
					"LOGSTART 17 log2\nLOGPART log2\nanother line",
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithoutFlusher",
				Pattern: `^LOGSTART \d+`,
				Raw:     []byte("LOGPART log1\nLOGPART log1\t   \n"),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithFlusher",
				Pattern: `^LOGSTART \d+`,
				Raw:     []byte("LOGPART log1\nLOGPART log1\t   \n"),
				ExpectedTokenized: []string{
					"LOGPART log1\nLOGPART log1",
				},

				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithFlusherWithMultipleLogsInBuffer",
				Pattern: `^LOGSTART \d+`,
				Raw:     []byte("LOGPART log1\nLOGSTART 123\nLOGPART log1\t   \n"),
				ExpectedTokenized: []string{
					"LOGPART log1",
					"LOGSTART 123\nLOGPART log1",
				},
				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithLongFlusherWithMultipleLogsInBuffer",
				Pattern: `^LOGSTART \d+`,
				Raw:     []byte("LOGPART log1\nLOGSTART 123\nLOGPART log1\t   \n"),
				ExpectedTokenized: []string{
					"LOGPART log1",
				},
				AdditionalIterations: 1,
				Sleep:                forcePeriod / 4,
			},
			&Flusher{
				forcePeriod: forcePeriod * 16,
			},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithFlusherWithLogStartingWithWhiteChars",
				Pattern: `^LOGSTART \d+`,
				Raw:     []byte("\nLOGSTART 333"),
				ExpectedTokenized: []string{
					"",
					"LOGSTART 333",
				},
				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
	}

	for _, tc := range testCases {
		cfg := &MultilineConfig{
			LineStartPattern: tc.Pattern,
		}

		splitFunc, err := cfg.getSplitFunc(unicode.UTF8, false, tc.Flusher, 0, tc.PreserveLeadingWhitespaces, tc.PreserveTrailingWhitespaces)
		require.NoError(t, err)
		t.Run(tc.Name, tc.RunFunc(splitFunc))
	}

	t.Run("FirstMatchHitsEndOfBuffer", func(t *testing.T) {
		splitFunc := NewLineStartSplitFunc(regexp.MustCompile("LOGSTART"), false, noTrim)
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
	testCases := []MultiLineTokenizerTestCase{
		{
			internal.TokenizerTestCase{
				Name:    "OneLogSimple",
				Pattern: `LOGEND \d+`,
				Raw:     []byte(`my log LOGEND 123`),
				ExpectedTokenized: []string{
					`my log LOGEND 123`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "TwoLogsSimple",
				Pattern: `LOGEND \d+`,
				Raw:     []byte(`log1 LOGEND 123log2 LOGEND 234`),
				ExpectedTokenized: []string{
					`log1 LOGEND 123`,
					`log2 LOGEND 234`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "TwoLogsLineEndSimple",
				Pattern: `LOGEND$`,
				Raw:     []byte("log1 LOGEND LOGEND\nlog2 LOGEND\n"),
				ExpectedTokenized: []string{
					"log1 LOGEND LOGEND",
					"log2 LOGEND",
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "NoMatches",
				Pattern: `LOGEND \d+`,
				Raw:     []byte(`file that has no matches in it`),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "NonMatchesAfter",
				Pattern: `LOGEND \d+`,
				Raw:     []byte(`part that matches LOGEND 123 part that doesn't match`),
				ExpectedTokenized: []string{
					`part that matches LOGEND 123`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "HugeLog100",
				Pattern: `LOGEND \d`,
				Raw: func() []byte {
					newRaw := internal.GeneratedByteSliceOfLength(100)
					newRaw = append(newRaw, []byte(`LOGEND 1 `)...)
					return newRaw
				}(),
				ExpectedTokenized: []string{
					string(internal.GeneratedByteSliceOfLength(100)) + `LOGEND 1`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "HugeLog10000",
				Pattern: `LOGEND \d`,
				Raw: func() []byte {
					newRaw := internal.GeneratedByteSliceOfLength(10000)
					newRaw = append(newRaw, []byte(`LOGEND 1 `)...)
					return newRaw
				}(),
				ExpectedTokenized: []string{
					string(internal.GeneratedByteSliceOfLength(10000)) + `LOGEND 1`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "HugeLog1000000",
				Pattern: `LOGEND \d`,
				Raw: func() []byte {
					newRaw := internal.GeneratedByteSliceOfLength(1000000)
					newRaw = append(newRaw, []byte(`LOGEND 1 `)...)
					return newRaw
				}(),
				ExpectedError: errors.New("bufio.Scanner: token too long"),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "MultipleMultilineLogs",
				Pattern: `^LOGEND.*$`,
				Raw:     []byte("LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1\t   \nLOGSTART 17 log2\nLOGPART log2\nLOGEND log2\nLOGSTART 43 log5"),
				ExpectedTokenized: []string{
					"LOGSTART 12 log1\t  \nLOGPART log1\nLOGEND log1",
					"LOGSTART 17 log2\nLOGPART log2\nLOGEND log2",
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithoutFlusher",
				Pattern: `^LOGEND.*$`,
				Raw:     []byte("LOGPART log1\nLOGPART log1\t   \n"),
			},
			&Flusher{},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithFlusher",
				Pattern: `^LOGEND.*$`,
				Raw:     []byte("LOGPART log1\nLOGPART log1\t   \n"),
				ExpectedTokenized: []string{
					"LOGPART log1\nLOGPART log1",
				},

				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithFlusherWithMultipleLogsInBuffer",
				Pattern: `^LOGEND.*$`,
				Raw:     []byte("LOGPART log1\nLOGEND\nLOGPART log1\t   \n"),
				ExpectedTokenized: []string{
					"LOGPART log1\nLOGEND",
					"LOGPART log1",
				},

				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithLongFlusherWithMultipleLogsInBuffer",
				Pattern: `^LOGEND.*$`,
				Raw:     []byte("LOGPART log1\nLOGEND\nLOGPART log1\t   \n"),
				ExpectedTokenized: []string{
					"LOGPART log1\nLOGEND",
				},

				AdditionalIterations: 1,
				Sleep:                forcePeriod / 4,
			},
			&Flusher{
				forcePeriod: forcePeriod * 16,
			},
		},
		{
			internal.TokenizerTestCase{
				Name:    "LogsWithFlusherWithLogStartingWithWhiteChars",
				Pattern: `LOGEND \d+$`,
				Raw:     []byte("\nLOGEND 333"),
				ExpectedTokenized: []string{
					"LOGEND 333",
				},

				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
	}

	for _, tc := range testCases {
		cfg := &MultilineConfig{
			LineEndPattern: tc.Pattern,
		}

		splitFunc, err := cfg.getSplitFunc(unicode.UTF8, false, tc.Flusher, 0, tc.PreserveLeadingWhitespaces, tc.PreserveTrailingWhitespaces)
		require.NoError(t, err)
		t.Run(tc.Name, tc.RunFunc(splitFunc))
	}
}

func TestNewlineSplitFunc(t *testing.T) {
	testCases := []MultiLineTokenizerTestCase{
		{
			internal.TokenizerTestCase{Name: "OneLogSimple",
				Raw: []byte("my log\n"),
				ExpectedTokenized: []string{
					`my log`,
				},
			}, nil,
		},
		{
			internal.TokenizerTestCase{Name: "OneLogCarriageReturn",
				Raw: []byte("my log\r\n"),
				ExpectedTokenized: []string{
					`my log`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "TwoLogsSimple",
				Raw: []byte("log1\nlog2\n"),
				ExpectedTokenized: []string{
					`log1`,
					`log2`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "TwoLogsCarriageReturn",
				Raw: []byte("log1\r\nlog2\r\n"),
				ExpectedTokenized: []string{
					`log1`,
					`log2`,
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "NoTailingNewline",
				Raw: []byte(`foo`),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "HugeLog100",
				Raw: func() []byte {
					newRaw := internal.GeneratedByteSliceOfLength(100)
					newRaw = append(newRaw, '\n')
					return newRaw
				}(),
				ExpectedTokenized: []string{
					string(internal.GeneratedByteSliceOfLength(100)),
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "HugeLog10000",
				Raw: func() []byte {
					newRaw := internal.GeneratedByteSliceOfLength(10000)
					newRaw = append(newRaw, '\n')
					return newRaw
				}(),
				ExpectedTokenized: []string{
					string(internal.GeneratedByteSliceOfLength(10000)),
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "HugeLog1000000",
				Raw: func() []byte {
					newRaw := internal.GeneratedByteSliceOfLength(1000000)
					newRaw = append(newRaw, '\n')
					return newRaw
				}(),
				ExpectedError: errors.New("bufio.Scanner: token too long"),
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "LogsWithoutFlusher",
				Raw: []byte("LOGPART log1"),
			},

			&Flusher{},
		},
		{
			internal.TokenizerTestCase{Name: "LogsWithFlusher",
				Raw: []byte("LOGPART log1"),
				ExpectedTokenized: []string{
					"LOGPART log1",
				},
				AdditionalIterations: 1,
				Sleep:                sleepDuration,
			},
			&Flusher{
				forcePeriod: forcePeriod,
			},
		},
		{
			internal.TokenizerTestCase{Name: "DefaultFlusherSplits",
				Raw: []byte("log1\nlog2\n"),
				ExpectedTokenized: []string{
					"log1",
					"log2",
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "LogsWithLogStartingWithWhiteChars",
				Raw: []byte("\nLOGEND 333\nAnother one"),
				ExpectedTokenized: []string{
					"",
					"LOGEND 333",
				},
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "PreserveLeadingWhitespaces",
				Raw: []byte("\n LOGEND 333 \nAnother one "),
				ExpectedTokenized: []string{
					"",
					" LOGEND 333",
				},
				PreserveLeadingWhitespaces: true,
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "PreserveTrailingWhitespaces",
				Raw: []byte("\n LOGEND 333 \nAnother one "),
				ExpectedTokenized: []string{
					"",
					"LOGEND 333 ",
				},
				PreserveTrailingWhitespaces: true,
			},
			nil,
		},
		{
			internal.TokenizerTestCase{Name: "PreserveBothLeadingAndTrailingWhitespaces",
				Raw: []byte("\n LOGEND 333 \nAnother one "),
				ExpectedTokenized: []string{
					"",
					" LOGEND 333 ",
				},
				PreserveLeadingWhitespaces:  true,
				PreserveTrailingWhitespaces: true,
			},
			nil,
		},
	}

	for _, tc := range testCases {
		splitFunc, err := NewNewlineSplitFunc(unicode.UTF8, false, getTrimFunc(tc.PreserveLeadingWhitespaces, tc.PreserveTrailingWhitespaces))
		require.NoError(t, err)
		if tc.Flusher != nil {
			splitFunc = tc.Flusher.SplitFunc(splitFunc)
		}
		t.Run(tc.Name, tc.RunFunc(splitFunc))
	}
}

type noSplitTestCase struct {
	Name              string
	Raw               []byte
	ExpectedTokenized [][]byte
}

func (tc noSplitTestCase) RunFunc(splitFunc bufio.SplitFunc) func(t *testing.T) {
	return func(t *testing.T) {
		scanner := bufio.NewScanner(bytes.NewReader(tc.Raw))
		scanner.Split(splitFunc)
		var tokenized [][]byte
		for {
			ok := scanner.Scan()
			if !ok {
				break
			}
			tokenized = append(tokenized, scanner.Bytes())
		}

		assert.Equal(t, tc.ExpectedTokenized, tokenized)
	}
}

func TestNoSplitFunc(t *testing.T) {
	const largeLogSize = 100
	testCases := []noSplitTestCase{
		{
			Name: "OneLogSimple",
			Raw:  []byte("my log\n"),
			ExpectedTokenized: [][]byte{
				[]byte("my log\n"),
			},
		},
		{
			Name: "TwoLogsSimple",
			Raw:  []byte("log1\nlog2\n"),
			ExpectedTokenized: [][]byte{
				[]byte("log1\nlog2\n"),
			},
		},
		{
			Name: "TwoLogsCarriageReturn",
			Raw:  []byte("log1\r\nlog2\r\n"),
			ExpectedTokenized: [][]byte{
				[]byte("log1\r\nlog2\r\n"),
			},
		},
		{
			Name:              "NoTailingNewline",
			Raw:               []byte(`foo`),
			ExpectedTokenized: [][]byte{[]byte("foo")},
		},
		{
			Name: "HugeLog100",
			Raw: func() []byte {
				return internal.GeneratedByteSliceOfLength(largeLogSize)
			}(),
			ExpectedTokenized: [][]byte{
				internal.GeneratedByteSliceOfLength(100),
			},
		},
		{
			Name: "HugeLog300",
			Raw: func() []byte {
				return internal.GeneratedByteSliceOfLength(largeLogSize * 3)
			}(),
			ExpectedTokenized: [][]byte{
				[]byte("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv"),
				[]byte("wxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr"),
				[]byte("stuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn"),
			},
		},
		{
			Name: "EOFBeforeMaxLogSize",
			Raw: func() []byte {
				return internal.GeneratedByteSliceOfLength(largeLogSize * 3.5)
			}(),
			ExpectedTokenized: [][]byte{
				[]byte("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuv"),
				[]byte("wxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqr"),
				[]byte("stuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmn"),
				[]byte("opqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijkl"),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc := SplitNone(largeLogSize)
		t.Run(tc.Name, tc.RunFunc(splitFunc))
	}
}

func TestNoopEncodingError(t *testing.T) {
	cfg := &MultilineConfig{
		LineEndPattern: "\n",
	}

	_, err := cfg.getSplitFunc(encoding.Nop, false, nil, 0, false, false)
	require.Equal(t, err, fmt.Errorf("line_start_pattern or line_end_pattern should not be set when using nop encoding"))

	cfg = &MultilineConfig{
		LineStartPattern: "\n",
	}

	_, err = cfg.getSplitFunc(encoding.Nop, false, nil, 0, false, false)
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
			splitFunc, err := NewNewlineSplitFunc(tc.encoding, false, noTrim)
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
