// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type adxTrace struct {
	TraceID            string         // TraceID associated to the Trace
	SpanID             string         // SpanID associated to the Trace
	ParentID           string         // ParentID associated to the Trace
	SpanName           string         // The SpanName of the Trace
	SpanStatus         string         // The SpanStatus Code associated to the Trace
	SpanStatusMessage  string         // The SpanStatusMessage associated to the Trace
	SpanKind           string         // The SpanKind of the Trace
	StartTime          string         // The start time of the occurrence. Formatted into string as RFC3339Nano
	EndTime            string         // The end time of the occurrence. Formatted into string as RFC3339Nano
	ResourceAttributes map[string]any // JSON Resource attributes that can then be parsed.
	TraceAttributes    map[string]any // JSON attributes that can then be parsed.
	Events             []*event       // Array containing the events in a span
	Links              []*link        // Array containing the link in a span
}

type event struct {
	EventName       string
	Timestamp       string
	EventAttributes map[string]any
}

type link struct {
	TraceID            string
	SpanID             string
	TraceState         string
	SpanLinkAttributes map[string]any
}

func mapToAdxTrace(resource pcommon.Resource, scope pcommon.InstrumentationScope, spanData ptrace.Span) *adxTrace {
	traceAttrib := spanData.Attributes().AsRaw()
	clonedTraceAttrib := cloneMap(traceAttrib)
	copyMap(clonedTraceAttrib, getScopeMap(scope))

	return &adxTrace{
		TraceID:            traceutil.TraceIDToHexOrEmptyString(spanData.TraceID()),
		SpanID:             traceutil.SpanIDToHexOrEmptyString(spanData.SpanID()),
		ParentID:           traceutil.SpanIDToHexOrEmptyString(spanData.ParentSpanID()),
		SpanName:           spanData.Name(),
		SpanStatus:         traceutil.StatusCodeStr(spanData.Status().Code()),
		SpanStatusMessage:  spanData.Status().Message(),
		SpanKind:           traceutil.SpanKindStr(spanData.Kind()),
		StartTime:          spanData.StartTimestamp().AsTime().Format(time.RFC3339Nano),
		EndTime:            spanData.EndTimestamp().AsTime().Format(time.RFC3339Nano),
		ResourceAttributes: resource.Attributes().AsRaw(),
		TraceAttributes:    clonedTraceAttrib,
		Events:             getEventsData(spanData),
		Links:              getLinksData(spanData),
	}
}

func getEventsData(sd ptrace.Span) []*event {
	events := make([]*event, sd.Events().Len())

	for i := 0; i < sd.Events().Len(); i++ {
		event := &event{
			Timestamp:       sd.Events().At(i).Timestamp().AsTime().Format(time.RFC3339Nano),
			EventName:       sd.Events().At(i).Name(),
			EventAttributes: sd.Events().At(i).Attributes().AsRaw(),
		}
		events[i] = event
	}
	return events
}

func getLinksData(sd ptrace.Span) []*link {
	links := make([]*link, sd.Links().Len())
	for i := 0; i < sd.Links().Len(); i++ {
		link := &link{
			TraceID:            traceutil.TraceIDToHexOrEmptyString(sd.Links().At(i).TraceID()),
			SpanID:             traceutil.SpanIDToHexOrEmptyString(sd.Links().At(i).SpanID()),
			TraceState:         sd.Links().At(i).TraceState().AsRaw(),
			SpanLinkAttributes: sd.Links().At(i).Attributes().AsRaw(),
		}
		links[i] = link
	}
	return links
}
