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

// Package spandata defines translators from Trace proto spans to OpenCensus Go spanData.
package stackdriverexporter

import (
	"context"
	"errors"
	"time"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/tracestate"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/kv/value"
	apitrace "go.opentelemetry.io/otel/api/trace"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/codes"
)

var errNilSpan = errors.New("expected a non-nil span")

// ProtoSpanToOCSpanData transforms a protobuf span into the equivalent trace.SpanData one.
// When resource is not nil, then its labels are attached to span attributes
func protoSpanToOCSpanData(span *tracepb.Span, resource *resourcepb.Resource) (*trace.SpanData, error) {
	if span == nil {
		return nil, errNilSpan
	}

	sTracestate, err := protoTracestateToOCTracestate(span.Tracestate)
	if err != nil {
		return nil, err
	}

	sc := trace.SpanContext{Tracestate: sTracestate}
	copy(sc.TraceID[:], span.TraceId)
	copy(sc.SpanID[:], span.SpanId)
	var parentSpanID trace.SpanID
	copy(parentSpanID[:], span.ParentSpanId)
	startTime, _ := ptypes.Timestamp(span.StartTime)
	endTime, _ := ptypes.Timestamp(span.EndTime)
	sd := &trace.SpanData{
		SpanContext:     sc,
		ParentSpanID:    parentSpanID,
		StartTime:       startTime,
		EndTime:         endTime,
		Name:            derefTruncatableString(span.Name),
		Attributes:      protoAttributesToOCAttributes(span.Attributes, resource),
		Links:           protoLinksToOCLinks(span.Links),
		Status:          protoStatusToOCStatus(span.Status),
		SpanKind:        protoSpanKindToOCSpanKind(span.Kind),
		MessageEvents:   protoTimeEventsToOCMessageEvents(span.TimeEvents),
		Annotations:     protoTimeEventsToOCAnnotations(span.TimeEvents),
		HasRemoteParent: protoSameProcessAsParentToOCHasRemoteParent(span.SameProcessAsParentSpan),
	}

	return sd, nil
}

func pdataResourceSpansToOTSpanData(rs pdata.ResourceSpans) ([]*export.SpanData, error) {
	resource := rs.Resource()
	var sds []*export.SpanData
	ilss := rs.InstrumentationLibrarySpans()
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		spans := ils.Spans()
		for j := 0; j < spans.Len(); j++ {
			sd, err := pdataSpanToOTSpanData(spans.At(j), resource, ils.InstrumentationLibrary())
			if err != nil {
				return nil, err
			}
			sds = append(sds, sd)
		}
	}

	return sds, nil
}

func pdataSpanToOTSpanData(
	span pdata.Span, 
	resource pdata.Resource, 
	il pdata.InstrumentationLibrary,
) (*export.SpanData, error) {
	if span.IsNil() {
		return nil, errNilSpan
	}
	sc := apitrace.SpanContext{}
	copy(sc.TraceID[:], span.TraceID())
	copy(sc.SpanID[:], span.SpanID())
	var parentSpanID apitrace.SpanID
	copy(parentSpanID[:], span.ParentSpanID())
	startTime := time.Unix(0, int64(span.StartTime()))
	endTime := time.Unix(0, int64(span.EndTime()))
	status := span.Status()
	r := sdkresource.New(
		pdataAttributesToOTAttributes(pdata.NewAttributeMap(), resource)...,
	)
	
	sd := &export.SpanData{
		SpanContext:     sc,
		ParentSpanID:    parentSpanID,
		SpanKind:        pdataSpanKindToOTSpanKind(span.Kind()),
		StartTime:       startTime,
		EndTime:         endTime,
		Name:            span.Name(),
		Attributes:      pdataAttributesToOTAttributes(span.Attributes(), resource),
		Links:           pdataLinksToOTLinks(span.Links()),
		MessageEvents:   pdataEventsToOTMessageEvents(span.Events()),
		HasRemoteParent: false, // no field for this in pdata Span
		StatusCode:      pdataStatusCodeToGRPCCode(status.Code()),
		StatusMessage:   status.Message(),
		DroppedAttributeCount: int(span.DroppedAttributesCount()),
		DroppedMessageEventCount: int(span.DroppedEventsCount()),
		DroppedLinkCount: int(span.DroppedLinksCount()),
		Resource: r,
	}
	if !il.IsNil() {
		sd.InstrumentationLibrary = instrumentation.Library{
			Name: il.Name(),
			Version: il.Version(),
		}
	}

	return sd, nil
}

