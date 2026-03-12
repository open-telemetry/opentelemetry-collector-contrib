// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"fmt"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func BenchmarkContextWithHeaders(b *testing.B) {
	baseCtx := b.Context()
	tests := []struct {
		name    string
		headers []kgo.RecordHeader
	}{
		{
			name:    "no headers",
			headers: nil,
		},
		{
			name: "1 header",
			headers: []kgo.RecordHeader{
				{Key: "trace-id", Value: []byte("abc123")},
			},
		},
		{
			name: "5 headers",
			headers: []kgo.RecordHeader{
				{Key: "trace-id", Value: []byte("abc123")},
				{Key: "span-id", Value: []byte("def456")},
				{Key: "tenant", Value: []byte("acme")},
				{Key: "source", Value: []byte("app1")},
				{Key: "env", Value: []byte("prod")},
			},
		},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				_ = contextWithHeaders(baseCtx, tt.headers)
			}
		})
	}
}

func BenchmarkGetMessageHeaderResourceAttributes(b *testing.B) {
	for _, numHeaders := range []int{1, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("headers=%d", numHeaders), func(b *testing.B) {
			// Build message headers: numHeaders matching + 1 unrelated.
			kgoHeaders := make([]kgo.RecordHeader, 0, numHeaders+1)
			headerAttrKeys := make(map[string]string, numHeaders)
			for i := range numHeaders {
				key := fmt.Sprintf("header-%d", i)
				kgoHeaders = append(kgoHeaders, kgo.RecordHeader{
					Key:   key,
					Value: fmt.Appendf(nil, "value-%d", i),
				})
				headerAttrKeys[key] = "kafka.header." + key
			}
			kgoHeaders = append(kgoHeaders, kgo.RecordHeader{
				Key:   "unrelated",
				Value: []byte("ignored"),
			})
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for k, v := range getMessageHeaderResourceAttributes(kgoHeaders, headerAttrKeys) {
					_ = k
					_ = v
				}
			}
		})
	}
}
