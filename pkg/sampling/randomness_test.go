// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestExplicitRandomness(t *testing.T) {
	const val uint64 = 0x80000000000000
	rv, err := UnsignedToRandomness(val)
	require.NoError(t, err)
	require.Equal(t, val, rv.Unsigned())

	rv, err = UnsignedToRandomness(MaxAdjustedCount)
	require.Equal(t, err, ErrRValueSize)
	require.Equal(t, AllProbabilitiesRandomness.Unsigned(), rv.Unsigned())
}

func ExampleTraceIDToRandomness() {
	// TraceID represented in hex as "abababababababababd29d6a7215ced0"
	exampleTid := pcommon.TraceID{
		// 9 meaningless bytes
		0xab, 0xab, 0xab, 0xab, 0xab, 0xab, 0xab, 0xab, 0xab,
		// 7 bytes randomness
		0xd2, 0x9d, 0x6a, 0x72, 0x15, 0xce, 0xd0,
	}
	rnd := TraceIDToRandomness(exampleTid)

	fmt.Printf("TraceIDToRandomness(%q).RValue() = %s", exampleTid, rnd.RValue())

	// Output: TraceIDToRandomness("abababababababababd29d6a7215ced0").RValue() = d29d6a7215ced0
}

func ExampleRValueToRandomness() {
	// Any 14 hex digits is a valid R-value.
	const exampleRvalue = "d29d6a7215ced0"

	// This converts to the internal unsigned integer representation.
	rnd, _ := RValueToRandomness(exampleRvalue)

	// The result prints the same as the input.
	fmt.Printf("RValueToRandomness(%q).RValue() = %s", exampleRvalue, rnd.RValue())

	// Output: RValueToRandomness("d29d6a7215ced0").RValue() = d29d6a7215ced0
}
