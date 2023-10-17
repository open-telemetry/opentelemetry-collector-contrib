// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
)

var expectedLogBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Body":"foo bar","InstrumentationScope.attributes.attr":"23","InstrumentationScope.name":"client span","InstrumentationScope.version":"1.2.3","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","SeverityNumber":2,"SeverityText":"INFO","SpanId":"1920212223242526","TraceFlags":3,"TraceId":"01020304050607080807060504030201"}`

var expectedSpanBody = `{"@timestamp":"2023-04-19T03:04:05.000000006Z","Attributes.service.instance.id":"23","Duration":1000000,"EndTimestamp":"2023-04-19T03:04:06.000000006Z","Events.fooEvent.evnetMockBar":"bar","Events.fooEvent.evnetMockFoo":"foo","Events.fooEvent.time":"2023-04-19T03:04:05.000000006Z","Kind":"SPAN_KIND_CLIENT","Link":"[{\"attribute\":{},\"spanID\":\"\",\"traceID\":\"01020304050607080807060504030200\"}]","Name":"client span","Resource.cloud.platform":"aws_elastic_beanstalk","Resource.cloud.provider":"aws","Resource.deployment.environment":"BETA","Resource.service.instance.id":"23","Resource.service.name":"some-service","Resource.service.version":"env-version-1234","SpanId":"1920212223242526","TraceId":"01020304050607080807060504030201","TraceStatus":0}`

func TestEncodeLog(t *testing.T) {
	model := &encodeModel{dedup: true, dedot: false}
	td := mockResourceLogs()
	
	logByte, err := model.encodeLog(td.ResourceLogs().At(0).Resource(), td.ResourceLogs().At(0).ScopeLogs().At(0).Scope(), td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0))
	assert.NoError(t, err)
	assert.Equal(t, expectedLogBody, string(logByte))
}

func TestEncodeSpan(t *testing.T) {
	model := &encodeModel{dedup: true, dedot: false}
	td := mockResourceSpans()
	spanByte, err := model.encodeSpan(td.ResourceSpans().At(0).Resource(), td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))
	assert.NoError(t, err)
	assert.Equal(t, expectedSpanBody, string(spanByte))
}

func mockResourceLogs() plog.Logs {
	logs := plog.NewLogs()

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	attr := resourceLogs.Resource().Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_elastic_beanstalk")
	attr.PutStr("deployment.environment", "BETA")
	attr.PutStr("service.instance.id", "23")
	attr.PutStr("service.version", "env-version-1234")

	resourceLogs.Resource().Attributes().PutStr(semconv.AttributeServiceName, "some-service")

	scopeSpans := resourceLogs.ScopeLogs().AppendEmpty()

	scope := scopeSpans.Scope()

	scope.SetName("client span")
	scope.SetVersion("1.2.3")
	scope.Attributes().PutStr("attr", "23")

	log := scopeSpans.LogRecords().AppendEmpty()

	tStart := time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)

	log.SetTimestamp(pcommon.NewTimestampFromTime(tStart))
	log.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	log.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	log.SetFlags(3)
	log.SetSeverityText("INFO")
	log.SetSeverityNumber(2)
	log.Body().SetStr("foo bar")
	log.Attributes().PutStr("service.instance.id", "23")

	return logs
}

func mockResourceSpans() ptrace.Traces {
	traces := ptrace.NewTraces()

	resourceSpans := traces.ResourceSpans().AppendEmpty()
	attr := resourceSpans.Resource().Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_elastic_beanstalk")
	attr.PutStr("deployment.environment", "BETA")
	attr.PutStr("service.instance.id", "23")
	attr.PutStr("service.version", "env-version-1234")

	resourceSpans.Resource().Attributes().PutStr(semconv.AttributeServiceName, "some-service")

	tStart := time.Date(2023, 4, 19, 3, 4, 5, 6, time.UTC)
	tEnd := time.Date(2023, 4, 19, 3, 4, 6, 6, time.UTC)

	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("client span")
	span.SetSpanID([8]byte{0x19, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26})
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1})
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
	span.Attributes().PutStr("service.instance.id", "23")
	span.Links().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 0})

	event := span.Events().AppendEmpty()
	event.SetName("fooEvent")
	event.SetTimestamp(pcommon.NewTimestampFromTime(tStart))
	event.Attributes().PutStr("evnetMockFoo", "foo")
	event.Attributes().PutStr("evnetMockBar", "bar")
	return traces
}
