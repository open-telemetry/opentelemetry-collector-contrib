// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestExplicitThreshold(t *testing.T) {
	const val uint64 = 0x80000000000000
	th, err := UnsignedToThreshold(val)
	require.NoError(t, err)
	require.Equal(t, val, th.Unsigned())

	th, err = UnsignedToThreshold(MaxAdjustedCount)
	require.Equal(t, err, ErrTValueSize)
	require.Equal(t, MaxAdjustedCount, th.Unsigned())

	// Note: the input is more out-of-range than above, test th ==
	// NeverSampleThreshold.
	th, err = UnsignedToThreshold(MaxAdjustedCount + 10)
	require.Equal(t, err, ErrTValueSize)
	require.Equal(t, NeverSampleThreshold, th)
}

// ExampleTValueToThreshold demonstrates how to convert a T-value
// string to a Threshold value.
func ExampleTValueToThreshold() {
	// "c" corresponds with rejecting 3/4 traces (or 0xc out of
	// 0x10), which is 25% sampling.
	const exampleTvalue = "c"

	tval, _ := TValueToThreshold(exampleTvalue)

	fmt.Printf("Probability(%q) = %f", exampleTvalue, tval.Probability())

	// Output:
	// Probability("c") = 0.250000
}

// ExampleTValueToThreshold demonstrates how to calculate whether a
// Threshold, calculated from a T-value, should be sampled at a given
// probability.
func ExampleThreshold_ShouldSample() {
	const exampleTvalue = "c"
	const exampleRvalue = "d29d6a7215ced0"

	tval, _ := TValueToThreshold(exampleTvalue)
	rval, _ := RValueToRandomness(exampleRvalue)

	fmt.Printf("TValueToThreshold(%q).ShouldSample(RValueToRandomness(%q) = %v",
		tval.TValue(), rval.RValue(), tval.ShouldSample(rval))

	// Output:
	// TValueToThreshold("c").ShouldSample(RValueToRandomness("d29d6a7215ced0") = true
}

// ExampleTValueToThreshold_traceid demonstrates how to calculate whether a
// Threshold, calculated from a T-value, should be sampled at a given
// probability.
func ExampleThreshold_ShouldSample_traceid() {
	const exampleTvalue = "c"

	// The leading 9 bytes (18 hex digits) of the TraceID string
	// are not used, only the trailing 7 bytes (14 hex digits,
	// i.e., 56 bits) are used.  Here, the dont-care digits are
	// set to 0xab and the R-value is "bd29d6a7215ced0".
	const exampleHexTraceID = "abababababababababd29d6a7215ced0"
	var tid pcommon.TraceID
	idbytes, _ := hex.DecodeString(exampleHexTraceID)
	copy(tid[:], idbytes)

	tval, _ := TValueToThreshold(exampleTvalue)
	rval := TraceIDToRandomness(tid)

	fmt.Printf("TValueToThreshold(%q).ShouldSample(TraceIDToRandomness(%q) = %v",
		tval.TValue(), exampleHexTraceID, tval.ShouldSample(rval))

	// Output:
	// TValueToThreshold("c").ShouldSample(TraceIDToRandomness("abababababababababd29d6a7215ced0") = true
}
