// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func BenchmarkContextWithHeaders(b *testing.B) {
	baseCtx := b.Context()
	tests := []struct {
		name    string
		headers messageHeaders
	}{
		{
			name:    "no headers",
			headers: franzHeaders{headers: nil},
		},
		{
			name: "1 header",
			headers: franzHeaders{headers: []kgo.RecordHeader{
				{Key: "trace-id", Value: []byte("abc123")},
			}},
		},
		{
			name: "5 headers",
			headers: franzHeaders{headers: []kgo.RecordHeader{
				{Key: "trace-id", Value: []byte("abc123")},
				{Key: "span-id", Value: []byte("def456")},
				{Key: "tenant", Value: []byte("acme")},
				{Key: "source", Value: []byte("app1")},
				{Key: "env", Value: []byte("prod")},
			}},
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
