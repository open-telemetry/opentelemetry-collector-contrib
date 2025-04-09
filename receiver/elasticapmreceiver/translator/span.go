package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func ConvertSpan(event *modelpb.APMEvent, dest ptrace.Span) {
	if event == nil {
		return
	}

	attrs := dest.Attributes()
	parseBaseEvent(event, attrs)

	if event.Span == nil {
		return
	}

	span := event.Span

	parseTrace(event.Trace, dest)
	dest.SetSpanID(ConvertSpanId(span.Id))
	if event.GetParentId() != "" {
		dest.SetParentSpanID(ConvertSpanId(event.ParentId))
	}
	dest.SetName(span.Name)
	dest.SetKind(ConvertSpanKind(event))
	start, end := GetStartAndEndTimestamps(event.Timestamp, event.Event.Duration)
	if start != nil && end != nil {
		dest.SetStartTimestamp(*start)
		dest.SetEndTimestamp(*end)
	}

	PutOptionalStr(attrs, "span.type", &span.Type)
	PutOptionalStr(attrs, "span.subtype", &span.Subtype)
	PutOptionalStr(attrs, "span.action", &span.Action)
	PutOptionalBool(attrs, "span.sync", span.Sync)
	PutOptionalFloat(attrs, "span.representative_count", &span.RepresentativeCount)
	parseDb(event, attrs)
	// TODO: span.Stacktrace
	// TODO: span.Links
	// TODO: span.SelfTime
}

func parseComposite(composite *modelpb.Composite, attrs pcommon.Map) {
	if composite == nil {
		return
	}

	attrs.PutStr("span.composite.compression_strategy", composite.GetCompressionStrategy().String())
	PutOptionalInt(attrs, "span.composite.count", &composite.Count)
	PutOptionalFloat(attrs, "span.composite.sum", &composite.Sum)
}

func parseDestinationService(service *modelpb.DestinationService, attrs pcommon.Map) {
	if service == nil {
		return
	}

	PutOptionalStr(attrs, "span.destination_service.type", &service.Type)
	PutOptionalStr(attrs, "span.destination_service.name", &service.Name)
	PutOptionalStr(attrs, "span.destination_service.resource", &service.Resource)
	// TODO: service.ResponseTime
}

func parseDb(event *modelpb.APMEvent, attrs pcommon.Map) {
	db := event.Span.Db
	if db == nil {
		return
	}

	PutOptionalStr(attrs, "span.db.instance", &db.Instance)
	PutOptionalStr(attrs, "db.query.text", &db.Statement)
	PutOptionalStr(attrs, "db.username", &db.UserName)
	PutOptionalStr(attrs, "db.system.name", &event.Span.DestinationService.Name)
}
