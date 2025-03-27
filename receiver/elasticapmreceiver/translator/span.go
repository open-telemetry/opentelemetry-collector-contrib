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
	dest.SetKind(ConvertSpanKind(span.Kind))
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
	parseDb(span.Db, attrs)
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

func parseDb(db *modelpb.DB, attrs pcommon.Map) {
	if db == nil {
		return
	}

	PutOptionalInt(attrs, "span.db.rows_affected", db.RowsAffected)
	PutOptionalStr(attrs, "span.db.instance", &db.Instance)
	PutOptionalStr(attrs, "span.db.statement", &db.Statement)
	PutOptionalStr(attrs, "span.db.type", &db.Type)
	PutOptionalStr(attrs, "span.db.username", &db.UserName)
	PutOptionalStr(attrs, "span.db.link", &db.Link)
}
