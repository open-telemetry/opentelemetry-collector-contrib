// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestRollingSpanLatencyProcessor_PassesTracesThroughUnchanged(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	set := processortest.NewNopSettings(component.MustNewType("rolling_span_latency"))
	sink := new(consumertest.TracesSink)

	tp, err := newRollingSpanLatencyProcessor(t.Context(), cfg, set, sink)
	require.NoError(t, err)
	require.NotNil(t, tp)

	require.NoError(t, tp.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { assert.NoError(t, tp.Shutdown(t.Context())) }()

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	require.NoError(t, tp.ConsumeTraces(t.Context(), td))

	require.Len(t, sink.AllTraces(), 1)
	assert.Equal(t, td, sink.AllTraces()[0])
}
