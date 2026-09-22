// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"
	"slices"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/otel/propagation"
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

// traceContextToHeaders converts the trace context in ctx into a
// kgo.RecordHeader slice using propagator.
func traceContextToHeaders(ctx context.Context, propagator propagation.TextMapPropagator) []kgo.RecordHeader {
	var headers headerCarrier
	propagator.Inject(ctx, &headers)
	return headers
}

// headerCarrier adapts a kgo.RecordHeader slice to propagation.TextMapCarrier.
type headerCarrier []kgo.RecordHeader

var _ propagation.TextMapCarrier = (*headerCarrier)(nil)

func (c *headerCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *headerCarrier) Set(key, value string) {
	for i, h := range *c {
		if h.Key == key {
			(*c)[i].Value = []byte(value)
			return
		}
	}
	*c = append(*c, kgo.RecordHeader{Key: key, Value: []byte(value)})
}

func (c *headerCarrier) Keys() []string {
	keys := make([]string, len(*c))
	for i, h := range *c {
		keys[i] = h.Key
	}
	return keys
}

// appendHeadersExcept appends the headers whose keys are not in excludeKeys to dst.
func appendHeadersExcept(dst, headers []kgo.RecordHeader, excludeKeys []string) []kgo.RecordHeader {
	for _, h := range headers {
		if !slices.Contains(excludeKeys, h.Key) {
			dst = append(dst, h)
		}
	}
	return dst
}
