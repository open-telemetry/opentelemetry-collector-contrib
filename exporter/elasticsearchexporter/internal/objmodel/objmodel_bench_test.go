// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"fmt"
	"testing"
)

// BenchmarkDocumentSort measures the sorting hotspot used by Document.Serialize.
// Each document is cloned with the timer stopped because sorting mutates it.
func BenchmarkDocumentSort(b *testing.B) {
	cases := []struct {
		name       string
		fieldCount int
	}{
		{
			name:       "fields_20",
			fieldCount: 20,
		},
		{
			name:       "fields_40",
			fieldCount: 40,
		},
		{
			name:       "fields_80",
			fieldCount: 80,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			base := newSortBenchmarkDocument(tc.fieldCount)
			b.ReportAllocs()

			for b.Loop() {
				b.StopTimer()
				doc := base.Clone()
				b.StartTimer()
				doc.sort()
			}
		})
	}
}

func newSortBenchmarkDocument(fieldCount int) Document {
	var doc Document
	for i := range fieldCount {
		// Use a deterministic non-sorted insertion order, matching documents
		// assembled from resource and data point attributes.
		fieldID := (i * 37) % fieldCount
		doc.AddString(fmt.Sprintf("metric.%03d", fieldID), "value")
	}
	return doc
}
