// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadog"

import (
	"context"
	"testing"
	"time"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
	traceconfig "github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-agent/pkg/trace/testutil"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTraceAgentConfig(t *testing.T) {
	cfg := traceconfig.New()
	require.NotZero(t, cfg.ReceiverPort)

	out := make(chan *pb.StatsPayload)
	agnt := NewAgentWithConfig(context.Background(), cfg, out)
	require.Zero(t, cfg.ReceiverPort)
	require.NotEmpty(t, cfg.Endpoints[0].APIKey)
	require.Equal(t, metrics.UnsetHostnamePlaceholder, cfg.Hostname)
	require.Equal(t, out, agnt.Concentrator.Out)
}

func TestTraceAgent(t *testing.T) {
	cfg := traceconfig.New()
	cfg.BucketInterval = 50 * time.Millisecond
	out := make(chan *pb.StatsPayload, 10)
	ctx := context.Background()
	a := NewAgentWithConfig(ctx, cfg, out)
	a.Start()
	defer a.Stop()

	traces := testutil.NewOTLPTracesRequest([]testutil.OTLPResourceSpan{
		{
			LibName:    "libname",
			LibVersion: "1.2",
			Attributes: map[string]any{},
			Spans: []*testutil.OTLPSpan{
				{Name: "1"},
				{Name: "2"},
				{Name: "3"},
			},
		},
		{
			LibName:    "other-libname",
			LibVersion: "2.1",
			Attributes: map[string]any{},
			Spans: []*testutil.OTLPSpan{
				{Name: "4", TraceID: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
				{Name: "5", TraceID: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}},
			},
		},
	}).Traces()

	a.Ingest(ctx, traces)
	var stats *pb.StatsPayload
	timeout := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case stats = <-out:
			if len(stats.Stats) != 0 {
				break loop
			}
		case <-timeout:
			t.Fatal("timed out")
		}
	}
	require.Len(t, stats.Stats, 1)
	require.Len(t, stats.Stats[0].Stats, 1)
	// considering all spans in rspans have distinct aggregations, we should have an equal amount
	// of groups
	require.Len(t, stats.Stats[0].Stats[0].Stats, traces.SpanCount())
	require.Len(t, a.TraceWriter.In, 0) // the trace writer channel should've been drained

	// Check that the payload is labeled
	val, ok := traces.ResourceSpans().At(0).Resource().Attributes().Get(keyStatsComputed)
	require.True(t, ok)
	require.Equal(t, pcommon.ValueTypeBool, val.Type())
	require.True(t, val.Bool())

	// Ingest again
	a.Ingest(ctx, traces)
	timeout = time.After(500 * time.Millisecond)
loop2:
	for {
		select {
		case stats = <-out:
			if len(stats.Stats) != 0 {
				t.Fatal("got payload when none was expected")
			}
		case <-timeout:
			// We got no stats (expected), thus we end the test
			break loop2
		}
	}
}
