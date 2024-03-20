// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"testing"
)

func BenchmarkSeedConversion(b *testing.B) {
	val := uint32(0x3024001) // Just a random 32 bit int
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		i32tob(val)
	}
}
