// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkarequest

import (
	"context"
	"fmt"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func benchRecords(n, valueSize int) []*kgo.Record {
	out := make([]*kgo.Record, n)
	for i := range n {
		out[i] = &kgo.Record{
			Topic: "bench",
			Value: make([]byte, valueSize),
		}
	}
	return out
}

func BenchmarkMergeSplit_Bytes(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("records=%d", n), func(b *testing.B) {
			records := benchRecords(n, 256)
			req := New(records)
			ctx := context.Background()
			maxSize := 4096
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_, err := req.MergeSplit(ctx, maxSize, exporterhelper.RequestSizerTypeBytes, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMergeSplit_Items(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("records=%d", n), func(b *testing.B) {
			records := benchRecords(n, 64)
			req := New(records)
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_, err := req.MergeSplit(ctx, 100, exporterhelper.RequestSizerTypeItems, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBytesSize(b *testing.B) {
	req := New(benchRecords(1000, 256))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = req.BytesSize()
	}
}
