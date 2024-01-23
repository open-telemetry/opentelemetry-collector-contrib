// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func ExampleTraceIDToRandomness() {
	var exampleTid = pcommon.TraceID{
		// 9 meaningless bytes
		0x12, 0x34, 0x56, 0x78, 0x91, 0x23, 0x45, 0x67, 0x89,
		// 7 bytes randomness
		0xd2, 0x9d, 0x6a, 0x72, 0x15, 0xce, 0xd0,
	}

	// TraceID represented in hex as
	// "123456789123456789d29d6a7215ced0"
	// from the worked example in OTEP 235.
	rnd := TraceIDToRandomness(exampleTid)

	fmt.Printf("TraceIDToRandomness(%q).RValue() = %s", exampleTid, rnd.RValue())

	// Output: TraceIDToRandomness("123456789123456789d29d6a7215ced0").RValue() = d29d6a7215ced0
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
