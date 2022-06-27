package azuredataexplorerexporter

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type AdxTrace struct {
	TraceId            string                 //TraceId associated to the Trace
	SpanId             string                 //SpanId associated to the Trace
	ParentId           string                 //ParentId associated to the Trace
	SpanName           string                 //The SpanName of the Trace
	SpanStatus         string                 //The SpanStatus associated to the Trace
	SpanKind           string                 //The SpanKind of the Trace
	StartTime          string                 //The start time of the occurance. Formatted into string as RFC3339
	EndTime            string                 //The end time of the occurance. Formatted into string as RFC3339
	ResourceAttributes map[string]interface{} //JSON Resource attributes that can then be parsed.
	TraceAttributes    map[string]interface{} //JSON attributes that can then be parsed.
	Events             []*Event               //Array containing the events in a span
	Links              []*Link                //Array containing the link in a span
}

type Event struct {
	EventName       string
	Timestamp       string
	EventAttributes map[string]interface{}
}

type Link struct {
	TraceId            string
	SpanId             string
	TraceState         string
	SpanLinkAttributes map[string]interface{}
}

func mapToAdxTrace(resource pcommon.Resource, scope pcommon.InstrumentationScope, spanData ptrace.Span, logger *zap.Logger) *AdxTrace {

	traceattrib := spanData.Attributes().AsRaw()
	clonedtraceattrib := cloneMap(traceattrib)
	copyAttributes(clonedtraceattrib, getScopeMap(scope))

	return &AdxTrace{
		TraceId:            spanData.TraceID().HexString(),
		SpanId:             spanData.SpanID().HexString(),
		ParentId:           spanData.ParentSpanID().HexString(),
		SpanName:           spanData.Name(),
		SpanStatus:         spanData.Status().Code().String(),
		SpanKind:           spanData.Kind().String(),
		StartTime:          spanData.StartTimestamp().AsTime().Format(time.RFC3339),
		EndTime:            spanData.EndTimestamp().AsTime().Format(time.RFC3339),
		ResourceAttributes: resource.Attributes().AsRaw(),
		TraceAttributes:    clonedtraceattrib,
		Events:             getEventsData(spanData),
		Links:              getLinksData(spanData),
	}
}

func getEventsData(sd ptrace.Span) []*Event {
	events := make([]*Event, sd.Events().Len())

	for i := 0; i < sd.Events().Len(); i++ {
		event := &Event{
			Timestamp:       sd.Events().At(i).Timestamp().AsTime().Format(time.RFC3339),
			EventName:       sd.Events().At(i).Name(),
			EventAttributes: sd.Events().At(i).Attributes().AsRaw(),
		}
		events[i] = event
	}
	return events
}

func getLinksData(sd ptrace.Span) []*Link {
	links := make([]*Link, sd.Links().Len())
	for i := 0; i < sd.Links().Len(); i++ {
		link := &Link{
			TraceId:            sd.Links().At(i).TraceID().HexString(),
			SpanId:             sd.Links().At(i).SpanID().HexString(),
			TraceState:         string(sd.Links().At(i).TraceState()),
			SpanLinkAttributes: sd.Links().At(i).Attributes().AsRaw(),
		}
		links[i] = link
	}
	return links
}
