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

package alibabacloudlogserviceexporter

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
	assert.Equal(t, len(gotLogs), 2)

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

func loadFromJSON(file string, obj interface{}) error {
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
	attrs.PutStr(conventions.AttributeServiceName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeHostName, "xxx.et15")
	attrs.PutStr(conventions.AttributeContainerName, "signup_aggregator")
	attrs.PutStr(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventions.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.PutStr(conventions.AttributeCloudAccountID, "999999998")
	attrs.PutStr(conventions.AttributeCloudRegion, "us-west-2")
	attrs.PutStr(conventions.AttributeCloudAvailabilityZone, "us-west-1b")
}

func fillHTTPClientSpan(span ptrace.Span) {
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeHTTPStatusCode] = 200
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
	attributes := make(map[string]interface{})
	attributes[conventions.AttributeHTTPMethod] = "GET"
	attributes[conventions.AttributeHTTPURL] = "https://api.example.com/users/junit"
	attributes[conventions.AttributeHTTPClientIP] = "192.168.15.32"
	attributes[conventions.AttributeHTTPStatusCode] = 200
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

func constructSpanAttributes(attributes map[string]interface{}) pcommon.Map {
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
	assert.Equal(t, spanKindToShortString(ptrace.SpanKindConsumer), "consumer")
	assert.Equal(t, spanKindToShortString(ptrace.SpanKindProducer), "producer")
	assert.Equal(t, spanKindToShortString(ptrace.SpanKindClient), "client")
	assert.Equal(t, spanKindToShortString(ptrace.SpanKindServer), "server")
	assert.Equal(t, spanKindToShortString(ptrace.SpanKindInternal), "internal")
	assert.Equal(t, spanKindToShortString(ptrace.SpanKindUnspecified), "")
}

func TestStatusCodeToShortString(t *testing.T) {
	assert.Equal(t, statusCodeToShortString(ptrace.StatusCodeOk), "OK")
	assert.Equal(t, statusCodeToShortString(ptrace.StatusCodeError), "ERROR")
	assert.Equal(t, statusCodeToShortString(ptrace.StatusCodeUnset), "UNSET")
}
