// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

func BenchmarkPeerTags_Native(b *testing.B) {
	benchmarkPeerTags(b)
}

func BenchmarkPeerTags_Legacy(b *testing.B) {
	err := featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), false)
	assert.NoError(b, err)
	defer func() {
		_ = featuregate.GlobalRegistry().Set(NativeIngestFeatureGate.ID(), true)
	}()

	benchmarkPeerTags(b)
}

func benchmarkPeerTags(b *testing.B) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Traces.ComputeStatsBySpanKind = true
	cfg.Traces.PeerTagsAggregation = true
	cfg.Traces.BucketInterval = 1 * time.Millisecond
	cfg.Traces.TraceBuffer = 0

	factory := NewFactory()
	creationParams := connectortest.NewNopSettings()
	metricsSink := &consumertest.MetricsSink{}

	tconn, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, metricsSink)
	assert.NoError(b, err)

	err = tconn.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		b.Errorf("Error starting connector: %v", err)
		return
	}
	defer func() {
		require.NoError(b, tconn.Shutdown(context.Background()))
	}()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = tconn.ConsumeTraces(context.Background(), genTrace())
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
