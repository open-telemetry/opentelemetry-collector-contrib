// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testdata

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pdata.Timestamp(TestSpanStartTime.UnixNano())

	TestSpanEventTime      = time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC)
	TestSpanEventTimestamp = pdata.Timestamp(TestSpanEventTime.UnixNano())

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pdata.Timestamp(TestSpanEndTime.UnixNano())
)

var (
	resourceAttributes1 = map[string]pdata.AttributeValue{
		"resource-attr": pdata.NewAttributeValueString("resource-attr-val-1"),
	}
	spanEventAttributes = map[string]pdata.AttributeValue{
		"span-event-attr": pdata.NewAttributeValueString("span-event-attr-val"),
	}
	spanLinkAttributes = map[string]pdata.AttributeValue{
		"span-link-attr": pdata.NewAttributeValueString("span-link-attr-val"),
	}
)

func GenerateTraceDataTwoSpansSameResource() pdata.Traces {
	td := GenerateTraceDataOneEmptyInstrumentationLibrary()
	rs0ils0 := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0)
	rs0ils0.Spans().Resize(2)
	fillSpanOne(rs0ils0.Spans().At(0))
	fillSpanTwo(rs0ils0.Spans().At(1))
	return td
}

func GenerateTraceDataOneEmptyInstrumentationLibrary() pdata.Traces {
	td := GenerateTraceDataNoLibraries()
	rs0 := td.ResourceSpans().At(0)
	rs0.InstrumentationLibrarySpans().Resize(1)
	return td
}

func GenerateTraceDataNoLibraries() pdata.Traces {
	td := GenerateTraceDataOneEmptyResourceSpans()
	rs0 := td.ResourceSpans().At(0)
	initResource1(rs0.Resource())
	return td
}

func GenerateTraceDataOneEmptyResourceSpans() pdata.Traces {
	td := GenerateTraceDataEmpty()
	td.ResourceSpans().Resize(1)
	return td
}

func GenerateTraceDataEmpty() pdata.Traces {
	td := pdata.NewTraces()
	return td
}

func fillSpanOne(span pdata.Span) {
	span.SetSpanID(pdata.NewSpanID([8]byte{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7}))
	span.SetParentSpanID(pdata.NewSpanID([8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}))
	span.SetTraceID(genTraceID())
	span.SetName("operationA")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	evs := span.Events()
	evs.Resize(2)
	ev0 := evs.At(0)
	ev0.SetTimestamp(TestSpanEventTimestamp)
	ev0.SetName("event-with-attr")
	initSpanEventAttributes(ev0.Attributes())
	ev0.SetDroppedAttributesCount(2)
	ev1 := evs.At(1)
	ev1.SetTimestamp(TestSpanEventTimestamp)
	ev1.SetName("event")
	ev1.SetDroppedAttributesCount(2)
	span.SetDroppedEventsCount(1)
	status := span.Status()
	status.SetCode(pdata.StatusCodeError)
	status.SetMessage("status-cancelled")
}

func fillSpanTwo(span pdata.Span) {
	span.SetName("operationB")
	span.SetKind(pdata.SpanKindServer)
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.Links().Resize(2)
	initSpanLinkAttributes(span.Links().At(0).Attributes())
	span.Links().At(0).SetDroppedAttributesCount(4)
	span.Links().At(1).SetDroppedAttributesCount(4)
	span.SetDroppedLinksCount(3)
	status := span.Status()
	status.SetCode(pdata.StatusCodeOk)
}

func initResource1(r pdata.Resource) {
	initResourceAttributes1(r.Attributes())
}

func initResourceAttributes1(dest pdata.AttributeMap) {
	dest.InitFromMap(resourceAttributes1)
}

func initSpanEventAttributes(dest pdata.AttributeMap) {
	dest.InitFromMap(spanEventAttributes)
}

func initSpanLinkAttributes(dest pdata.AttributeMap) {
	dest.InitFromMap(spanLinkAttributes)
}

func genTraceID() pdata.TraceID {
	var traceID [16]byte
	traceID[0] = 0xff
	return pdata.NewTraceID(traceID)
}
