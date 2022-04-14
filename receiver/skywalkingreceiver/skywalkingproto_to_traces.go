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

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"encoding/binary"
	"reflect"
	"time"
	"unsafe"

	conventions "go.opentelemetry.io/collector/model/semconv/v1.8.0"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agentV3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	AttributeRefType                  = "refType"
	AttributeParentService            = "parent.service"
	AttributeParentInstance           = "parent.service.instance"
	AttributeParentEndpoint           = "parent.endpoint"
	AttributeNetworkAddressUsedAtPeer = "network.AddressUsedAtPeer"
)

var otSpanTagsMapping = map[string]string{
	"url":         conventions.AttributeHTTPURL,
	"status_code": conventions.AttributeHTTPStatusCode,
	"db.type":     conventions.AttributeDBSystem,
	"db.instance": conventions.AttributeDBName,
	"mq.broker":   conventions.AttributeNetPeerName,
}

func SkywalkingToTraces(segment *agentV3.SegmentObject) ptrace.Traces {
	traceData := ptrace.NewTraces()

	swSpans := segment.Spans
	if swSpans == nil && len(swSpans) == 0 {
		return traceData
	}

	resourceSpan := traceData.ResourceSpans().AppendEmpty()
	rs := resourceSpan.Resource()
	for _, span := range swSpans {
		swTagsToInternalResource(span, rs)
		rs.Attributes().Insert(conventions.AttributeServiceName, pcommon.NewValueString(segment.GetService()))
		rs.Attributes().Insert(conventions.AttributeServiceInstanceID, pcommon.NewValueString(segment.GetServiceInstance()))
	}

	il := resourceSpan.ScopeSpans().AppendEmpty()
	swSpansToSpanSlice(segment.GetTraceId(), swSpans, il.Spans())

	return traceData
}

func swTagsToInternalResource(span *agentV3.SpanObject, dest pcommon.Resource) {
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
		otKey, ok := otSpanTagsMapping[tag.Key]
		if ok {
			attrs.UpsertString(otKey, tag.Value)
		}
	}
}

func swSpansToSpanSlice(traceID string, spans []*agentV3.SpanObject, dest ptrace.SpanSlice) {
	if len(spans) == 0 {
		return
	}

	dest.EnsureCapacity(len(spans))
	for _, span := range spans {
		if span == nil {
			continue
		}
		swSpanToSpan(traceID, span, dest.AppendEmpty())
	}
}

func swSpanToSpan(traceID string, span *agentV3.SpanObject, dest ptrace.Span) {
	dest.SetTraceID(stringToTraceID(traceID))
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
			dest.SetKind(ptrace.SpanKindConsumer)
		} else if span.SpanType == agentV3.SpanType_Exit {
			dest.SetKind(ptrace.SpanKindProducer)
		}
	case span.GetSpanType() == agentV3.SpanType_Exit:
		dest.SetKind(ptrace.SpanKindClient)
	case span.GetSpanType() == agentV3.SpanType_Entry:
		dest.SetKind(ptrace.SpanKindServer)
	case span.GetSpanType() == agentV3.SpanType_Local:
		dest.SetKind(ptrace.SpanKindInternal)
	default:
		dest.SetKind(ptrace.SpanKindUnspecified)
	}

	swLogsToSpanEvents(span.GetLogs(), dest.Events())
	// skywalking: In the across thread and across processes, these references target the parent segments.
	swReferencesToSpanLinks(span.Refs, dest.Links())
}

func swReferencesToSpanLinks(refs []*agentV3.SegmentReference, dest ptrace.SpanLinkSlice) {
	if len(refs) == 0 {
		return
	}

	dest.EnsureCapacity(len(refs))

	for _, ref := range refs {
		link := dest.AppendEmpty()
		link.SetTraceID(stringToTraceID(ref.TraceId))
		link.SetSpanID(stringToParentSpanID(ref.ParentTraceSegmentId))
		link.SetTraceState("")
		kvParis := []*common.KeyStringValuePair{
			{
				Key:   AttributeParentService,
				Value: ref.ParentService,
			},
			{
				Key:   AttributeParentInstance,
				Value: ref.ParentServiceInstance,
			},
			{
				Key:   AttributeParentEndpoint,
				Value: ref.ParentEndpoint,
			},
			{
				Key:   AttributeNetworkAddressUsedAtPeer,
				Value: ref.NetworkAddressUsedAtPeer,
			},
			{
				Key:   AttributeRefType,
				Value: ref.RefType.String(),
			},
		}
		swKvPairsToInternalAttributes(kvParis, link.Attributes())
	}
}

func setInternalSpanStatus(span *agentV3.SpanObject, dest ptrace.SpanStatus) {
	if span.GetIsError() {
		dest.SetCode(ptrace.StatusCodeError)
		dest.SetMessage("ERROR")
	} else {
		dest.SetCode(ptrace.StatusCodeOk)
		dest.SetMessage("SUCCESS")
	}
}

func swLogsToSpanEvents(logs []*agentV3.Log, dest ptrace.SpanEventSlice) {
	if len(logs) == 0 {
		return
	}
	dest.EnsureCapacity(len(logs))

	for i, log := range logs {
		var event ptrace.SpanEvent
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

func swKvPairsToInternalAttributes(pairs []*common.KeyStringValuePair, dest pcommon.Map) {
	if pairs == nil {
		return
	}

	for _, pair := range pairs {
		dest.UpsertString(pair.Key, pair.Value)
	}
}

// microsecondsToTimestamp converts epoch microseconds to pcommon.Timestamp
func microsecondsToTimestamp(ms int64) pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(time.UnixMilli(ms))
}

func stringToTraceID(traceID string) pcommon.TraceID {
	return pcommon.NewTraceID(unsafeStringToBytes(traceID))
}

func stringToParentSpanID(traceID string) pcommon.SpanID {
	return pcommon.NewSpanID(unsafeStringTo8Bytes(traceID))
}

// uInt32ToSpanID converts the uint64 representation of a SpanID to pcommon.SpanID.
func uInt32ToSpanID(id uint32) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint32(spanID[:], id)
	return pcommon.NewSpanID(spanID)
}

func unsafeStringToBytes(s string) [16]byte {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)

	var b [16]byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}

func unsafeStringTo8Bytes(s string) [8]byte {
	p := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&s)).Data)
	var b [8]byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	hdr.Data = uintptr(p)
	hdr.Cap = len(s)
	hdr.Len = len(s)
	return b
}
