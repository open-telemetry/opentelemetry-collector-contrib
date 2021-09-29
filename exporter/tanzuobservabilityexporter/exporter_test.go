// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tanzuobservabilityexporter

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
)

func TestSpansRequireTraceAndSpanIDs(t *testing.T) {
	spanWithNoTraceID := pdata.NewSpan()
	spanWithNoTraceID.SetSpanID(pdata.NewSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9}))
	spanWithNoSpanID := pdata.NewSpan()
	spanWithNoSpanID.SetTraceID(pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}))
	traces := constructTraces([]pdata.Span{spanWithNoTraceID, spanWithNoSpanID})

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
		pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
		pdata.NewSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9}),
		pdata.SpanID{},
	)
	traces := constructTraces([]pdata.Span{minSpan})

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
	traceID := pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})

	rootSpan := createSpan(
		"root",
		traceID,
		pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}),
		pdata.SpanID{},
	)

	clientSpan := createSpan(
		"client",
		traceID,
		pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 2}),
		rootSpan.SpanID(),
	)
	clientSpan.SetKind(pdata.SpanKindClient)
	event := pdata.NewSpanEvent()
	event.SetName("client-event")
	event.CopyTo(clientSpan.Events().AppendEmpty())

	status := pdata.NewSpanStatus()
	status.SetCode(pdata.StatusCodeError)
	status.SetMessage("an error event occurred")
	status.CopyTo(clientSpan.Status())

	clientAttrs := pdata.NewAttributeMap()
	clientAttrs.InsertString(labelApplication, "test-app")
	clientAttrs.CopyTo(clientSpan.Attributes())

	serverSpan := createSpan(
		"server",
		traceID,
		pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 3}),
		clientSpan.SpanID(),
	)
	serverSpan.SetKind(pdata.SpanKindServer)
	serverSpan.SetTraceState("key=val")
	serverAttrs := pdata.NewAttributeMap()
	serverAttrs.InsertString(conventions.AttributeServiceName, "the-server")
	serverAttrs.InsertString(conventions.AttributeHTTPMethod, "POST")
	serverAttrs.InsertInt(conventions.AttributeHTTPStatusCode, 403)
	serverAttrs.CopyTo(serverSpan.Attributes())

	traces := constructTraces([]pdata.Span{rootSpan, clientSpan, serverSpan})
	resourceAttrs := pdata.NewAttributeMap()
	resourceAttrs.InsertString("resource", "R1")
	resourceAttrs.InsertString(conventions.AttributeServiceName, "test-service")
	resourceAttrs.CopyTo(traces.ResourceSpans().At(0).Resource().Attributes())

	expected := []*span{
		{
			Name:    "root",
			SpanID:  uuid.MustParse("00000000000000000000000000000001"),
			TraceID: uuid.MustParse("01010101010101010101010101010101"),
			Tags: map[string]string{
				"resource":       "R1",
				labelApplication: "defaultApp",
				labelService:     "test-service",
				labelStatusCode:  "0",
			},
		},
		{
			Name:         "client",
			SpanID:       uuid.MustParse("00000000000000000000000000000002"),
			TraceID:      uuid.MustParse("01010101010101010101010101010101"),
			ParentSpanID: uuid.MustParse("00000000000000000000000000000001"),
			Tags: map[string]string{
				"resource":         "R1",
				labelApplication:   "test-app",
				labelService:       "test-service",
				labelStatusCode:    "2",
				labelStatusMessage: "an error event occurred",
				"error":            "true",
				labelSpanKind:      "client",
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
			Tags: map[string]string{
				"resource":                          "R1",
				labelApplication:                    "defaultApp",
				labelService:                        "the-server",
				labelStatusCode:                     "0",
				labelSpanKind:                       "server",
				conventions.AttributeHTTPStatusCode: "403",
				conventions.AttributeHTTPMethod:     "POST",
				"w3c.tracestate":                    "key=val",
			},
		},
	}

	validateTraces(t, expected, traces)
}

func validateTraces(t *testing.T, expected []*span, traces pdata.Traces) {
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
	}
}

func TestExportTraceDataRespectsContext(t *testing.T) {
	traces := constructTraces([]pdata.Span{createSpan(
		"root",
		pdata.NewTraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
		pdata.NewSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9}),
		pdata.SpanID{},
	)})

	sender := &mockSender{}
	cfg := createDefaultConfig()
	exp := tracesExporter{
		cfg:    cfg.(*Config),
		sender: sender,
		logger: zap.NewNop(),
	}
	mockOTelTracesExporter, err := exporterhelper.NewTracesExporter(
		cfg,
		componenttest.NewNopExporterCreateSettings(),
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
	traceID pdata.TraceID,
	spanID pdata.SpanID,
	parentSpanID pdata.SpanID,
) pdata.Span {
	span := pdata.NewSpan()
	span.SetName(name)
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	return span
}

func constructTraces(spans []pdata.Span) pdata.Traces {
	traces := pdata.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)
	rs := traces.ResourceSpans().AppendEmpty()
	rs.InstrumentationLibrarySpans().EnsureCapacity(1)
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()
	ils.Spans().EnsureCapacity(len(spans))
	for _, span := range spans {
		span.CopyTo(ils.Spans().AppendEmpty())
	}
	return traces
}

func consumeTraces(ptrace pdata.Traces) ([]*span, error) {
	ctx := context.Background()
	sender := &mockSender{}

	cfg := createDefaultConfig()
	exp := tracesExporter{
		cfg:    cfg.(*Config),
		sender: sender,
		logger: zap.NewNop(),
	}
	mockOTelTracesExporter, err := exporterhelper.NewTracesExporter(
		cfg,
		componenttest.NewNopExporterCreateSettings(),
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
	parents, followsFrom []string,
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
	}
	m.spans = append(m.spans, span)
	return nil
}
func (m *mockSender) Flush() error { return nil }
func (m *mockSender) Close()       {}
