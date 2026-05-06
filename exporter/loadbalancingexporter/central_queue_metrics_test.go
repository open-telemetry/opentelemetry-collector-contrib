// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestCentralQueueMetricsItemDoesNotRetainUncompressedPayload(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	md := compressibleMetrics(4096)
	item, err := newCentralQueueMetricsItem([]byte("route-a"), md, codec, time.Now())
	require.NoError(t, err)
	require.Equal(t, signalKindMetrics, item.signal)
	require.Equal(t, md.DataPointCount(), item.count)
	require.Greater(t, item.uncompressedBytes, item.compressedBytes)
	require.Len(t, item.payload, item.compressedBytes)
	require.Equal(t, len(item.payload), cap(item.payload))

	decoded, err := decodeCentralQueueMetricsItem(item, codec)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(md, decoded))
}

func compressibleMetrics(points int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "central-queue-test")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("central-queue-test")
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("central.queue.test")
	metric.SetEmptyGauge()
	for range points {
		dp := metric.Gauge().DataPoints().AppendEmpty()
		dp.SetIntValue(42)
		dp.Attributes().PutStr("host.name", "same-host")
		dp.Attributes().PutStr("deployment.environment", "test")
	}
	return md
}
