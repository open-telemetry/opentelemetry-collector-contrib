// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/jsonwriter"
)

// newECSLogDocument builds a fat ECS-style log document: many dotted string
// fields, plus a timestamp and a few integers. n is the number of string fields.
func newECSLogDocument(n int) Document {
	var doc Document
	doc.Add("@timestamp", TimestampValue(dijkstra))
	doc.AddInt("event.severity", 9)
	fixed := [][2]string{
		{"trace.id", "aabbccddeeff00112233445566778899"},
		{"span.id", "aabbccddeeff0011"},
		{"log.level", "info"},
		{"message", "GET /api/checkout completed"},
		{"service.name", "checkout"},
		{"host.name", "host-abc"},
		{"cloud.provider", "aws"},
		{"cloud.region", "eu-central-1"},
	}
	for i := range n {
		if i < len(fixed) {
			doc.AddString(fixed[i][0], fixed[i][1])
			continue
		}
		doc.AddString(fmt.Sprintf("labels.k8s.io/label-%02d", i), fmt.Sprintf("value-%02d-xxxxxxxx", i))
	}
	return doc
}

// BenchmarkDocumentSerialize is the production ECS path: Dedup + JSON write
// with dedot. Dedup sorts in place, so each iteration copies fields back to
// unsorted order.
func BenchmarkDocumentSerialize(b *testing.B) {
	for _, n := range []int{20, 40, 80} {
		b.Run(fmt.Sprintf("fields_%d", n), func(b *testing.B) {
			base := newECSLogDocument(n)
			scratch := make([]field, len(base.fields))
			var buf bytes.Buffer
			b.ReportAllocs()
			for b.Loop() {
				doc := Document{fields: scratch[:copy(scratch, base.fields)]}
				buf.Reset()
				if err := doc.Serialize(&buf, true, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDocumentSerializeJSON isolates JSON writing after Dedup, so a
// CPU profile can show the encoder without the sort cost.
func BenchmarkDocumentSerializeJSON(b *testing.B) {
	for _, n := range []int{20, 40, 80} {
		b.Run(fmt.Sprintf("fields_%d", n), func(b *testing.B) {
			doc := newECSLogDocument(n)
			doc.Dedup(nil)
			var buf bytes.Buffer
			w := jsonwriter.New(&buf)
			b.ReportAllocs()
			for b.Loop() {
				buf.Reset()
				doc.writeJSON(w, true)
			}
		})
	}
}
