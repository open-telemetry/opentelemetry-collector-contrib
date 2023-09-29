// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flush

import (
	"bufio"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
)

func TestNewlineSplitFunc(t *testing.T) {
	testCases := []struct {
		name        string
		flushPeriod time.Duration
		baseFunc    bufio.SplitFunc
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:     "FlushNoPeriod",
			input:    []byte("complete line\nincomplete"),
			baseFunc: scanLinesStrict,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("complete line\n"), "complete line"),
			},
		},
		{
			name:        "FlushIncompleteLineAfterPeriod",
			input:       []byte("complete line\nincomplete"),
			baseFunc:    scanLinesStrict,
			flushPeriod: 100 * time.Millisecond,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("complete line\n"), "complete line"),
				splittest.ExpectReadMore(),
				splittest.Eventually(splittest.ExpectToken("incomplete"), 150*time.Millisecond, 10*time.Millisecond),
			},
		},
	}

	for _, tc := range testCases {
		splitFunc := WithPeriod(tc.baseFunc, tc.flushPeriod)
		t.Run(tc.name, splittest.New(splitFunc, tc.input, tc.steps...))
	}
}

func scanLinesStrict(data []byte, atEOF bool) (advance int, token []byte, err error) {
	advance, token, err = bufio.ScanLines(data, atEOF)
	if advance == len(token) {
		return 0, nil, nil
	}
	return
}
