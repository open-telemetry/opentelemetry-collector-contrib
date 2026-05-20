// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector/internal/metadata"
)

var (
	testTraceID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	testSpanID1 = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func genTrace() ptrace.Traces {
	start := time.Now().Add(-1 * time.Second)
	end := time.Now()
	traces := ptrace.NewTraces()
	rspan := traces.ResourceSpans().AppendEmpty()
	rattrs := rspan.Resource().Attributes()
	rattrs.PutStr("deployment.environment", "test_env")
	rattrs.PutStr("service.name", "test_svc")
	sspan := rspan.ScopeSpans().AppendEmpty()
	span := sspan.Spans().AppendEmpty()
	span.SetTraceID(testTraceID)
	span.SetSpanID(testSpanID1)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(end))
	span.SetName("span_name")
	span.SetKind(ptrace.SpanKindClient)
	span.Attributes().PutStr("peer.service", "my_peer_svc")
	span.Attributes().PutStr("rpc.service", "my_rpc_svc")
	span.Attributes().PutStr("net.peer.name", "my_net_peer")
	return traces
}

func BenchmarkPeerTags(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Traces.ComputeStatsBySpanKind = true
	cfg.Traces.PeerTagsAggregation = true
	cfg.Traces.BucketInterval = 1 * time.Millisecond
	cfg.Traces.TraceBuffer = 0

	factory := NewFactory()
	creationParams := connectortest.NewNopSettings(metadata.Type)
	metricsSink := &consumertest.MetricsSink{}

	tconn, err := factory.CreateTracesToMetrics(b.Context(), creationParams, cfg, metricsSink)
	assert.NoError(b, err)

	err = tconn.Start(b.Context(), componenttest.NewNopHost())
	if err != nil {
		b.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		require.NoError(b, tconn.Shutdown(b.Context()))
	}()

	for b.Loop() {
		err = tconn.ConsumeTraces(b.Context(), genTrace())
		assert.NoError(b, err)
		for {
			metrics := metricsSink.AllMetrics()
			if len(metrics) > 0 {
				assert.Len(b, metrics, 1)
				break
			}
		}
		metricsSink.Reset()
	}
}
