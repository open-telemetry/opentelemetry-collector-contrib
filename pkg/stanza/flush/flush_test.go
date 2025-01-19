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
			baseFunc: splittest.ScanLinesStrict,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("complete line\n"), "complete line"),
			},
		},
		{
			name:        "FlushIncompleteLineAfterPeriod",
			input:       []byte("complete line\nincomplete"),
			baseFunc:    splittest.ScanLinesStrict,
			flushPeriod: 100 * time.Millisecond,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len("complete line\n"), "complete line"),
				splittest.ExpectReadMore(),
				splittest.Eventually(splittest.ExpectToken("incomplete"), 150*time.Millisecond, 10*time.Millisecond),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"/WithPeriod", splittest.New(WithPeriod(tc.baseFunc, tc.flushPeriod), tc.input, tc.steps...))

		previousState := &State{LastDataChange: time.Now()}
		t.Run(tc.name+"/Func", splittest.New(previousState.Func(tc.baseFunc, tc.flushPeriod), tc.input, tc.steps...))
	}
}
