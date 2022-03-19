// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skywalkingreceiver

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.8.0"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

var OtSpanTagsMapping = map[string]string{
	"url":         conventions.AttributeHTTPURL,
	"status_code": conventions.AttributeHTTPStatusCode,
	"db.type":     conventions.AttributeDBSystem,
	"db.instance": conventions.AttributeDBName,
	"mq.broker":   conventions.AttributeNetPeerName,
}

func SkywalkingToTraces(segment *agentV3.SegmentObject) pdata.Traces {
	traceData := pdata.NewTraces()

	swSpans := segment.Spans
	if swSpans == nil && len(swSpans) == 0 {
		return traceData
	}

	resourceSpan := traceData.ResourceSpans().AppendEmpty()
	rs := resourceSpan.Resource()
	for _, span := range swSpans {
		swTagsToInternalResource(span, rs)
		rs.Attributes().Insert(conventions.AttributeServiceName, pdata.NewAttributeValueString(segment.GetService()))
		rs.Attributes().Insert(conventions.AttributeServiceInstanceID, pdata.NewAttributeValueString(segment.GetServiceInstance()))
	}

	il := resourceSpan.InstrumentationLibrarySpans().AppendEmpty()
	swSpansToOtlpSpans(segment.GetTraceId(), swSpans, il.Spans())

	return traceData
}

func swTagsToInternalResource(span *agentV3.SpanObject, dest pdata.Resource) {
	if span == nil {
		return
	}

	attrs := dest.Attributes()
	attrs.Clear()

	tags := span.Tags
	if tags == nil {
		return
	}

	for _, tag := range tags {
		otKey, ok := OtSpanTagsMapping[tag.Key]
		if ok {
			attrs.UpsertString(otKey, tag.Value)
		}
	}
}

func swSpansToSpanSlice(traceId string, spans []*agentV3.SpanObject, dest pdata.SpanSlice) {
	if len(spans) == 0 {
		return
	}

	dest.EnsureCapacity(len(spans))
	for _, span := range spans {
		if span == nil {
			continue
		}
		swSpanToOtelSpan(traceId, span, dest.AppendEmpty())
	}
}

func swSpanToSpan(traceId string, span *agentV3.SpanObject, dest pdata.Span) {
	dest.SetTraceID(stringToTraceID(traceId))
	dest.SetSpanID(uInt32ToSpanID(uint32(span.GetSpanId())))

	// parent spanid = -1, means(root span) no parent span in skywalking,so just make otlp's parent span id empty.
	if span.ParentSpanId != -1 {
		dest.SetParentSpanID(uInt32ToSpanID(uint32(span.GetParentSpanId())))
	}

	dest.SetName(span.OperationName)
	dest.SetStartTimestamp(microsecondsToTimestamp(span.GetStartTime()))
	dest.SetEndTimestamp(microsecondsToTimestamp(span.GetEndTime()))

	attrs := dest.Attributes()
	attrs.EnsureCapacity(len(span.Tags))
	swKvPairsToInternalAttributes(span.Tags, attrs)
	// drop the attributes slice if all of them were replaced during translation
	if attrs.Len() == 0 {
		attrs.Clear()
	}

	setInternalSpanStatus(span, dest.Status())

	switch {
	case span.SpanLayer == agentV3.SpanLayer_MQ:
		if span.SpanType == agentV3.SpanType_Entry {
			dest.SetKind(pdata.SpanKindConsumer)
		} else if span.SpanType == agentV3.SpanType_Exit {
			dest.SetKind(pdata.SpanKindProducer)
		}
	case span.GetSpanType() == agentV3.SpanType_Exit:
		dest.SetKind(pdata.SpanKindClient)
	case span.GetSpanType() == agentV3.SpanType_Entry:
		dest.SetKind(pdata.SpanKindServer)
	case span.GetSpanType() == agentV3.SpanType_Local:
		dest.SetKind(pdata.SpanKindInternal)
	default:
		dest.SetKind(pdata.SpanKindUnspecified)
	}

	swLogsToSpanEvents(span.GetLogs(), dest.Events())
	// swkyalking: In the across thread and across process, these references targeting the parent segments.
	swReferencesToSpanLinks(span.Refs, int64(span.ParentSpanId), dest.Links())
}

func swReferencesToSpanLinks(refs []*agentV3.SegmentReference, excludeParentID int64, dest pdata.SpanLinkSlice) {
	if len(refs) == 0 {
		return
	}

	dest.EnsureCapacity(len(refs))

	for _, ref := range refs {
		parentTraceSegmentID := ""
		for _, part := range ref.ParentTraceSegmentId {
			parentTraceSegmentID += fmt.Sprintf("%d", part)
		}
		link := dest.AppendEmpty()
		link.SetTraceID(stringToTraceID(ref.TraceId))
		link.SetSpanID(stringToParentSpanId(ref.ParentTraceSegmentId))
	}
}

func setInternalSpanStatus(span *agentV3.SpanObject, dest pdata.SpanStatus) {
	statusCode := pdata.StatusCodeUnset
	statusMessage := ""

	if span.GetIsError() {
		statusCode = pdata.StatusCodeError
		statusMessage = "ERROR"
	} else {
		statusCode = pdata.StatusCodeOk
		statusMessage = "SUCCESS"
	}

	dest.SetCode(statusCode)
	dest.SetMessage(statusMessage)
}

func swLogsToSpanEvents(logs []*agentV3.Log, dest pdata.SpanEventSlice) {
	if len(logs) == 0 {
		return
	}
	dest.EnsureCapacity(len(logs))

	for i, log := range logs {
		var event pdata.SpanEvent
		if dest.Len() > i {
			event = dest.At(i)
		} else {
			event = dest.AppendEmpty()
		}

		event.SetName("logs")
		event.SetTimestamp(microsecondsToTimestamp(log.GetTime()))
		if len(log.GetData()) == 0 {
			continue
		}

		attrs := event.Attributes()
		attrs.Clear()
		attrs.EnsureCapacity(len(log.GetData()))
		swKvPairsToInternalAttributes(log.GetData(), attrs)
	}
}

func swKvPairsToInternalAttributes(pairs []*common.KeyStringValuePair, dest pdata.AttributeMap) {
	if pairs == nil {
		return
	}

	for _, pair := range pairs {
		dest.UpsertString(pair.Key, pair.Value)
	}
}

// microsecondsToTimestamp converts epoch microseconds to pdata.Timestamp
func microsecondsToTimestamp(ms int64) pdata.Timestamp {
	return pdata.NewTimestampFromTime(time.UnixMilli(ms))
}

func stringToTraceID(traceId string) pdata.TraceID {
	return pdata.NewTraceID(stringToBytes(traceId))
}

func stringToParentSpanId(traceId string) pdata.SpanID {
	traceID := stringTo8Bytes(traceId)
	return pdata.NewSpanID(traceID)
}

// uInt32ToSpanID converts the uint64 representation of a SpanID to pdata.SpanID.
func uInt32ToSpanID(id uint32) pdata.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint32(spanID[:], id)
	return pdata.NewSpanID(spanID)
}

func stringToBytes(s string) [16]byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[16]byte)(unsafe.Pointer(&bh))
}

func stringTo8Bytes(s string) [8]byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[8]byte)(unsafe.Pointer(&bh))
}
