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
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	apitrace "go.opentelemetry.io/otel/trace"
)

func pdataResourceSpansToOTSpanData(rs pdata.ResourceSpans) []*export.SpanSnapshot {
	resource := rs.Resource()
	var sds []*export.SpanSnapshot
	ilss := rs.InstrumentationLibrarySpans()
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		spans := ils.Spans()
		for j := 0; j < spans.Len(); j++ {
			sd := pdataSpanToOTSpanData(spans.At(j), resource, ils.InstrumentationLibrary())
			sds = append(sds, sd)
		}
	}

	return sds
}

func pdataSpanToOTSpanData(
	span pdata.Span,
	resource pdata.Resource,
	il pdata.InstrumentationLibrary,
) *export.SpanSnapshot {
	sc := apitrace.SpanContext{}
	sc.TraceID = span.TraceID().Bytes()
	sc.SpanID = span.SpanID().Bytes()
	startTime := time.Unix(0, int64(span.StartTime()))
	endTime := time.Unix(0, int64(span.EndTime()))
	// TODO: Decide if ignoring the error is fine.
	r, _ := sdkresource.New(
		context.Background(),
		sdkresource.WithAttributes(pdataAttributesToOTAttributes(pdata.NewAttributeMap(), resource)...),
	)

	sd := &export.SpanSnapshot{
		SpanContext:              sc,
		ParentSpanID:             span.ParentSpanID().Bytes(),
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
	sd.InstrumentationLibrary = instrumentation.Library{
		Name:    il.Name(),
		Version: il.Version(),
	}
	status := span.Status()
	sd.StatusCode = pdataStatusCodeToOTCode(status.Code())
	sd.StatusMessage = status.Message()

	return sd
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

func pdataStatusCodeToOTCode(c pdata.StatusCode) codes.Code {
	switch c {
	case pdata.StatusCodeOk:
		return codes.Ok
	case pdata.StatusCodeError:
		return codes.Error
	default:
		return codes.Unset
	}
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
	appendAttrs(resource.Attributes())
	appendAttrs(attrs)
	return otAttrs
}

func pdataLinksToOTLinks(links pdata.SpanLinkSlice) []apitrace.Link {
	size := links.Len()
	otLinks := make([]apitrace.Link, 0, size)
	for i := 0; i < size; i++ {
		link := links.At(i)
		sc := apitrace.SpanContext{}
		sc.TraceID = link.TraceID().Bytes()
		sc.SpanID = link.SpanID().Bytes()
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
		otEvents = append(otEvents, export.Event{
			Name:       event.Name(),
			Attributes: pdataAttributesToOTAttributes(event.Attributes(), pdata.NewResource()),
			Time:       time.Unix(0, int64(event.Timestamp())),
		})
	}
	return otEvents
}
