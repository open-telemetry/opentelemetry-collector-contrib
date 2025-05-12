// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
)

type logKeyValuePair struct {
	Key   string
	Value string
}

type logKeyValuePairs []logKeyValuePair

func (kv logKeyValuePairs) Len() int           { return len(kv) }
func (kv logKeyValuePairs) Swap(i, j int)      { kv[i], kv[j] = kv[j], kv[i] }
func (kv logKeyValuePairs) Less(i, j int) bool { return kv[i].Key < kv[j].Key }

func TestTraceDataToLogService(t *testing.T) {
	gotLogs := traceDataToLogServiceData(constructSpanData())
	assert.Len(t, gotLogs, 2)

	gotLogPairs := make([][]logKeyValuePair, 0, len(gotLogs))

	for _, log := range gotLogs {
		pairs := make([]logKeyValuePair, 0, len(log.Contents))
		for _, content := range log.Contents {
			pairs = append(pairs, logKeyValuePair{
				Key:   content.GetKey(),
				Value: content.GetValue(),
			})
		}
		gotLogPairs = append(gotLogPairs, pairs)
	}

	wantLogs := make([][]logKeyValuePair, 0, len(gotLogs))
	resultLogFile := "./testdata/logservice_trace_data.json"
	if err := loadFromJSON(resultLogFile, &wantLogs); err != nil {
		t.Errorf("Failed load log key value pairs from %q: %v", resultLogFile, err)
		return
	}
	for j := 0; j < len(gotLogs); j++ {
		sort.Sort(logKeyValuePairs(gotLogPairs[j]))
		sort.Sort(logKeyValuePairs(wantLogs[j]))
		assert.Equal(t, wantLogs[j], gotLogPairs[j])
	}
}

func loadFromJSON(file string, obj any) error {
	blob, err := os.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(blob, obj)
	}

	return err
}

func constructSpanData() ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)
	rspans := traces.ResourceSpans().AppendEmpty()
	fillResource(rspans.Resource())
	rspans.ScopeSpans().EnsureCapacity(1)
	ispans := rspans.ScopeSpans().AppendEmpty()
	ispans.Scope().SetName("golang-sls-exporter")
	ispans.Scope().SetVersion("v0.1.0")
	ispans.Spans().EnsureCapacity(2)
	fillHTTPClientSpan(ispans.Spans().AppendEmpty())
	fillHTTPServerSpan(ispans.Spans().AppendEmpty())
	return traces
}

func fillResource(resource pcommon.Resource) {
	attrs := resource.Attributes()
	attrs.PutStr(string(conventions.ServiceNameKey), "signup_aggregator")
	attrs.PutStr(string(conventions.HostNameKey), "xxx.et15")
	attrs.PutStr(string(conventions.ContainerNameKey), "signup_aggregator")
	attrs.PutStr(string(conventions.ContainerImageNameKey), "otel/signupaggregator")
	attrs.PutStr(string(conventions.ContainerImageTagKey), "v1")
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(conventions.CloudAccountIDKey), "999999998")
	attrs.PutStr(string(conventions.CloudRegionKey), "us-west-2")
	attrs.PutStr(string(conventions.CloudAvailabilityZoneKey), "us-west-1b")
}

func fillHTTPClientSpan(span ptrace.Span) {
	attributes := make(map[string]any)
	attributes[string(conventions.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventions.HTTPURLKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.HTTPStatusCodeKey)] = 200
	endTime := time.Unix(12300, 123456789)
	startTime := endTime.Add(-90 * time.Second)
	constructSpanAttributes(attributes).CopyTo(span.Attributes())

	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
	span.TraceState().FromRaw("x:y")

	event := span.Events().AppendEmpty()
	event.SetName("event")
	event.SetTimestamp(1024)
	event.Attributes().PutStr("key", "value")

	link := span.Links().AppendEmpty()
	link.TraceState().FromRaw("link:state")
	link.Attributes().PutStr("link", "true")

	status := span.Status()
	status.SetCode(1)
	status.SetMessage("OK")
}

func fillHTTPServerSpan(span ptrace.Span) {
	attributes := make(map[string]any)
	attributes[string(conventions.HTTPMethodKey)] = http.MethodGet
	attributes[string(conventions.HTTPURLKey)] = "https://api.example.com/users/junit"
	attributes[string(conventions.HTTPClientIPKey)] = "192.168.15.32"
	attributes[string(conventions.HTTPStatusCodeKey)] = 200
	endTime := time.Unix(12300, 123456789)
	startTime := endTime.Add(-90 * time.Second)
	constructSpanAttributes(attributes).CopyTo(span.Attributes())

	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := span.Status()
	status.SetCode(2)
	status.SetMessage("something error")
}

func constructSpanAttributes(attributes map[string]any) pcommon.Map {
	attrs := pcommon.NewMap()
	for key, value := range attributes {
		if cast, ok := value.(int); ok {
			attrs.PutInt(key, int64(cast))
		} else if cast, ok := value.(int64); ok {
			attrs.PutInt(key, cast)
		} else {
			attrs.PutStr(key, fmt.Sprintf("%v", value))
		}
	}
	return attrs
}

func newTraceID() pcommon.TraceID {
	r := [16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F}
	return pcommon.TraceID(r)
}

func newSegmentID() pcommon.SpanID {
	r := [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98}
	return pcommon.SpanID(r)
}

func TestSpanKindToShortString(t *testing.T) {
	assert.Equal(t, "consumer", spanKindToShortString(ptrace.SpanKindConsumer))
	assert.Equal(t, "producer", spanKindToShortString(ptrace.SpanKindProducer))
	assert.Equal(t, "client", spanKindToShortString(ptrace.SpanKindClient))
	assert.Equal(t, "server", spanKindToShortString(ptrace.SpanKindServer))
	assert.Equal(t, "internal", spanKindToShortString(ptrace.SpanKindInternal))
	assert.Empty(t, spanKindToShortString(ptrace.SpanKindUnspecified))
}

func TestStatusCodeToShortString(t *testing.T) {
	assert.Equal(t, "OK", statusCodeToShortString(ptrace.StatusCodeOk))
	assert.Equal(t, "ERROR", statusCodeToShortString(ptrace.StatusCodeError))
	assert.Equal(t, "UNSET", statusCodeToShortString(ptrace.StatusCodeUnset))
}
