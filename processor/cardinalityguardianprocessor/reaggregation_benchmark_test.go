// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Benchmarks.

func BenchmarkReaggregateNumberDataPoints_NoCollision(b *testing.B) {
	// Benchmark the fast path: no collisions, early exit.
	dps := pmetric.NewNumberDataPointSlice()
	for i := range 100 {
		dp := dps.AppendEmpty()
		dp.Attributes().PutInt("id", int64(i))
		dp.SetIntValue(int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)
	}
}

func BenchmarkReaggregateNumberDataPoints_AllCollide(b *testing.B) {
	// Worst case: all data points share the same identity.
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dps := pmetric.NewNumberDataPointSlice()
		for j := range 100 {
			dp := dps.AppendEmpty()
			dp.Attributes().PutStr("method", "GET")
			dp.SetIntValue(int64(j))
		}
		b.StartTimer()
		reaggregateNumberDataPoints(dps, pmetric.MetricTypeSum, true)
	}
}