func pdataSpanKindToOTSpanKind(k pdata.SpanKind) apitrace.SpanKind {
	switch k {
	case pdata.SpanKindUNSPECIFIED:
		return apitrace.SpanKindInternal
	case pdata.SpanKindINTERNAL:
		return apitrace.SpanKindInternal
	case pdata.SpanKindSERVER:
		return apitrace.SpanKindServer
	case pdata.SpanKindCLIENT:
		return apitrace.SpanKindClient
	case pdata.SpanKindPRODUCER:
		return apitrace.SpanKindProducer
	case pdata.SpanKindCONSUMER:
		return apitrace.SpanKindConsumer
	default:
		return apitrace.SpanKindUnspecified
	}
}

func pdataStatusCodeToGRPCCode(c pdata.StatusCode) codes.Code {
	return codes.Code(c)
}

func pdataAttributesToOTAttributes(attrs pdata.AttributeMap, resource pdata.Resource) []kv.KeyValue {
	otAttrs := make([]kv.KeyValue, 0, attrs.Len())
	appendAttrs := func(m pdata.AttributeMap) {
		m.ForEach(func(k string, v pdata.AttributeValue) {
			var newVal value.Value
			switch v.Type() {
			case pdata.AttributeValueSTRING:
				newVal = value.String(v.StringVal())
			case pdata.AttributeValueBOOL:
				newVal = value.Bool(v.BoolVal())
			case pdata.AttributeValueINT:
				newVal = value.Int64(v.IntVal())
			// pdata Double, Array, and Map cannot be converted to value.Value
			default:
				return
			}
			otAttrs = append(otAttrs, kv.KeyValue{
				Key: kv.Key(k),
				Value: newVal,
			})
		})
	}
	if !resource.IsNil() {
		appendAttrs(resource.Attributes())
	}
	appendAttrs(attrs)
	return otAttrs
}

func pdataLinksToOTLinks(links pdata.SpanLinkSlice) []apitrace.Link {
	size := links.Len()
	otLinks := make([]apitrace.Link, 0, size)
	for i := 0; i < size; i++ {
		link := links.At(i)
		if link.IsNil() {
			continue
		}
		sc := apitrace.SpanContext{}
		copy(sc.TraceID[:], link.TraceID())
		copy(sc.SpanID[:], link.SpanID())
		otLinks = append(otLinks, apitrace.Link{
			SpanContext: sc,
			Attributes: pdataAttributesToOTAttributes(link.Attributes(), pdata.NewResource()),
		})
	}
	return otLinks
}

func pdataEventsToOTMessageEvents(events pdata.SpanEventSlice) []export.Event {
	size := events.Len()
	otEvents := make([]export.Event, 0, size)
	for i := 0; i < size; i++ {
		event := events.At(i)
		if event.IsNil() {
			continue
		}
		otEvents = append(otEvents, export.Event{
			Name: event.Name(),
			Attributes: pdataAttributesToOTAttributes(event.Attributes(), pdata.NewResource()),
			Time: time.Unix(0, int64(event.Timestamp())),
		})
	}
	return otEvents
}

func derefTruncatableString(ts *tracepb.TruncatableString) string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

func protoStatusToOCStatus(s *tracepb.Status) (ts trace.Status) {
	if s == nil {
		return
	}
	return trace.Status{
		Code:    s.Code,
		Message: s.Message,
	}
}

