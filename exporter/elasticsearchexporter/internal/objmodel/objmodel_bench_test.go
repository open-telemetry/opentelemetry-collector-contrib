// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package objmodel

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Reproduces Document.Serialize/Dedup cost seen in ECS metrics encoding
// (~17% Serialize, ~12% Dedup of production CPU profile).
func BenchmarkDocumentSerializeECS(b *testing.B) {
	cases := []int{20, 40, 80}
	for _, n := range cases {
		b.Run(fmt.Sprintf("fields_%d", n), func(b *testing.B) {
			doc := newBenchDocument(n)
			var buf bytes.Buffer
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				buf.Reset()
				// Serialize calls Dedup internally (same as ecsDataPointsEncoder.encodeMetrics).
				if err := doc.Serialize(&buf, true, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func newBenchDocument(attrCount int) Document {
	attrs := pcommon.NewMap()
	attrs.PutStr("service.name", "bench-service")
	attrs.PutStr("host.name", "bench-host")
	attrs.PutStr("cloud.provider", "azure")
	attrs.PutStr("cloud.region", "eastus")
	attrs.PutStr("data_stream.type", "metrics")
	attrs.PutStr("data_stream.dataset", "generic")
	attrs.PutStr("data_stream.namespace", "default")
	for i := 0; i < attrCount; i++ {
		attrs.PutStr(fmt.Sprintf("label.%02d", i), fmt.Sprintf("v-%d", i))
	}
	doc := DocumentFromAttributes(attrs)
	doc.AddTimestamp("@timestamp", pcommon.NewTimestampFromTime(time.Unix(1_700_000_000, 0)))
	doc.Add("http.server.request.duration", DoubleValue(1.23))
	return doc
}
