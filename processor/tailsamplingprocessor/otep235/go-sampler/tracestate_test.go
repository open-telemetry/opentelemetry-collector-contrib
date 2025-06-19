// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package sampler

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestTracestateThresholdUpdate(t *testing.T) {
	type testCase struct {
		tstate       string
		threshold    int64
		randomness   int64
		newThreshold int64
		output       string
	}

	for _, test := range []testCase{
		{
			tstate:       "",
			threshold:    -1,
			randomness:   -1,
			newThreshold: 0,
			output:       "ot=th:0",
		},
		{
			tstate:       "ot=rv:80808080808080",
			threshold:    -1,
			randomness:   0x80808080808080,
			newThreshold: 0,
			output:       "ot=rv:80808080808080;th:0",
		},
		{
			tstate:       "co=whateverr,ed=nowaysir,ot=xx:abc;yy:def;th:0;rv:abcdefabcdefab",
			threshold:    0,
			randomness:   0xabcdefabcdefab,
			newThreshold: 0x8000000000,
			output:       "ot=xx:abc;yy:def;rv:abcdefabcdefab;th:8,co=whateverr,ed=nowaysir",
		},
		{
			tstate:       "ot=xx:abc;yy:def;th:0;rv:abcdefabcdefab,co=whateverr,ed=nowaysir",
			threshold:    0,
			randomness:   0xabcdefabcdefab,
			newThreshold: 0x7c00000000,
			output:       "ot=xx:abc;yy:def;rv:abcdefabcdefab;th:7c,co=whateverr,ed=nowaysir",
		},
		{
			tstate:       "co=whateverr,ot=xx:abc;yy:def;th:0,ed=nowaysir",
			threshold:    0,
			randomness:   -1,
			newThreshold: 0x7c00000000,
			output:       "ot=xx:abc;yy:def;th:7c,co=whateverr,ed=nowaysir",
		},
		{
			tstate:       "co=whateverr,ot=xx:abc;yy:def;th:8,ed=nowaysir",
			threshold:    0x80000000000000,
			randomness:   -1,
			newThreshold: -1,
			output:       "ot=xx:abc;yy:def,co=whateverr,ed=nowaysir",
		},
	} {
		ts, err := trace.ParseTraceState(test.tstate)
		require.NoError(t, err)

		otts := ts.Get("ot")
		threshold, savePos, hasThreshold := tracestateHasThreshold(otts)
		require.Equal(t, test.threshold >= 0, hasThreshold)
		if test.threshold >= 0 {
			require.Equal(t, test.threshold, threshold)
		}

		rnd, hasRandom := tracestateHasRandomness(otts)
		require.Equal(t, test.randomness >= 0, hasRandom)
		if test.randomness >= 0 {
			require.Equal(t, test.randomness, rnd)
		}

		rts, err := combineTracestate(ts, test.newThreshold, test.newThreshold >= 0, threshold, savePos, hasThreshold)
		require.NoError(t, err)
		require.Equal(t, test.output, rts.String())
	}
}
