// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"testing"
)

func BenchmarkSeedConversion(b *testing.B) {
	val := uint32(0x3024001) // Just a random 32 bit int

	for b.Loop() {
		i32tob(val)
	}
}
