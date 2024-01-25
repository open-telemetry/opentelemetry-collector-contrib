// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"

import "fmt"

func ExampleTValueToThreshold() {
	// "c" corresponds with rejecting 3/4 traces, or rejecting 0xc
	// out of 0x10, or 25% sampling.
	const exampleTvalue = "c"

	tval, _ := TValueToThreshold(exampleTvalue)

	fmt.Printf("Probability(%q) = %f", exampleTvalue, tval.Probability())

	// Output: Probability("c") = 0.250000
}

func ExampleThreshold_ShouldSample() {
	const exampleTvalue = "c"
	const exampleRvalue = "d29d6a7215ced0"

	tval, _ := TValueToThreshold(exampleTvalue)
	rval, _ := RValueToRandomness(exampleRvalue)

	fmt.Printf("TValueToThreshold(%q).ShouldSample(RValueToRandomness(%q) = %v",
		tval.TValue(), rval.RValue(), tval.ShouldSample(rval))

	// Output: TValueToThreshold("c").ShouldSample(RValueToRandomness("d29d6a7215ced0") = true
}
