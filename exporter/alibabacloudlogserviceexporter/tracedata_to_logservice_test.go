// Copyright 2020, OpenTelemetry Authors
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
	"io/ioutil"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
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
		if !reflect.DeepEqual(gotLogPairs[j], wantLogs[j]) {
			t.Errorf("Unsuccessful conversion \nGot:\n\t%v\nWant:\n\t%v", gotLogPairs[j], wantLogs[j])
		}
	}
}

func loadFromJSON(file string, obj interface{}) error {
	blob, err := ioutil.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(blob, obj)
	}

	return err
}

func constructSpanData() pdata.Traces {
	traces := pdata.NewTraces()
	traces.ResourceSpans().EnsureCapacity(1)
	rspans := traces.ResourceSpans().AppendEmpty()
	fillResource(rspans.Resource())
	rspans.InstrumentationLibrarySpans().EnsureCapacity(1)
	ispans := rspans.InstrumentationLibrarySpans().AppendEmpty()
	ispans.InstrumentationLibrary().SetName("golang-sls-exporter")
	ispans.InstrumentationLibrary().SetVersion("v0.1.0")
	ispans.Spans().EnsureCapacity(2)
	fillHTTPClientSpan(ispans.Spans().AppendEmpty())
	fillHTTPServerSpan(ispans.Spans().AppendEmpty())
	return traces
}

func fillResource(resource pdata.Resource) {
	attrs := resource.Attributes()
	attrs.InsertString(conventions.AttributeServiceName, "signup_aggregator")
	attrs.InsertString(conventions.AttributeHostName, "xxx.et15")
	attrs.InsertString(conventions.AttributeContainerName, "signup_aggregator")
	attrs.InsertString(conventions.AttributeContainerImageName, "otel/signupaggregator")
	attrs.InsertString(conventions.AttributeContainerImageTag, "v1")
	attrs.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attrs.InsertString(conventions.AttributeCloudAccountID, "999999998")
	attrs.InsertString(conventions.AttributeCloudRegion, "us-west-2")
	attrs.InsertString(conventions.AttributeCloudAvailabilityZone, "us-west-1b")
}

func fillHTTPClientSpan(span pdata.Span) {
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
	span.SetKind(pdata.SpanKindClient)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(endTime))
	span.SetTraceState("x:y")

	event := span.Events().AppendEmpty()
	event.SetName("event")
	event.SetTimestamp(1024)
	event.Attributes().InsertString("key", "value")

	link := span.Links().AppendEmpty()
	link.SetTraceState("link:state")
	link.Attributes().InsertString("link", "true")

	status := span.Status()
	status.SetCode(1)
	status.SetMessage("OK")
}

func fillHTTPServerSpan(span pdata.Span) {
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
	span.SetKind(pdata.SpanKindServer)
	span.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pdata.NewTimestampFromTime(endTime))

	status := span.Status()
	status.SetCode(2)
	status.SetMessage("something error")
}

func constructSpanAttributes(attributes map[string]interface{}) pdata.AttributeMap {
	attrs := pdata.NewAttributeMap()
	for key, value := range attributes {
		if cast, ok := value.(int); ok {
			attrs.InsertInt(key, int64(cast))
		} else if cast, ok := value.(int64); ok {
			attrs.InsertInt(key, cast)
		} else {
			attrs.InsertString(key, fmt.Sprintf("%v", value))
		}
	}
	return attrs
}

func newTraceID() pdata.TraceID {
	r := [16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F}
	return pdata.NewTraceID(r)
}

func newSegmentID() pdata.SpanID {
	r := [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x64, 0x7D, 0x98}
	return pdata.NewSpanID(r)
}

func TestSpanKindToShortString(t *testing.T) {
	assert.Equal(t, spanKindToShortString(pdata.SpanKindConsumer), "consumer")
	assert.Equal(t, spanKindToShortString(pdata.SpanKindProducer), "producer")
	assert.Equal(t, spanKindToShortString(pdata.SpanKindClient), "client")
	assert.Equal(t, spanKindToShortString(pdata.SpanKindServer), "server")
	assert.Equal(t, spanKindToShortString(pdata.SpanKindInternal), "internal")
	assert.Equal(t, spanKindToShortString(pdata.SpanKindUnspecified), "")
}

func TestStatusCodeToShortString(t *testing.T) {
	assert.Equal(t, statusCodeToShortString(pdata.StatusCodeOk), "OK")
	assert.Equal(t, statusCodeToShortString(pdata.StatusCodeError), "ERROR")
	assert.Equal(t, statusCodeToShortString(pdata.StatusCodeUnset), "UNSET")
}
