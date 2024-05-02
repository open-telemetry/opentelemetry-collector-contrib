// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var (
	defaultConfig = createDefaultConfig().(*Config)
)

// Tests the export onTraceData callback with no Spans
func TestExporterTraceDataCallbackNoSpans(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	traces := ptrace.NewTraces()

	assert.NoError(t, exporter.onTraceData(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 0)
}

// Tests the export onTraceData callback with a single Span
func TestExporterTraceDataCallbackSingleSpan(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	// re-use some test generation method(s) from trace_to_envelope_test
	resource := getResource()
	scope := getScope()
	span := getDefaultHTTPServerSpan()

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	r := rs.Resource()
	resource.CopyTo(r)
	ilss := rs.ScopeSpans().AppendEmpty()
	scope.CopyTo(ilss.Scope())
	span.CopyTo(ilss.Spans().AppendEmpty())

	assert.NoError(t, exporter.onTraceData(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 1)
}

// Tests the export onTraceData callback with a single Span with SpanEvents
func TestExporterTraceDataCallbackSingleSpanWithSpanEvents(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	config := createDefaultConfig().(*Config)
	config.SpanEventsEnabled = true
	exporter := getExporter(config, mockTransportChannel)

	// re-use some test generation method(s) from trace_to_envelope_test
	resource := getResource()
	scope := getScope()
	span := getDefaultHTTPServerSpan()

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	r := rs.Resource()
	resource.CopyTo(r)
	ilss := rs.ScopeSpans().AppendEmpty()
	scope.CopyTo(ilss.Scope())

	spanEvent1 := getSpanEvent("foo", map[string]any{"foo": "bar"})
	spanEvent1.CopyTo(span.Events().AppendEmpty())

	spanEvent2 := getSpanEvent("bar", map[string]any{"bar": "baz"})
	spanEvent2.CopyTo(span.Events().AppendEmpty())

	span.CopyTo(ilss.Spans().AppendEmpty())

	assert.NoError(t, exporter.onTraceData(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 3)
}

// Tests the export onTraceData callback with a single Span that fails to produce an envelope
func TestExporterTraceDataCallbackSingleSpanNoEnvelope(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	// re-use some test generation method(s) from trace_to_envelope_test
	resource := getResource()
	scope := getScope()
	span := getDefaultInternalSpan()

	// Make this a FaaS span, which will trigger an error, because conversion
	// of them is currently not supported.
	span.Attributes().PutStr(conventions.AttributeFaaSTrigger, "http")

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	r := rs.Resource()
	resource.CopyTo(r)
	ilss := rs.ScopeSpans().AppendEmpty()
	scope.CopyTo(ilss.Scope())
	span.CopyTo(ilss.Spans().AppendEmpty())

	err := exporter.onTraceData(context.Background(), traces)
	assert.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err), "error should be permanent")

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 0)
}

func getMockTransportChannel() *mockTransportChannel {
	transportChannelMock := mockTransportChannel{}
	transportChannelMock.On("Send", mock.Anything)
	return &transportChannelMock
}

func getExporter(config *Config, transportChannel transportChannel) *traceExporter {
	return &traceExporter{
		config,
		transportChannel,
		zap.NewNop(),
	}
}
