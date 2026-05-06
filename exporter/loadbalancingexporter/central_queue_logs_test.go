// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestCentralQueueLogsItemDoesNotRetainUncompressedPayload(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	logs := compressibleLogs(16000, 2048)
	item, err := newCentralQueueLogsItem([]byte("route-a"), logs, codec, time.Now())
	require.NoError(t, err)
	require.Equal(t, signalKindLogs, item.signal)
	require.Equal(t, logs.LogRecordCount(), item.count)
	require.Greater(t, item.uncompressedBytes, item.compressedBytes*100)
	require.Len(t, item.payload, item.compressedBytes)
	require.Equal(t, len(item.payload), cap(item.payload))

	decoded, err := decodeCentralQueueLogsItem(item, codec)
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(logs, decoded))
}
