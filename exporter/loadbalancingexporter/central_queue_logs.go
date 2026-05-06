// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
)

func newCentralQueueLogsItem(routingKey []byte, logs plog.Logs, codec *queuePayloadCodec, now time.Time) (centralQueueItem, error) {
	marshaler := &plog.ProtoMarshaler{}
	payload, err := marshaler.MarshalLogs(logs)
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
		signal:             signalKindLogs,
		routingKey:         append([]byte(nil), routingKey...),
		payload:            encoded,
		compressedBytes:    len(encoded),
		uncompressedBytes:  len(payload),
		count:              logs.LogRecordCount(),
		enqueuedAtUnixNano: now.UnixNano(),
	}, nil
}

func decodeCentralQueueLogsItem(item centralQueueItem, codec *queuePayloadCodec) (plog.Logs, error) {
	decoded, err := codec.Decode(item.payload)
	if err != nil {
		return plog.Logs{}, err
	}
	return (&plog.ProtoUnmarshaler{}).UnmarshalLogs(decoded)
}
