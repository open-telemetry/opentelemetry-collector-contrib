// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genaiadapterconnector

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)
}

func TestGetOrCreateConnector_Singleton(t *testing.T) {
	connectors := &sync.Map{}
	set := connectortest.NewNopSettings(metadata.Type)
	c1 := getOrCreateConnector(connectors, set)
	c2 := getOrCreateConnector(connectors, set)
	assert.Same(t, c1, c2)
}

func TestGetOrCreateConnector_DifferentIDs(t *testing.T) {
	connectors := &sync.Map{}
	set1 := connectortest.NewNopSettings(metadata.Type)
	set2 := connectortest.NewNopSettings(metadata.Type)
	set2.ID = component.MustNewIDWithName("genai_adapter", "other")

	c1 := getOrCreateConnector(connectors, set1)
	c2 := getOrCreateConnector(connectors, set2)
	assert.NotSame(t, c1, c2)
}

func TestCreateTracesToTraces(t *testing.T) {
	factory := NewFactory()
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig()

	conn, err := factory.CreateTracesToTraces(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, conn)

	_, ok := conn.(*genAIAdapterConnector)
	assert.True(t, ok)
}

func TestCreateTracesToLogs(t *testing.T) {
	factory := NewFactory()
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := factory.CreateDefaultConfig()

	conn, err := factory.CreateTracesToLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, conn)

	_, ok := conn.(*logsOnlyConnector)
	assert.True(t, ok)
}

func TestStartShutdown(t *testing.T) {
	connectors := &sync.Map{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)

	assert.NoError(t, c.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, c.Shutdown(t.Context()))
}

func TestConsumeTraces_OISpanTransformed(t *testing.T) {
	connectors := &sync.Map{}
	tracesSink := &consumertest.TracesSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.tracesConsumer = tracesSink

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("openinference.span.kind", "LLM")
	span.Attributes().PutStr("llm.model_name", "gpt-4")

	err := c.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	require.Len(t, tracesSink.AllTraces(), 1)
	outSpan := tracesSink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	_, hasGenAIModel := outSpan.Attributes().Get("gen_ai.request.model")
	assert.True(t, hasGenAIModel)
	_, hasOIKind := outSpan.Attributes().Get("openinference.span.kind")
	assert.False(t, hasOIKind)
}

func TestConsumeTraces_NonOISpanPassthrough(t *testing.T) {
	connectors := &sync.Map{}
	tracesSink := &consumertest.TracesSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.tracesConsumer = tracesSink

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "POST")
	span.Attributes().PutStr("http.url", "https://example.com")

	err := c.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	require.Len(t, tracesSink.AllTraces(), 1)
	outSpan := tracesSink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)

	method, _ := outSpan.Attributes().Get("http.method")
	assert.Equal(t, "POST", method.AsString())
	url, _ := outSpan.Attributes().Get("http.url")
	assert.Equal(t, "https://example.com", url.AsString())
}

func TestConsumeTraces_LLOEmitsLogs(t *testing.T) {
	connectors := &sync.Map{}
	logsSink := &consumertest.LogsSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.logsConsumer = logsSink

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test.scope")
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("input.value", "user prompt")
	span.Attributes().PutStr("output.value", "assistant response")

	err := c.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	require.Len(t, logsSink.AllLogs(), 1)
	assert.Positive(t, logsSink.AllLogs()[0].LogRecordCount())

	_, hasInput := span.Attributes().Get("input.value")
	assert.False(t, hasInput)
	_, hasOutput := span.Attributes().Get("output.value")
	assert.False(t, hasOutput)
}

func TestConsumeTraces_ForwardsToTracesConsumer(t *testing.T) {
	connectors := &sync.Map{}
	tracesSink := &consumertest.TracesSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.tracesConsumer = tracesSink

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := c.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)
	assert.Len(t, tracesSink.AllTraces(), 1)
}

func TestConsumeTraces_ForwardsToLogsConsumer(t *testing.T) {
	connectors := &sync.Map{}
	logsSink := &consumertest.LogsSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.logsConsumer = logsSink

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test.scope")
	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("gen_ai.prompt", "hello")

	err := c.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)
	assert.Len(t, logsSink.AllLogs(), 1)
}

func TestConsumeTraces_NoLogsWhenNoLLO(t *testing.T) {
	connectors := &sync.Map{}
	logsSink := &consumertest.LogsSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.logsConsumer = logsSink

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("http.method", "GET")

	err := c.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)
	assert.Empty(t, logsSink.AllLogs())
}

func TestLogsOnlyConnector_ConsumeTracesNoOp(t *testing.T) {
	connectors := &sync.Map{}
	tracesSink := &consumertest.TracesSink{}
	set := connectortest.NewNopSettings(metadata.Type)
	c := getOrCreateConnector(connectors, set)
	c.tracesConsumer = tracesSink

	wrapper := &logsOnlyConnector{c}

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := wrapper.ConsumeTraces(t.Context(), td)
	assert.NoError(t, err)
	assert.Empty(t, tracesSink.AllTraces())
}
