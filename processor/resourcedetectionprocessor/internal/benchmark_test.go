// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

// generateResource creates a resource with keys in range [start, end[
func generateResource(b *testing.B, start, end int) pcommon.Resource {
	b.Helper()
	count := end - start
	require.GreaterOrEqual(b, count, 0)
	r := pcommon.NewResource()
	r.Attributes().EnsureCapacity(count)
	for i := start; i < end; i++ {
		r.Attributes().PutStr(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}
	return r
}

func BenchmarkMergeResource(b *testing.B) {
	tests := map[string]struct {
		countFrom  int
		countTo    int
		overlapPct int // how identical the two maps are
	}{
		// Scenario 1: Merge into empty map.
		// Measures allocation and bulk insertion speed.
		"From_1_Into_0": {
			countFrom:  1,
			countTo:    0,
			overlapPct: 0,
		},
		"From_1000_Into_0": {
			countFrom:  1000,
			countTo:    0,
			overlapPct: 0,
		},

		// Scenario 2: Merge into an identical map.
		// Measures lookup speed (checking for existing keys).
		"From_1_Into_1_Match": {
			countFrom:  1,
			countTo:    1,
			overlapPct: 100,
		},
		"From_1000_Into_1000_Match": {
			countFrom:  1000,
			countTo:    1000,
			overlapPct: 100,
		},

		// Scenario 3: partial overlap between the maps
		"From_1000_Into_1000_50Pct": {
			countFrom:  1000,
			countTo:    1000,
			overlapPct: 50,
		},
	}

	for name, tc := range tests {
		b.Run(name, func(b *testing.B) {
			toBase := generateResource(b, 0, tc.countTo)

			overlapCount := (tc.countFrom * tc.overlapPct) / 100
			startFrom := max(tc.countTo-overlapCount, 0)
			from := generateResource(b, startFrom, startFrom+tc.countFrom)

			to := pcommon.NewResource()

			// Do we need to reset 'to' every iteration?
			// 1. If destination is empty, we just Clear() (fast).
			// 2. If overlap is 100%, we don't mutate 'to', so NO reset needed.
			// 3. Otherwise (Partial), we mutate, so we MUST reset.
			isPartial := tc.countTo > 0 && tc.overlapPct < 100

			// If we aren't resetting in the loop (Match case), fill it once now.
			if tc.countTo > 0 && !isPartial {
				toBase.CopyTo(to)
			}

			b.ReportAllocs()

			for b.Loop() {
				if tc.countTo == 0 {
					// Fast path: just clear. No need to stop timer.
					to.Attributes().Clear()
				} else if isPartial {
					// Slow path: strictly required for correctness in Partial tests.
					b.StopTimer()
					toBase.CopyTo(to)
					b.StartTimer()
				}

				MergeResource(to, from, false)
			}
		})
	}
}

func BenchmarkResourceProvider(b *testing.B) {
	// Set up a provider with 3 detectors.
	// This forces the 'detectResource' function to manage multiple goroutines and merge them.
	d1 := &benchDetector{res: generateResource(b, 0, 10)}
	d2 := &benchDetector{res: generateResource(b, 0, 10)}
	d3 := &benchDetector{res: generateResource(b, 0, 10)}
	provider := NewResourceProvider(zap.NewNop(), 0, d1, d2, d3)
	ctx := b.Context()
	client := &http.Client{}
	_ = provider.Refresh(ctx, client)

	// Each test: "1 in X operations is a Refresh"
	// 0       = Read Only
	// 100     = 1% Writes (99 Reads / 1 Refresh)
	// 10      = 10% Writes (9 Reads / 1 Refresh)
	tests := []struct {
		name      string
		refreshAt int
	}{
		{
			"ReadOnly",
			0,
		},
		{
			"Mixed_1Pct_Write",
			100,
		},
		{
			"Mixed_10Pct_Write",
			10,
		},
	}

	for _, tc := range tests {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()

			// RunParallel simulates concurrent access
			// (Trace Processors reading + Background Refresher writing)
			b.RunParallel(func(pb *testing.PB) {
				for i := 1; pb.Next(); i++ {
					// If refreshAt > 0 and we hit the interval, Write. Otherwise, Read.
					if tc.refreshAt > 0 && i%tc.refreshAt == 0 {
						_ = provider.Refresh(ctx, client)
					} else {
						_, _, _ = provider.Get(ctx, client)
					}
				}
			})
		})
	}
}

type benchDetector struct {
	// mockDetector uses mock.Mock which introduces overhead
	// that we don't want to consider for the benchmarks,
	// so we create the benchDetector to work around that.
	res pcommon.Resource
}

func (d *benchDetector) Detect(_ context.Context) (pcommon.Resource, string, error) {
	r := pcommon.NewResource()
	d.res.CopyTo(r)
	return r, "http://schema", nil
}
