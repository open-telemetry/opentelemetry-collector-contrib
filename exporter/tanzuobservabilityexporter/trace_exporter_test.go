// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

func TestSpansRequireTraceAndSpanIDs(t *testing.T) {
	spanWithNoTraceID := ptrace.NewSpan()
	spanWithNoTraceID.SetSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
	spanWithNoSpanID := ptrace.NewSpan()
	spanWithNoSpanID.SetTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	traces := constructTraces([]ptrace.Span{spanWithNoTraceID, spanWithNoSpanID})

	_, err := consumeTraces(traces)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), errInvalidSpanID.Error()))
	assert.True(t, strings.Contains(err.Error(), errInvalidTraceID.Error()))
}

func TestExportTraceDataMinimum(t *testing.T) {
	// <operationName> source=<source> <spanTags> <start_milliseconds> <duration_milliseconds>
	// getAllUsers source=localhost traceId=7b3bf470-9456-11e8-9eb6-529269fb1459 spanId=0313bafe-9457-11e8-9eb6-529269fb1459 parent=2f64e538-9457-11e8-9eb6-529269fb1459 application=Wavefront service=auth cluster=us-west-2 shard=secondary http.method=GET 1552949776000 343
	minSpan := createSpan(
		"root",
		[16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		[8]byte{9, 9, 9, 9, 9, 9, 9, 9},
		pcommon.SpanID{},
	)
	traces := constructTraces([]ptrace.Span{minSpan})

	expected := []*span{{
		Name:    "root",
		TraceID: uuid.MustParse("01010101-0101-0101-0101-010101010101"),
		SpanID:  uuid.MustParse("00000000-0000-0000-0909-090909090909"),
		Tags: map[string]string{
			labelApplication: "defaultApp",
			labelService:     "defaultService",
		},
	}}

	validateTraces(t, expected, traces)
}

func TestExportTraceDataFullTrace(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})

	rootSpan := createSpan(
		"root",
		traceID,
		[8]byte{0, 0, 0, 0, 0, 0, 0, 1},
		pcommon.SpanID{},
	)

	clientSpan := createSpan(
		"client",
		traceID,
		[8]byte{0, 0, 0, 0, 0, 0, 0, 2},
		rootSpan.SpanID(),
	)

	clientSpan.SetKind(ptrace.SpanKindClient)
	event := ptrace.NewSpanEvent()
	event.SetName("client-event")
	event.CopyTo(clientSpan.Events().AppendEmpty())

	status := ptrace.NewStatus()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("an error event occurred")
	status.CopyTo(clientSpan.Status())

	clientSpan.Attributes().PutStr(labelApplication, "test-app")

	serverSpan := createSpan(
		"server",
		traceID,
		pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 3}),
		clientSpan.SpanID(),
	)
	serverSpan.SetKind(ptrace.SpanKindServer)
	serverSpan.TraceState().FromRaw("key=val")
	serverAttrs := serverSpan.Attributes()
	serverAttrs.PutStr(conventions.AttributeServiceName, "the-server")
	serverAttrs.PutStr(conventions.AttributeHTTPMethod, "POST")
	serverAttrs.PutInt(conventions.AttributeHTTPStatusCode, 403)
	serverAttrs.PutStr(labelSource, "test_source")

	traces := constructTraces([]ptrace.Span{rootSpan, clientSpan, serverSpan})
	resourceAttrs := traces.ResourceSpans().At(0).Resource().Attributes()
	resourceAttrs.PutStr("resource", "R1")
	resourceAttrs.PutStr(conventions.AttributeServiceName, "test-service")
	resourceAttrs.PutStr(labelSource, "test-source")

	expected := []*span{
		{
			Name:    "root",
			SpanID:  uuid.MustParse("00000000000000000000000000000001"),
			TraceID: uuid.MustParse("01010101010101010101010101010101"),
			Source:  "test-source",
			Tags: map[string]string{
				"resource":       "R1",
				labelApplication: "defaultApp",
				labelService:     "test-service",
			},
		},
		{
			Name:         "client",
			SpanID:       uuid.MustParse("00000000000000000000000000000002"),
			TraceID:      uuid.MustParse("01010101010101010101010101010101"),
			ParentSpanID: uuid.MustParse("00000000000000000000000000000001"),
			Source:       "test-source",
			Tags: map[string]string{
				"resource":                "R1",
				labelApplication:          "test-app",
				labelService:              "test-service",
				"otel.status_description": "an error event occurred",
				"error":                   "true",
				labelSpanKind:             "client",
			},
			SpanLogs: []senders.SpanLog{{
				Fields: map[string]string{labelEventName: "client-event"},
			}},
		},
		{
			Name:         "server",
			SpanID:       uuid.MustParse("00000000000000000000000000000003"),
			TraceID:      uuid.MustParse("01010101010101010101010101010101"),
			ParentSpanID: uuid.MustParse("00000000000000000000000000000002"),
			Source:       "test-source",
			Tags: map[string]string{
				"resource":                          "R1",
				labelApplication:                    "defaultApp",
				labelService:                        "the-server",
				labelSpanKind:                       "server",
				conventions.AttributeHTTPStatusCode: "403",
				conventions.AttributeHTTPMethod:     "POST",
				"w3c.tracestate":                    "key=val",
			},
		},
	}

	validateTraces(t, expected, traces)
}

