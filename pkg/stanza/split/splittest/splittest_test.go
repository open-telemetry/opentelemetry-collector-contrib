// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splittest

import (
	"bufio"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	testCases := []struct {
		name      string
		splitFunc bufio.SplitFunc
		input     []byte
		steps     []Step
	}{
		{
			name:      "ScanRunes",
			splitFunc: bufio.ScanRunes,
			input:     []byte("foo, bar!"),
			steps: []Step{
				ExpectToken("f"),
				ExpectToken("o"),
				ExpectToken("o"),
				ExpectToken(","),
				ExpectToken(" "),
				ExpectToken("b"),
				ExpectToken("a"),
				ExpectToken("r"),
				ExpectToken("!"),
			},
		},
		{
			name:      "ScanBytes",
			splitFunc: bufio.ScanBytes,
			input:     []byte("foo, bar!"),
			steps: []Step{
				ExpectToken("f"),
				ExpectToken("o"),
				ExpectToken("o"),
				ExpectToken(","),
				ExpectToken(" "),
				ExpectToken("b"),
				ExpectToken("a"),
				ExpectToken("r"),
				ExpectToken("!"),
			},
		},
		{
			name:      "ScanWords",
			splitFunc: bufio.ScanWords,
			input:     []byte("a be see four five!"),
			steps: []Step{
				ExpectAdvanceToken(len("a "), "a"),
				ExpectAdvanceToken(len("be "), "be"),
				ExpectAdvanceToken(len("see "), "see"),
				ExpectAdvanceToken(len("four "), "four"),
				ExpectAdvanceToken(len("five!"), "five!"),
			},
		},
		{
			name:      "ScanLines",
			splitFunc: bufio.ScanLines,
			input:     []byte("a\nbe\nsee\nfour\nfive!"),
			steps: []Step{
				ExpectAdvanceToken(len("a\n"), "a"),
				ExpectAdvanceToken(len("be\n"), "be"),
				ExpectAdvanceToken(len("see\n"), "see"),
				ExpectAdvanceToken(len("four\n"), "four"),
				ExpectAdvanceToken(len("five!"), "five!"),
			},
		},
		{
			name:      "ScanWordsEmpty",
			splitFunc: bufio.ScanWords,
			input:     []byte("   "),
			steps: []Step{
				ExpectAdvanceNil(1),
				ExpectAdvanceNil(1),
				ExpectAdvanceNil(1),
			},
		},
		{
			name:      "ScanLinesEmpty",
			splitFunc: bufio.ScanLines,
			input:     []byte("\n\n\n"),
			steps: []Step{
				ExpectAdvanceToken(1, ""),
				ExpectAdvanceToken(1, ""),
				ExpectAdvanceToken(1, ""),
			},
		},
		{
			name:      "ScanLinesNoEOF",
			splitFunc: ScanLinesStrict,
			input:     []byte("foo bar.\nhello world!\nincomplete line"),
			steps: []Step{
				ExpectAdvanceToken(len("foo bar.\n"), "foo bar."),
				ExpectAdvanceToken(len("hello world!\n"), "hello world!"),
			},
		},
		{
			name:      "ScanLinesError",
			splitFunc: scanLinesError,
			input:     []byte("foo bar.\nhello error!\n"),
			steps: []Step{
				ExpectAdvanceToken(len("foo bar.\n"), "foo bar."),
				ExpectError("hello error!"),
			},
		},
		{
			name:      "ScanLinesStrictWithFlush",
			splitFunc: scanLinesStrictWithFlush(100 * time.Millisecond),
			input:     []byte("foo bar.\nhello world!\nincomplete line"),
			steps: []Step{
				ExpectAdvanceToken(len("foo bar.\n"), "foo bar."),
				ExpectAdvanceToken(len("hello world!\n"), "hello world!"),
				ExpectReadMore(),
				Eventually(ExpectToken("incomplete line"), 150*time.Millisecond, 10*time.Millisecond),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, New(tc.splitFunc, tc.input, tc.steps...))
	}
}

func scanLinesError(data []byte, atEOF bool) (advance int, token []byte, err error) {
	advance, token, err = bufio.ScanLines(data, atEOF)
	if strings.Contains(string(token), "error") {
		return advance, token, errors.New(string(token))
	}
	return
}

func scanLinesStrictWithFlush(flushPeriod time.Duration) bufio.SplitFunc {
	now := time.Now()
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = ScanLinesStrict(data, atEOF)
		if advance > 0 || token != nil || err != nil {
			return
		}
		if time.Since(now) > flushPeriod {
			now = time.Now()
			return len(data), data, nil
		}
		return 0, nil, nil
	}
}
