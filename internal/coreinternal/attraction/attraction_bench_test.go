// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction

import (
	"fmt"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// newMap builds a deterministic attribute map for benchmarks: some keys match ^secret\.
// and the rest don't.
func newMap(b *testing.B, totalAttrs int, secretRatio float64) pcommon.Map {
	b.Helper()

	secretCount := int(float64(totalAttrs) * secretRatio)
	secretCount = max(secretCount, 0)
	secretCount = min(secretCount, totalAttrs)
	publicCount := totalAttrs - secretCount

	attrs := pcommon.NewMap()
	attrs.EnsureCapacity(totalAttrs)
	for i := range secretCount {
		attrs.PutStr(fmt.Sprintf("secret.k%d", i), "value")
	}
	for i := range publicCount {
		attrs.PutStr(fmt.Sprintf("public.k%d", i), "value")
	}
	return attrs
}

func BenchmarkAttrProc_HashRegex(b *testing.B) {
	ap, err := NewAttrProc(&Settings{
		Actions: []ActionKeyValue{
			{
				RegexPattern: "^secret\\.",
				Action:       HASH,
			},
		},
	})
	if err != nil {
		b.Fatal(err)
	}

	cases := []struct {
		name string
		// totalAttrs is the total number of attributes in the map.
		totalAttrs int
		// secretRatio is the fraction of attributes whose keys start
		// with "secret." (i.e., match ^secret\.).
		secretRatio float64
	}{
		{
			name:        "N20_R10",
			totalAttrs:  20,
			secretRatio: 0.10, // low hashing
		},
		{
			name:        "N20_R50",
			totalAttrs:  20,
			secretRatio: 0.50, // hashing dominates
		},
		{
			name:        "N2000_R10",
			totalAttrs:  2000,
			secretRatio: 0.10,
		},
		{
			name:        "N2000_R50",
			totalAttrs:  2000,
			secretRatio: 0.50,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			template := newMap(b, tc.totalAttrs, tc.secretRatio)
			loopAttrs := pcommon.NewMap()
			loopAttrs.EnsureCapacity(tc.totalAttrs)

			for b.Loop() {
				b.StopTimer()
				loopAttrs.Clear()
				template.CopyTo(loopAttrs)
				b.StartTimer()

				ap.Process(b.Context(), nil, loopAttrs)
			}
		})
	}
}