func validateTraces(t *testing.T, expected []*span, traces ptrace.Traces) {
	actual, err := consumeTraces(traces)
	require.NoError(t, err)
	require.Equal(t, len(expected), len(actual))
	for i := 0; i < len(expected); i++ {
		assert.Equal(t, expected[i].Name, actual[i].Name)
		assert.Equal(t, expected[i].TraceID, actual[i].TraceID)
		assert.Equal(t, expected[i].SpanID, actual[i].SpanID)
		assert.Equal(t, expected[i].ParentSpanID, actual[i].ParentSpanID)
		for k, v := range expected[i].Tags {
			a, ok := actual[i].Tags[k]
			assert.True(t, ok, "tag '"+k+"' not found")
			assert.Equal(t, v, a)
		}
		assert.Equal(t, expected[i].StartMillis, actual[i].StartMillis)
		assert.Equal(t, expected[i].DurationMillis, actual[i].DurationMillis)
		assert.Equal(t, expected[i].SpanLogs, actual[i].SpanLogs)
		assert.Equal(t, expected[i].Source, actual[i].Source)
	}
}

func TestExportTraceDataWithInstrumentationDetails(t *testing.T) {
	minSpan := createSpan(
		"root",
		pcommon.TraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
		pcommon.SpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9}),
		pcommon.SpanID{},
	)
	traces := constructTraces([]ptrace.Span{minSpan})

	scope := traces.ResourceSpans().At(0).ScopeSpans().At(0).Scope()
	scope.SetName("instrumentation_name")
	scope.SetVersion("v0.0.1")

	expected := []*span{{
		Name:    "root",
		TraceID: uuid.MustParse("01010101-0101-0101-0101-010101010101"),
		SpanID:  uuid.MustParse("00000000-0000-0000-0909-090909090909"),
		Tags: map[string]string{
			labelApplication:      "defaultApp",
			labelService:          "defaultService",
			labelOtelScopeName:    "instrumentation_name",
			labelOtelScopeVersion: "v0.0.1",
		},
	}}

	validateTraces(t, expected, traces)
}

func TestExportTraceDataRespectsContext(t *testing.T) {
	traces := constructTraces([]ptrace.Span{createSpan(
		"root",
		pcommon.TraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
		pcommon.SpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9}),
		pcommon.SpanID{},
	)})

	sender := &mockSender{}
	cfg := createDefaultConfig()
	exp := tracesExporter{
		cfg:    cfg.(*Config),
		sender: sender,
		logger: zap.NewNop(),
	}
	mockOTelTracesExporter, err := exporterhelper.NewTracesExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg,
		exp.pushTraceData,
		exporterhelper.WithShutdown(exp.shutdown),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.Error(t, mockOTelTracesExporter.ConsumeTraces(ctx, traces))
}

func createSpan(
	name string,
	traceID pcommon.TraceID,
	spanID pcommon.SpanID,
	parentSpanID pcommon.SpanID,
) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetName(name)
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	return span
}

func constructTraces(spans []ptrace.Span) ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)
	rs := traces.ResourceSpans().AppendEmpty()
	rs.ScopeSpans().EnsureCapacity(1)
	ils := rs.ScopeSpans().AppendEmpty()
	ils.Spans().EnsureCapacity(len(spans))
	for _, span := range spans {
		span.CopyTo(ils.Spans().AppendEmpty())
	}
	return traces
}

func consumeTraces(ptrace ptrace.Traces) ([]*span, error) {
	ctx := context.Background()
	sender := &mockSender{}

	cfg := createDefaultConfig()
	exp := tracesExporter{
		cfg:    cfg.(*Config),
		sender: sender,
		logger: zap.NewNop(),
	}
	mockOTelTracesExporter, err := exporterhelper.NewTracesExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		cfg,
		exp.pushTraceData,
		exporterhelper.WithShutdown(exp.shutdown),
	)

	if err != nil {
		return nil, err
	}
	if err := mockOTelTracesExporter.ConsumeTraces(ctx, ptrace); err != nil {
		return nil, err
	}
	if err := mockOTelTracesExporter.Shutdown(ctx); err != nil {
		return nil, err
	}
	return sender.spans, nil
}

// implements the spanSender interface
type mockSender struct {
	spans []*span
}

func (m *mockSender) SendSpan(
	name string,
	startMillis, durationMillis int64,
	source, traceID, spanID string,
	parents, _ []string,
	spanTags []senders.SpanTag,
	spanLogs []senders.SpanLog,
) error {
	var parentSpanID uuid.UUID
	if len(parents) == 1 {
		parentSpanID = uuid.MustParse(parents[0])
	}
	tags := map[string]string{}
	for _, pair := range spanTags {
		tags[pair.Key] = pair.Value
	}
	span := &span{
		Name:           name,
		TraceID:        uuid.MustParse(traceID),
		SpanID:         uuid.MustParse(spanID),
		ParentSpanID:   parentSpanID,
		Tags:           tags,
		StartMillis:    startMillis,
		DurationMillis: durationMillis,
		SpanLogs:       spanLogs,
		Source:         source,
	}
	m.spans = append(m.spans, span)
	return nil
}
func (m *mockSender) Flush() error { return nil }
func (m *mockSender) Close()       {}
