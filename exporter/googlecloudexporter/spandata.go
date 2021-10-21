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
package googlecloudexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	apitrace "go.opentelemetry.io/otel/trace"
)

func pdataResourceSpansToOTSpanData(rs pdata.ResourceSpans) []sdktrace.ReadOnlySpan {
	resource := rs.Resource()
	var sds []sdktrace.ReadOnlySpan
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
) spanSnapshot {
	sc := apitrace.SpanContextConfig{
		TraceID: span.TraceID().Bytes(),
		SpanID:  span.SpanID().Bytes(),
	}
	parentSc := apitrace.SpanContextConfig{
		TraceID: span.TraceID().Bytes(),
		SpanID:  span.ParentSpanID().Bytes(),
	}
	startTime := time.Unix(0, int64(span.StartTimestamp()))
	endTime := time.Unix(0, int64(span.EndTimestamp()))
	// TODO: Decide if ignoring the error is fine.
	r, _ := sdkresource.New(
		context.Background(),
		sdkresource.WithAttributes(pdataAttributesToOTAttributes(pdata.NewAttributeMap(), resource)...),
	)

	status := span.Status()
	return spanSnapshot{
		spanContext:          apitrace.NewSpanContext(sc),
		parent:               apitrace.NewSpanContext(parentSc),
		spanKind:             pdataSpanKindToOTSpanKind(span.Kind()),
		startTime:            startTime,
		endTime:              endTime,
		name:                 span.Name(),
		attributes:           pdataAttributesToOTAttributes(span.Attributes(), resource),
		links:                pdataLinksToOTLinks(span.Links()),
		events:               pdataEventsToOTMessageEvents(span.Events()),
		droppedAttributes:    int(span.DroppedAttributesCount()),
		droppedMessageEvents: int(span.DroppedEventsCount()),
		droppedLinks:         int(span.DroppedLinksCount()),
		resource:             r,
		instrumentationLibrary: instrumentation.Library{
			Name:    il.Name(),
			Version: il.Version(),
		},
		status: sdktrace.Status{
			Code:        pdataStatusCodeToOTCode(status.Code()),
			Description: status.Message(),
		},
	}
}

func pdataSpanKindToOTSpanKind(k pdata.SpanKind) apitrace.SpanKind {
	switch k {
	case pdata.SpanKindUnspecified:
		return apitrace.SpanKindInternal
	case pdata.SpanKindInternal:
		return apitrace.SpanKindInternal
	case pdata.SpanKindServer:
		return apitrace.SpanKindServer
	case pdata.SpanKindClient:
		return apitrace.SpanKindClient
	case pdata.SpanKindProducer:
		return apitrace.SpanKindProducer
	case pdata.SpanKindConsumer:
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

func pdataAttributesToOTAttributes(attrs pdata.AttributeMap, resource pdata.Resource) []attribute.KeyValue {
	otAttrs := make([]attribute.KeyValue, 0, attrs.Len())
	appendAttrs := func(m pdata.AttributeMap) {
		m.Range(func(k string, v pdata.AttributeValue) bool {
			switch v.Type() {
			case pdata.AttributeValueTypeString:
				otAttrs = append(otAttrs, attribute.String(k, v.StringVal()))
			case pdata.AttributeValueTypeBool:
				otAttrs = append(otAttrs, attribute.Bool(k, v.BoolVal()))
			case pdata.AttributeValueTypeInt:
				otAttrs = append(otAttrs, attribute.Int64(k, v.IntVal()))
			case pdata.AttributeValueTypeDouble:
				otAttrs = append(otAttrs, attribute.Float64(k, v.DoubleVal()))
			}
			return true
		})
	}
	appendAttrs(resource.Attributes())
	appendAttrs(attrs)
	return otAttrs
}

func pdataLinksToOTLinks(links pdata.SpanLinkSlice) []sdktrace.Link {
	size := links.Len()
	otLinks := make([]sdktrace.Link, 0, size)
	for i := 0; i < size; i++ {
		link := links.At(i)
		sc := apitrace.SpanContextConfig{}
		sc.TraceID = link.TraceID().Bytes()
		sc.SpanID = link.SpanID().Bytes()
		otLinks = append(otLinks, sdktrace.Link{
			SpanContext: apitrace.NewSpanContext(sc),
			Attributes:  pdataAttributesToOTAttributes(link.Attributes(), pdata.NewResource()),
		})
	}
	return otLinks
}

func pdataEventsToOTMessageEvents(events pdata.SpanEventSlice) []sdktrace.Event {
	size := events.Len()
	otEvents := make([]sdktrace.Event, 0, size)
	for i := 0; i < size; i++ {
		event := events.At(i)
		otEvents = append(otEvents, sdktrace.Event{
			Name:       event.Name(),
			Attributes: pdataAttributesToOTAttributes(event.Attributes(), pdata.NewResource()),
			Time:       time.Unix(0, int64(event.Timestamp())),
		})
	}
	return otEvents
}