func protoTracestateToOCTracestate(ts *tracepb.Span_Tracestate) (*tracestate.Tracestate, error) {
	if ts == nil {
		return nil, nil
	}
	return tracestate.New(nil, protoEntriesToOCEntries(ts.Entries)...)
}

func protoEntriesToOCEntries(protoEntries []*tracepb.Span_Tracestate_Entry) []tracestate.Entry {
	ocEntries := make([]tracestate.Entry, 0, len(protoEntries))
	for _, protoEntry := range protoEntries {
		var entry tracestate.Entry
		if protoEntry != nil {
			entry.Key = protoEntry.Key
			entry.Value = protoEntry.Value
		}
		ocEntries = append(ocEntries, entry)
	}
	return ocEntries
}

func protoLinksToOCLinks(sls *tracepb.Span_Links) []trace.Link {
	if sls == nil || len(sls.Link) == 0 {
		return nil
	}
	links := make([]trace.Link, 0, len(sls.Link))
	for _, sl := range sls.Link {
		var traceID trace.TraceID
		var spanID trace.SpanID
		copy(traceID[:], sl.TraceId)
		copy(spanID[:], sl.SpanId)
		links = append(links, trace.Link{
			TraceID: traceID,
			SpanID:  spanID,
			Type:    protoLinkTypeToOCLinkType(sl.Type),
		})
	}
	return links
}

func protoLinkTypeToOCLinkType(lt tracepb.Span_Link_Type) trace.LinkType {
	switch lt {
	case tracepb.Span_Link_CHILD_LINKED_SPAN:
		return trace.LinkTypeChild
	case tracepb.Span_Link_PARENT_LINKED_SPAN:
		return trace.LinkTypeParent
	default:
		return trace.LinkTypeUnspecified
	}
}

func protoAttributesToOCAttributes(attrs *tracepb.Span_Attributes, resource *resourcepb.Resource) map[string]interface{} {
	if attrs == nil && resource == nil {
		return nil
	}

	ocAttrsMap := make(map[string]interface{})

	if resource != nil {
		for key, value := range resource.Labels {
			ocAttrsMap[key] = value
		}
	}

	if attrs == nil || len(attrs.AttributeMap) == 0 {
		return ocAttrsMap
	}
	for key, attr := range attrs.AttributeMap {
		if attr == nil || attr.Value == nil {
			continue
		}
		switch value := attr.Value.(type) {
		case *tracepb.AttributeValue_BoolValue:
			ocAttrsMap[key] = value.BoolValue

		case *tracepb.AttributeValue_IntValue:
			ocAttrsMap[key] = value.IntValue

		case *tracepb.AttributeValue_StringValue:
			ocAttrsMap[key] = derefTruncatableString(value.StringValue)
		}
	}

	return ocAttrsMap
}

func protoTimeEventsToOCMessageEvents(tes *tracepb.Span_TimeEvents) []trace.MessageEvent {
	// TODO: (@odeke-em) file a bug with OpenCensus-Go and ask them why
	// only MessageEvents are implemented.
	if tes == nil || len(tes.TimeEvent) == 0 {
		return nil
	}

	ocmes := make([]trace.MessageEvent, 0, len(tes.TimeEvent))
	for _, te := range tes.TimeEvent {
		if te == nil || te.Value == nil {
			continue
		}
		tme, ok := te.Value.(*tracepb.Span_TimeEvent_MessageEvent_)
		if !ok || tme == nil {
			continue
		}
		me := tme.MessageEvent
		var ocme trace.MessageEvent
		// TODO: (@odeke-em) file an issue with OpenCensus-Go to ask why
		// they have these attributes as int64 yet the proto definitions
		// are uint64, this could be a potential loss of precision particularly
		// in very high traffic systems.
		ocme.MessageID = int64(me.Id)
		ocme.UncompressedByteSize = int64(me.UncompressedSize)
		ocme.CompressedByteSize = int64(me.CompressedSize)
		ocme.EventType = protoMessageEventTypeToOCEventType(me.Type)
		teTime, _ := ptypes.Timestamp(te.Time)
		ocme.Time = teTime
		ocmes = append(ocmes, ocme)
	}

	// For the purposes of rigorous equality during comparisons and tests,
	// the absence of message events should return nil instead of []
	if len(ocmes) == 0 {
		return nil
	}
	return ocmes
}

