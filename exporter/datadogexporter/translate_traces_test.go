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

package datadogexporter

import (
	"fmt"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

func NewResourceSpansData(mockTraceID []byte, mockSpanID []byte, mockParentSpanID []byte) pdata.ResourceSpans {
	// The goal of this test is to ensure that each span in
	// pdata.ResourceSpans is transformed to its *trace.SpanData correctly!

	endTime := time.Now().Round(time.Second)
	pdataEndTime := pdata.TimestampUnixNano(endTime.UnixNano())
	startTime := endTime.Add(-90 * time.Second)
	pdataStartTime := pdata.TimestampUnixNano(startTime.UnixNano())

	rs := pdata.NewResourceSpans()
	rs.InitEmpty()

	span := pdata.NewSpan()
	span.InitEmpty()
	traceID := pdata.NewTraceID(mockTraceID)
	spanID := pdata.NewSpanID(mockSpanID)
	parentSpanID := pdata.NewSpanID(mockParentSpanID)
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName("End-To-End Here")
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTime(pdataStartTime)
	span.SetEndTime(pdataEndTime)

	status := pdata.NewSpanStatus()
	status.InitEmpty()
	status.SetCode(13)
	status.SetMessage("This is not a drill!")
	status.CopyTo(span.Status())

	events := pdata.NewSpanEventSlice()
	events.Resize(2)

	events.At(0).SetTimestamp(pdataStartTime)
	events.At(0).SetName("start")

	events.At(1).SetTimestamp(pdataEndTime)
	events.At(1).SetName("end")
	pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	}).CopyTo(events.At(1).Attributes())
	events.MoveAndAppendTo(span.Events())

	pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
		"cache_hit":  pdata.NewAttributeValueBool(true),
		"timeout_ns": pdata.NewAttributeValueInt(12e9),
		"ping_count": pdata.NewAttributeValueInt(25),
		"agent":      pdata.NewAttributeValueString("ocagent"),
	}).CopyTo(span.Attributes())

	resource := pdata.NewResource()
	resource.InitEmpty()
	pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
		"namespace":    pdata.NewAttributeValueString("kube-system"),
		"service.name": pdata.NewAttributeValueString("test-resource-service-name"),
	}).CopyTo(resource.Attributes())

	resource.CopyTo(rs.Resource())

	il := pdata.NewInstrumentationLibrary()
	il.InitEmpty()
	il.SetName("test_il_name")
	il.SetVersion("test_il_version")

	spans := pdata.NewSpanSlice()
	spans.Append(span)

	ilss := pdata.NewInstrumentationLibrarySpansSlice()
	ilss.Resize(1)
	il.CopyTo(ilss.At(0).InstrumentationLibrary())
	spans.MoveAndAppendTo(ilss.At(0).Spans())
	ilss.CopyTo(rs.InstrumentationLibrarySpans())
	return rs
}

func TestBasicTracesTranslation(t *testing.T) {
	hostname := "testhostname"

	// generate mock trace, span and parent span ids
	mockTraceID := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := []byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := []byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	// create mock resource span data
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID)

	// translate mocks to datadog traces
	datadogPayload, err := resourceSpansToDatadogSpans(rs, hostname, &Config{})

	if err != nil {
		t.Fatalf("Failed to convert from pdata ResourceSpans to pb.TracePayload: %v", err)
	}

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure trace id gets translated to uint64 correctly
	assert.Equal(t, decodeAPMId(mockTraceID), datadogPayload.Traces[0].TraceID)

	// ensure the correct number of spans are expected
	assert.Equal(t, 1, len(datadogPayload.Traces[0].Spans))

	// ensure span's trace id matches payload trace id
	assert.Equal(t, datadogPayload.Traces[0].TraceID, datadogPayload.Traces[0].Spans[0].TraceID)

	// ensure span's spanId and parentSpanId are set correctly
	assert.Equal(t, decodeAPMId(mockSpanID), datadogPayload.Traces[0].Spans[0].SpanID)
	assert.Equal(t, decodeAPMId(mockParentSpanID), datadogPayload.Traces[0].Spans[0].ParentID)

	// ensure that span.resource defaults to otlp span.name
	assert.Equal(t, "End-To-End Here", datadogPayload.Traces[0].Spans[0].Resource)

	// ensure that span.name defaults to string representing instrumentation library if present
	assert.Equal(t, fmt.Sprintf("%s.%s", datadogPayload.Traces[0].Spans[0].Meta[tracetranslator.TagInstrumentationName], pdata.SpanKindSERVER), datadogPayload.Traces[0].Spans[0].Name)

	// ensure that span.type is based on otlp span.kind
	assert.Equal(t, "server", datadogPayload.Traces[0].Spans[0].Type)

	// ensure that span.meta and span.metrics pick up attibutes, instrumentation ibrary and resource attribs
	assert.Equal(t, 10, len(datadogPayload.Traces[0].Spans[0].Meta))
	assert.Equal(t, 1, len(datadogPayload.Traces[0].Spans[0].Metrics))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(1), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span meta also inccludes correctly sets resource attributes
	assert.Equal(t, "kube-system", datadogPayload.Traces[0].Spans[0].Meta["namespace"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure a duration and start time are calculated
	assert.NotNil(t, datadogPayload.Traces[0].Spans[0].Start)
	assert.NotNil(t, datadogPayload.Traces[0].Spans[0].Duration)
}
