// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/client"
)

func BenchmarkMetadataToHeaders(b *testing.B) {
	// Scenario 1: Keys configured but NONE match the metadata.
	// This is the hot path where the fix avoids a wasted allocation.
	b.Run("NoMatch", func(b *testing.B) {
		for _, numKeys := range []int{1, 5, 10} {
			b.Run(fmt.Sprintf("keys=%d", numKeys), func(b *testing.B) {
				keys := make([]string, numKeys)
				for i := range numKeys {
					keys[i] = fmt.Sprintf("x-no-match-%d", i)
				}
				// Metadata has entries, but none match the configured keys
				ctx := client.NewContext(b.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{
						"x-other-key": {"value1"},
					}),
				})
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_ = metadataToHeaders(ctx, keys)
				}
			})
		}
	})

	// Scenario 2: All keys match.
	// Both implementations allocate here; verifies no regression.
	b.Run("AllMatch", func(b *testing.B) {
		for _, numKeys := range []int{1, 5, 10} {
			b.Run(fmt.Sprintf("keys=%d", numKeys), func(b *testing.B) {
				keys := make([]string, numKeys)
				md := make(map[string][]string, numKeys)
				for i := range numKeys {
					k := fmt.Sprintf("x-key-%d", i)
					keys[i] = k
					md[k] = []string{fmt.Sprintf("val-%d", i)}
				}
				ctx := client.NewContext(b.Context(), client.Info{
					Metadata: client.NewMetadata(md),
				})
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					_ = metadataToHeaders(ctx, keys)
				}
			})
		}
	})

	// Scenario 3: Empty keys slice (early return).
	// Baseline -- neither implementation allocates.
	b.Run("NoKeys", func(b *testing.B) {
		ctx := client.NewContext(b.Context(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-key": {"value"},
			}),
		})
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			_ = metadataToHeaders(ctx, nil)
		}
	})
}
