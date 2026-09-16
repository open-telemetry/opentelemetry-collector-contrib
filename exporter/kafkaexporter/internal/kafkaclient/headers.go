// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"slices"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// metadataToHeaders converts context metadata into a kgo.RecordHeader slice.
func metadataToHeaders(ctx context.Context, keys []string) []kgo.RecordHeader {
	if len(keys) == 0 {
		return nil
	}
	info := client.FromContext(ctx)
	var headers []kgo.RecordHeader
	for _, key := range keys {
		for _, v := range info.Metadata.Get(key) {
			if headers == nil {
				headers = make([]kgo.RecordHeader, 0, len(keys))
			}
			headers = append(headers, kgo.RecordHeader{Key: key, Value: []byte(v)})
		}
	}
	return headers
}

// traceContextToHeaders converts the sampled span context in ctx into W3C
// Trace Context headers. It returns nil if the span is not sampled.
func traceContextToHeaders(ctx context.Context) []kgo.RecordHeader {
	if !trace.SpanContextFromContext(ctx).IsSampled() {
		return nil
	}
	var record kgo.Record
	propagation.TraceContext{}.Inject(ctx, kotel.NewRecordCarrier(&record))
	return record.Headers
}

// isTraceContextHeader reports whether key is a W3C Trace Context header.
func isTraceContextHeader(key string) bool {
	return slices.Contains(propagation.TraceContext{}.Fields(), key)
}
