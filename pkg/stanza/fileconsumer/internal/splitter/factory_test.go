// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splitter

import (
	"bufio"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split/splittest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestFactorySplitFunc(t *testing.T) {
	testCases := []struct {
		name        string
		baseFunc    bufio.SplitFunc
		trimFunc    trim.Func
		flushPeriod time.Duration
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:        "ScanLinesStrict",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			trimFunc:    trim.Nop,
			flushPeriod: 0,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
			},
		},
		{
			name:        "ScanLinesStrictWithTrim",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			trimFunc:    trim.Whitespace,
			flushPeriod: 0,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), "hello"),
				splittest.ExpectAdvanceToken(len(" world \n"), "world"),
			},
		},
		{
			name:        "ScanLinesStrictWithFlush",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			trimFunc:    trim.Nop,
			flushPeriod: 100 * time.Millisecond,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
				splittest.ExpectReadMore(),
				splittest.Eventually(
					splittest.ExpectAdvanceToken(len(" extra "), " extra "), 200*time.Millisecond, 10*time.Millisecond,
				),
			},
		},
		{
			name:        "ScanLinesStrictWithTrimAndFlush",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			trimFunc:    trim.Whitespace,
			flushPeriod: 100 * time.Millisecond,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), "hello"),
				splittest.ExpectAdvanceToken(len(" world \n"), "world"),
				splittest.ExpectReadMore(),
				splittest.Eventually(
					splittest.ExpectAdvanceToken(len(" extra "), "extra"), 200*time.Millisecond, 10*time.Millisecond,
				),
			},
		},
	}

	for _, tc := range testCases {
		factory := NewFactory(tc.baseFunc, tc.trimFunc, tc.flushPeriod)
		t.Run(tc.name, splittest.New(factory.SplitFunc(), tc.input, tc.steps...))
	}
}
