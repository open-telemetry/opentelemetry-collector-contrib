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
	"errors"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	apitrace "go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc/codes"
)

var errNilSpan = errors.New("expected a non-nil span")

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
	copy(sc.TraceID[:], span.TraceID().Bytes())
	copy(sc.SpanID[:], span.SpanID().Bytes())
	var parentSpanID apitrace.SpanID
	copy(parentSpanID[:], span.ParentSpanID().Bytes())
	startTime := time.Unix(0, int64(span.StartTime()))
	endTime := time.Unix(0, int64(span.EndTime()))
	r := sdkresource.New(
		pdataAttributesToOTAttributes(pdata.NewAttributeMap(), resource)...,
	)

	sd := &export.SpanData{
		SpanContext:              sc,
		ParentSpanID:             parentSpanID,
		SpanKind:                 pdataSpanKindToOTSpanKind(span.Kind()),
		StartTime:                startTime,
		EndTime:                  endTime,
		Name:                     span.Name(),
		Attributes:               pdataAttributesToOTAttributes(span.Attributes(), resource),
		Links:                    pdataLinksToOTLinks(span.Links()),
		MessageEvents:            pdataEventsToOTMessageEvents(span.Events()),
		HasRemoteParent:          false, // no field for this in pdata Span
		DroppedAttributeCount:    int(span.DroppedAttributesCount()),
		DroppedMessageEventCount: int(span.DroppedEventsCount()),
		DroppedLinkCount:         int(span.DroppedLinksCount()),
		Resource:                 r,
	}
	if !il.IsNil() {
		sd.InstrumentationLibrary = instrumentation.Library{
			Name:    il.Name(),
			Version: il.Version(),
		}
	}
	if status := span.Status(); !status.IsNil() {
		sd.StatusCode = pdataStatusCodeToGRPCCode(status.Code())
		sd.StatusMessage = status.Message()
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

func pdataAttributesToOTAttributes(attrs pdata.AttributeMap, resource pdata.Resource) []label.KeyValue {
	otAttrs := make([]label.KeyValue, 0, attrs.Len())
	appendAttrs := func(m pdata.AttributeMap) {
		m.ForEach(func(k string, v pdata.AttributeValue) {
			switch v.Type() {
			case pdata.AttributeValueSTRING:
				otAttrs = append(otAttrs, label.String(k, v.StringVal()))
			case pdata.AttributeValueBOOL:
				otAttrs = append(otAttrs, label.Bool(k, v.BoolVal()))
			case pdata.AttributeValueINT:
				otAttrs = append(otAttrs, label.Int64(k, v.IntVal()))
			case pdata.AttributeValueDOUBLE:
				otAttrs = append(otAttrs, label.Float64(k, v.DoubleVal()))
			// pdata Array, and Map cannot be converted to value.Value
			default:
				return
			}
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
		copy(sc.TraceID[:], link.TraceID().Bytes())
		copy(sc.SpanID[:], link.SpanID().Bytes())
		otLinks = append(otLinks, apitrace.Link{
			SpanContext: sc,
			Attributes:  pdataAttributesToOTAttributes(link.Attributes(), pdata.NewResource()),
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
			Name:       event.Name(),
			Attributes: pdataAttributesToOTAttributes(event.Attributes(), pdata.NewResource()),
			Time:       time.Unix(0, int64(event.Timestamp())),
		})
	}
	return otEvents
}
