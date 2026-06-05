// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
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
