// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

import (
	"testing"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

var defaultConfig = createDefaultConfig().(*Config)

// Tests the export onTraceData callback with no Spans
func TestExporterTraceDataCallbackNoSpans(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

	traces := ptrace.NewTraces()

	assert.NoError(t, exporter.consumeTraces(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 0)
	mockTransportChannel.AssertNumberOfCalls(t, "Flush", 0)
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

	assert.NoError(t, exporter.consumeTraces(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 1)
	mockTransportChannel.AssertNumberOfCalls(t, "Flush", 1)
}

// Tests the export onTraceData callback calls exporter flush only once for 8 spans
func TestExporterTraceDataCallbackCallFlushOnce(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getExporter(defaultConfig, mockTransportChannel)

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
	span.CopyTo(ilss.Spans().AppendEmpty())
	ilss.CopyTo(rs.ScopeSpans().AppendEmpty())
	rs.CopyTo(traces.ResourceSpans().AppendEmpty())

	assert.NoError(t, exporter.consumeTraces(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 8)
	mockTransportChannel.AssertNumberOfCalls(t, "Flush", 1)
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

	assert.NoError(t, exporter.consumeTraces(context.Background(), traces))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 3)
	mockTransportChannel.AssertNumberOfCalls(t, "Flush", 1)
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
	span.Attributes().PutStr(string(conventions.FaaSTriggerKey), "http")

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	r := rs.Resource()
	resource.CopyTo(r)
	ilss := rs.ScopeSpans().AppendEmpty()
	scope.CopyTo(ilss.Scope())
	span.CopyTo(ilss.Spans().AppendEmpty())

	err := exporter.consumeTraces(context.Background(), traces)
	assert.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err), "error should be permanent")

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 0)
	mockTransportChannel.AssertNumberOfCalls(t, "Flush", 0)
}

func getMockTransportChannel() *mockTransportChannel {
	transportChannelMock := mockTransportChannel{}
	transportChannelMock.On("Send", mock.Anything)
	transportChannelMock.On("Flush", mock.Anything)
	return &transportChannelMock
}

func getExporter(config *Config, transportChannel appinsights.TelemetryChannel) *azureMonitorExporter {
	return &azureMonitorExporter{
		config,
		transportChannel,
		zap.NewNop(),
		newMetricPacker(zap.NewNop()),
	}
}
