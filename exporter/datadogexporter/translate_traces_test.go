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
	// "fmt"
	// "math"
	// "net/http"
	// "strconv"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	// "github.com/stretchr/testify/require"
	// "go.opencensus.io/trace"
	// otlpcommon "go.opentelemetry.io/collector/internal/data/opentelemetry-proto-gen/common/v1"
	// tracev1 "go.opentelemetry.io/collector/internal/data/opentelemetry-proto-gen/trace/v1"
	// "go.opentelemetry.io/collector/translator/conventions"
	// tracetranslator "go.opentelemetry.io/collector/translator/trace"
	// "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
)

func NewResourceSpansData() pdata.ResourceSpans {
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
	traceID := pdata.NewTraceID([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F})
	spanID := pdata.NewSpanID([]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8})
	parentSpanID := pdata.NewSpanID([]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8})
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

	links := pdata.NewSpanLinkSlice()
	links.Resize(2)
	links.At(0).SetTraceID(pdata.NewTraceID([]byte{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF}))
	links.At(0).SetSpanID(pdata.NewSpanID([]byte{0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7}))
	links.At(1).SetTraceID(pdata.NewTraceID([]byte{0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF}))
	links.At(1).SetSpanID([]byte{0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7})
	links.MoveAndAppendTo(span.Links())

	pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
		"cache_hit":  pdata.NewAttributeValueBool(true),
		"timeout_ns": pdata.NewAttributeValueInt(12e9),
		"ping_count": pdata.NewAttributeValueInt(25),
		"agent":      pdata.NewAttributeValueString("ocagent"),
	}).CopyTo(span.Attributes())

	resource := pdata.NewResource()
	resource.InitEmpty()
	pdata.NewAttributeMap().InitFromMap(map[string]pdata.AttributeValue{
		"namespace": pdata.NewAttributeValueString("kube-system"),
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

func TestTracesValue(t *testing.T) {
	hostname := "testhostname"
	env := "testenv"

	payload := pb.TracePayload{
		HostName:     hostname,
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	rs := NewResourceSpansData()
	datadogPayload, err := resourceSpansToDatadogSpans(rs, hostname, &Config{})

	assert.Equal(t, nil, err)
	assert.IsType(t, pb.TracePayload{}, datadogPayload)
	assert.Equal(t, payload.HostName, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))
}
