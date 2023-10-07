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
		flushPeriod time.Duration
		maxLength   int
		trimFunc    trim.Func
		input       []byte
		steps       []splittest.Step
	}{
		{
			name:     "ScanLinesStrict",
			input:    []byte(" hello \n world \n extra "),
			baseFunc: splittest.ScanLinesStrict,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
			},
		},
		{
			name:        "ScanLinesStrictWithFlush",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			flushPeriod: 100 * time.Millisecond,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), " hello "),
				splittest.ExpectAdvanceToken(len(" world \n"), " world "),
				splittest.ExpectReadMore(),
				splittest.Eventually(splittest.ExpectAdvanceToken(len(" extra "), " extra "),
					200*time.Millisecond, 10*time.Millisecond),
			},
		},
		{
			name:      "ScanLinesStrictWithMaxLength",
			input:     []byte(" hello \n world \n extra "),
			baseFunc:  splittest.ScanLinesStrict,
			maxLength: 4,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(4, " hel"),
				splittest.ExpectAdvanceToken(4, "lo "),
				splittest.ExpectAdvanceToken(4, " wor"),
				splittest.ExpectAdvanceToken(4, "ld "),
				splittest.ExpectAdvanceToken(4, " ext"),
			},
		},
		{
			name:     "ScanLinesStrictWithTrim",
			input:    []byte(" hello \n world \n extra "),
			baseFunc: splittest.ScanLinesStrict,
			trimFunc: trim.Whitespace,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), "hello"),
				splittest.ExpectAdvanceToken(len(" world \n"), "world"),
			},
		},
		{
			name:        "ScanLinesStrictWithFlushAndMaxLength",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			flushPeriod: 100 * time.Millisecond,
			maxLength:   4,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(4, " hel"),
				splittest.ExpectAdvanceToken(4, "lo "),
				splittest.ExpectAdvanceToken(4, " wor"),
				splittest.ExpectAdvanceToken(4, "ld "),
				splittest.ExpectAdvanceToken(4, " ext"),
				splittest.ExpectReadMore(),
				splittest.Eventually(splittest.ExpectAdvanceToken(3, "ra "),
					200*time.Millisecond, 10*time.Millisecond),
			},
		},
		{
			name:        "ScanLinesStrictWithFlushAndTrim",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			flushPeriod: 100 * time.Millisecond,
			trimFunc:    trim.Whitespace,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(len(" hello \n"), "hello"),
				splittest.ExpectAdvanceToken(len(" world \n"), "world"),
				splittest.ExpectReadMore(),
				splittest.Eventually(splittest.ExpectAdvanceToken(len(" extra "), "extra"),
					200*time.Millisecond, 10*time.Millisecond),
			},
		},
		{
			name:        "ScanLinesStrictWithMaxLengthAndTrim",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			flushPeriod: 0,
			maxLength:   4,
			trimFunc:    trim.Whitespace,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(4, "hel"), // trimmed to length before whitespace
				splittest.ExpectAdvanceToken(4, "lo"),
				splittest.ExpectAdvanceToken(4, "wor"),
				splittest.ExpectAdvanceToken(4, "ld"),
				splittest.ExpectAdvanceToken(4, "ext"),
			},
		},
		{
			name:        "ScanLinesStrictWithFlushAndMaxLengthAndTrim",
			input:       []byte(" hello \n world \n extra "),
			baseFunc:    splittest.ScanLinesStrict,
			flushPeriod: 100 * time.Millisecond,
			maxLength:   4,
			trimFunc:    trim.Whitespace,
			steps: []splittest.Step{
				splittest.ExpectAdvanceToken(4, "hel"),
				splittest.ExpectAdvanceToken(4, "lo"),
				splittest.ExpectAdvanceToken(4, "wor"),
				splittest.ExpectAdvanceToken(4, "ld"),
				splittest.ExpectAdvanceToken(4, "ext"),
				splittest.ExpectReadMore(),
				splittest.Eventually(splittest.ExpectAdvanceToken(3, "ra"),
					200*time.Millisecond, 10*time.Millisecond),
			},
		},
	}

	for _, tc := range testCases {
		factory := NewFactory(tc.baseFunc, tc.trimFunc, tc.flushPeriod, tc.maxLength)
		t.Run(tc.name, splittest.New(factory.SplitFunc(), tc.input, tc.steps...))
	}
}
