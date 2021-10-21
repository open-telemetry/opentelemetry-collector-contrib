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

package googlecloudexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
)

func TestPDataResourceSpansToOTSpanData_endToEnd(t *testing.T) {
	// The goal of this test is to ensure that each span in
	// pdata.ResourceSpans is transformed to its *trace.SpanData correctly!

	endTime := time.Now().Round(time.Second)
	pdataEndTime := pdata.NewTimestampFromTime(endTime)
	startTime := endTime.Add(-90 * time.Second)
	pdataStartTime := pdata.NewTimestampFromTime(startTime)

	rs := pdata.NewResourceSpans()
	resource := rs.Resource()
	resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"namespace": pdata.NewAttributeValueString("kube-system"),
	})
	ilss := rs.InstrumentationLibrarySpans()
	il := ilss.AppendEmpty().InstrumentationLibrary()
	il.SetName("test_il_name")
	il.SetVersion("test_il_version")
	span := ilss.At(0).Spans().AppendEmpty()
	traceID := pdata.NewTraceID([16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F})
	spanID := pdata.NewSpanID([8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})
	parentSpanID := pdata.NewSpanID([8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8})
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName("End-To-End Here")
	span.SetKind(pdata.SpanKindServer)
	span.SetStartTimestamp(pdataStartTime)
	span.SetEndTimestamp(pdataEndTime)

	status := span.Status()
	status.SetCode(pdata.StatusCodeError)
	status.SetMessage("This is not a drill!")

	events := span.Events()
	event0 := events.AppendEmpty()
	event0.SetTimestamp(pdataStartTime)
	event0.SetName("start")

	event1 := events.AppendEmpty()
	event1.SetTimestamp(pdataEndTime)
	event1.SetName("end")
	event1.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	})

	links := span.Links()
	link0 := links.AppendEmpty()
	link0.SetTraceID(pdata.NewTraceID([16]byte{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF}))
	link0.SetSpanID(pdata.NewSpanID([8]byte{0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7}))
	link1 := links.AppendEmpty()
	link1.SetTraceID(pdata.NewTraceID([16]byte{0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF}))
	link1.SetSpanID(pdata.NewSpanID([8]byte{0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7}))

	span.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"cache_hit":  pdata.NewAttributeValueBool(true),
		"timeout_ns": pdata.NewAttributeValueInt(12e9),
		"ping_count": pdata.NewAttributeValueInt(25),
		"agent":      pdata.NewAttributeValueString("ocagent"),
	})

	gotOTSpanData := pdataResourceSpansToOTSpanData(rs)

	wantOTSpanData := &spanSnapshot{
		spanContext: apitrace.NewSpanContext(apitrace.SpanContextConfig{
			TraceID: apitrace.TraceID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
			SpanID:  apitrace.SpanID{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8},
		}),
		spanKind: apitrace.SpanKindServer,
		parent: apitrace.NewSpanContext(apitrace.SpanContextConfig{
			TraceID: apitrace.TraceID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
			SpanID:  apitrace.SpanID{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8},
		}),
		name:      "End-To-End Here",
		startTime: startTime,
		endTime:   endTime,
		events: []sdktrace.Event{
			{
				Time:       startTime,
				Name:       "start",
				Attributes: []attribute.KeyValue{},
			},
			{
				Time:       endTime,
				Name:       "end",
				Attributes: []attribute.KeyValue{attribute.Bool("flag", false)},
			},
		},
		links: []sdktrace.Link{
			{
				SpanContext: apitrace.NewSpanContext(apitrace.SpanContextConfig{
					TraceID: apitrace.TraceID{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF},
					SpanID:  apitrace.SpanID{0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7},
				}),
				Attributes: []attribute.KeyValue{},
			},
			{
				SpanContext: apitrace.NewSpanContext(apitrace.SpanContextConfig{
					TraceID: apitrace.TraceID{0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF},
					SpanID:  apitrace.SpanID{0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7},
				}),
				Attributes: []attribute.KeyValue{},
			},
		},
		status: sdktrace.Status{
			Code:        codes.Error,
			Description: "This is not a drill!",
		},
		attributes: []attribute.KeyValue{
			attribute.String("namespace", "kube-system"),
			attribute.Int64("ping_count", 25),
			attribute.String("agent", "ocagent"),
			attribute.Bool("cache_hit", true),
			attribute.Int64("timeout_ns", 12e9),
		},
		instrumentationLibrary: instrumentation.Library{
			Name:    "test_il_name",
			Version: "test_il_version",
		},
	}

	assert.EqualValues(t, 1, len(gotOTSpanData))
	assert.EqualValues(t, wantOTSpanData.SpanContext(), gotOTSpanData[0].SpanContext())
	assert.EqualValues(t, wantOTSpanData.SpanKind(), gotOTSpanData[0].SpanKind())
	assert.EqualValues(t, wantOTSpanData.Parent(), gotOTSpanData[0].Parent())
	assert.EqualValues(t, wantOTSpanData.Name(), gotOTSpanData[0].Name())
	assert.EqualValues(t, wantOTSpanData.StartTime(), gotOTSpanData[0].StartTime())
	assert.EqualValues(t, wantOTSpanData.EndTime(), gotOTSpanData[0].EndTime())
	assert.EqualValues(t, wantOTSpanData.Events(), gotOTSpanData[0].Events())
	assert.EqualValues(t, wantOTSpanData.Links(), gotOTSpanData[0].Links())
	assert.EqualValues(t, wantOTSpanData.Status(), gotOTSpanData[0].Status())
	assert.ElementsMatch(t, wantOTSpanData.Attributes(), gotOTSpanData[0].Attributes())
	assert.EqualValues(t, wantOTSpanData.InstrumentationLibrary(), gotOTSpanData[0].InstrumentationLibrary())
}
