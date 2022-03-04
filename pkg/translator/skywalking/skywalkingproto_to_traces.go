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

package skywalking // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/skywalking"
import (
	"encoding/binary"
	"fmt"
	"reflect"
	"unsafe"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.8.0"
	v3 "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

var OtSpanTagsMapping = map[string]string{
	"url":         conventions.AttributeHTTPURL,
	"status_code": conventions.AttributeHTTPStatusCode,
	"db.type":     conventions.AttributeDBSystem,
	"db.instance": conventions.AttributeDBName,
	"mq.broker":   conventions.AttributeNetPeerName,
}

func ToOtlpTraces(segment *agentV3.SegmentObject) pdata.Traces {
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

func swSpansToOtlpSpans(traceID string, spans []*agentV3.SpanObject, dest pdata.SpanSlice) {
	if len(spans) == 0 {
		return
	}

	dest.EnsureCapacity(len(spans))
	for _, span := range spans {
		if span == nil {
			continue
		}
		swSpanToOtelSpan(traceID, span, dest.AppendEmpty())
	}
}

func swSpanToOtelSpan(traceID string, span *agentV3.SpanObject, dest pdata.Span) {
	// TODO: fix traceID
	dest.SetTraceID(stringToTraceID(traceID))
	dest.SetTraceID(pdata.NewTraceID([16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}))

	dest.SetSpanID(uInt32ToSpanID(uint32(span.GetSpanId())))
	//dest.SetTraceState()

	// parent spanid = -1, means(root span) no parent span in skywalking,so just make otlp's parent span id empty.
	if span.ParentSpanId > -1 {
		dest.SetParentSpanID(uInt32ToSpanID(uint32(span.GetParentSpanId())))
	}

	dest.SetName(span.OperationName)

	dest.SetStartTimestamp(microsecondsToUnixNano(span.GetStartTime()))
	dest.SetEndTimestamp(microsecondsToUnixNano(span.GetEndTime()))

	attrs := dest.Attributes()
	attrs.EnsureCapacity(len(span.Tags))
	swKvPairsToInternalAttributes(span.Tags, attrs)
	// drop the attributes slice if all of them were replaced during translation
	if attrs.Len() == 0 {
		attrs.Clear()
	}

	setInternalSpanStatus(span, dest.Status())

	switch span.GetSpanType() {
	case agentV3.SpanType_Exit:
		dest.SetKind(pdata.SpanKindClient)
	case agentV3.SpanType_Entry:
		dest.SetKind(pdata.SpanKindServer)
	case agentV3.SpanType_Local:
		dest.SetKind(pdata.SpanKindInternal)
	default:
		dest.SetKind(pdata.SpanKindInternal)
	}

	swLogsToSpanEvents(span.GetLogs(), dest.Events())
	// swkyalking: In the across thread and across process, these references targeting the parent segments.
	swReferencesToSpanLinks(span.Refs, dest.Links())
}

func swReferencesToSpanLinks(refs []*agentV3.SegmentReference, dest pdata.SpanLinkSlice) {
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
		//link.SetTraceState()
		link.SetSpanID(stringToParentSpanID(ref.ParentTraceSegmentId))
	}
}

func setInternalSpanStatus(span *agentV3.SpanObject, dest pdata.SpanStatus) {
	var statusCode pdata.StatusCode
	var statusMessage string

	if span.GetIsError() {
		statusCode = pdata.StatusCodeOk
		statusMessage = "SUCCESS"
	} else {
		statusCode = pdata.StatusCodeError
		statusMessage = "ERROR"
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
		event.SetTimestamp(microsecondsToUnixNano(log.GetTime()))
		if len(log.GetData()) == 0 {
			continue
		}

		attrs := event.Attributes()
		attrs.Clear()
		attrs.EnsureCapacity(len(log.GetData()))
		swKvPairsToInternalAttributes(log.GetData(), attrs)
	}
}

func swKvPairsToInternalAttributes(pairs []*v3.KeyStringValuePair, dest pdata.AttributeMap) {
	if pairs == nil {
		return
	}

	for _, pair := range pairs {
		dest.UpsertString(pair.Key, pair.Value)
	}
}

// microsecondsToUnixNano converts epoch microseconds to pdata.Timestamp
func microsecondsToUnixNano(ms int64) pdata.Timestamp {
	return pdata.Timestamp(uint64(ms) * 1000)
}

func stringToTraceID(traceID string) pdata.TraceID {
	pTraceID := stringToBytes(traceID)
	return pdata.NewTraceID(pTraceID)
}

func stringToParentSpanID(traceID string) pdata.SpanID {
	pTraceID := stringTo8Bytes(traceID)
	return pdata.NewSpanID(pTraceID)
}

// TraceIDToUInt64Pair converts the pdata.TraceID to a pair of uint64 representation.
func TraceIDToUInt64Pair(traceID pdata.TraceID) (uint64, uint64) {
	bytes := traceID.Bytes()
	return binary.BigEndian.Uint64(bytes[:8]), binary.BigEndian.Uint64(bytes[8:])
}

// uInt32ToSpanID converts the uint64 representation of a SpanID to pdata.SpanID.
func uInt32ToSpanID(id uint32) pdata.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint32(spanID[:], id)
	return pdata.NewSpanID(spanID)
}

func stringToBytes(s string) [16]byte {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)
	var b [16]byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}

func stringTo8Bytes(s string) [8]byte {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)
	var b [8]byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}