func protoTimeEventsToOCAnnotations(tes *tracepb.Span_TimeEvents) []trace.Annotation {
	if tes == nil || len(tes.TimeEvent) == 0 {
		return nil
	}

	ocanns := make([]trace.Annotation, 0, len(tes.TimeEvent))
	for _, te := range tes.TimeEvent {
		if te == nil || te.Value == nil {
			continue
		}
		tann, ok := te.Value.(*tracepb.Span_TimeEvent_Annotation_)
		if !ok || tann == nil {
			continue
		}
		me := tann.Annotation
		var ocann trace.Annotation
		// TODO: (@odeke-em) file an issue with OpenCensus-Go to ask why
		// they have these attributes as int64 yet the proto definitions
		// are uint64, this could be a potential loss of precision particularly
		// in very high traffic systems.
		teTime, _ := ptypes.Timestamp(te.Time)
		ocann.Time = teTime
		ocann.Message = me.Description.GetValue()
		ocann.Attributes = protoSpanAttributesToOCAttributes(me.Attributes)
		ocanns = append(ocanns, ocann)
	}

	// For the purposes of rigorous equality during comparisons and tests,
	// the absence of annotations should return nil instead of []
	if len(ocanns) == 0 {
		return nil
	}
	return ocanns
}

func protoSpanAttributesToOCAttributes(sa *tracepb.Span_Attributes) map[string]interface{} {
	if sa == nil || len(sa.AttributeMap) == 0 {
		return nil
	}

	amap := make(map[string]interface{})
	for pkey, pattr := range sa.AttributeMap {
		var value interface{}

		if pattr != nil && pattr.Value != nil {
			switch typ := pattr.Value.(type) {
			case *tracepb.AttributeValue_BoolValue:
				value = typ.BoolValue

			case *tracepb.AttributeValue_DoubleValue:
				value = typ.DoubleValue

			case *tracepb.AttributeValue_IntValue:
				value = typ.IntValue

			case *tracepb.AttributeValue_StringValue:
				value = typ.StringValue.GetValue()
			}
		}

		amap[pkey] = value
	}

	// For the purposes of rigorous equality during comparisons and
	// tests, the absence of attributes should return nil instead of []
	if len(amap) == 0 {
		return nil
	}
	return amap
}

func protoMessageEventTypeToOCEventType(st tracepb.Span_TimeEvent_MessageEvent_Type) trace.MessageEventType {
	switch st {
	case tracepb.Span_TimeEvent_MessageEvent_SENT:
		return trace.MessageEventTypeSent
	case tracepb.Span_TimeEvent_MessageEvent_RECEIVED:
		return trace.MessageEventTypeRecv
	default:
		return trace.MessageEventTypeUnspecified
	}
}

func protoSpanKindToOCSpanKind(kind tracepb.Span_SpanKind) int {
	switch kind {
	case tracepb.Span_CLIENT:
		return trace.SpanKindClient
	case tracepb.Span_SERVER:
		return trace.SpanKindServer
	default:
		return trace.SpanKindUnspecified
	}
}

// Translating a variable that states if the parent and the current span are in the same process
// to a variable that indicates whether the current span and parent are in different process.
func protoSameProcessAsParentToOCHasRemoteParent(sameProcessAsParent *wrappers.BoolValue) bool {
	if sameProcessAsParent == nil {
		return false
	}
	return !sameProcessAsParent.Value
}
