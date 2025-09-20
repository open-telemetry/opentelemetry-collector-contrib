// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slowsqlconnector

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

const (
	stringAttrName        = "stringAttrName"
	intAttrName           = "intAttrName"
	doubleAttrName        = "doubleAttrName"
	boolAttrName          = "boolAttrName"
	nullAttrName          = "nullAttrName"
	mapAttrName           = "mapAttrName"
	arrayAttrName         = "arrayAttrName"
	sampleLatency         = float64(11)
	sampleLatencyDuration = time.Duration(sampleLatency) * time.Second
)

type serviceSpans struct {
	serviceName string
	spans       []span
}

type span struct {
	name       string
	kind       ptrace.SpanKind
	statusCode ptrace.StatusCode
}

// buildSampleTrace builds the following trace:
//
//	service-a (server) ->
//	  service-a (client) ->
//	    service-b (server)
func buildSampleTrace() ptrace.Traces {
	traces := ptrace.NewTraces()

	initServiceSpans(
		serviceSpans{
			serviceName: "service-a",
			spans: []span{
				{
					name:       "svc-a-ep1",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
				{
					name:       "svc-a-ep2",
					kind:       ptrace.SpanKindClient,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(
		serviceSpans{
			serviceName: "service-b",
			spans: []span{
				{
					name:       "svc-b-ep1",
					kind:       ptrace.SpanKindServer,
					statusCode: ptrace.StatusCodeError,
				},
			},
		}, traces.ResourceSpans().AppendEmpty())
	initServiceSpans(serviceSpans{}, traces.ResourceSpans().AppendEmpty())
	return traces
}

func initServiceSpans(serviceSpans serviceSpans, spans ptrace.ResourceSpans) {
	if serviceSpans.serviceName != "" {
		spans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), serviceSpans.serviceName)
	}

	ils := spans.ScopeSpans().AppendEmpty()
	for _, span := range serviceSpans.spans {
		initSpan(span, ils.Spans().AppendEmpty())
	}
}

func initSpan(span span, s ptrace.Span) {
	s.SetKind(span.kind)
	s.SetName(span.name)
	s.Status().SetCode(span.statusCode)
	now := time.Now()
	s.Attributes().PutStr(string(conventions.DBSystemKey), "mysql")
	s.Attributes().PutStr(dbStatementKey, "select * from test")
	s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(sampleLatencyDuration)))

	s.Attributes().PutStr(stringAttrName, "stringAttrValue")
	s.Attributes().PutInt(intAttrName, 99)
	s.Attributes().PutDouble(doubleAttrName, 99.99)
	s.Attributes().PutBool(boolAttrName, true)
	s.Attributes().PutEmpty(nullAttrName)
	s.Attributes().PutEmptyMap(mapAttrName)
	s.Attributes().PutEmptySlice(arrayAttrName)
	s.SetTraceID([16]byte{byte(42)})
	s.SetSpanID([8]byte{byte(42)})

	e := s.Events().AppendEmpty()
	e.SetName("slow sql")
}
