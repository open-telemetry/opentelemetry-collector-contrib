// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

func newCentralQueueMetricsItem(routingKey []byte, md pmetric.Metrics, codec *queuePayloadCodec, now time.Time) (centralQueueItem, error) {
	marshaler := &pmetric.ProtoMarshaler{}
	payload, err := marshaler.MarshalMetrics(md)
	if err != nil {
		return centralQueueItem{}, err
	}
	encoded, err := codec.Encode(payload)
	if err != nil {
		return centralQueueItem{}, err
	}
	encoded = append([]byte(nil), encoded...)
	encoded = encoded[:len(encoded):len(encoded)]
	return centralQueueItem{
		signal:             signalKindMetrics,
		routingKey:         append([]byte(nil), routingKey...),
		payload:            encoded,
		compressedBytes:    len(encoded),
		uncompressedBytes:  len(payload),
		count:              md.DataPointCount(),
		enqueuedAtUnixNano: now.UnixNano(),
	}, nil
}

func decodeCentralQueueMetricsItem(item centralQueueItem, codec *queuePayloadCodec) (pmetric.Metrics, error) {
	decoded, err := codec.Decode(item.payload)
	if err != nil {
		return pmetric.Metrics{}, err
	}
	return (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(decoded)
}
