package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func parseEvent(event *modelpb.Event, attrs pcommon.Map) {
	if event == nil {
		return
	}

	PutOptionalStr(attrs, "event.outcome", &event.Outcome)
	PutOptionalStr(attrs, "event.action", &event.Action)
	PutOptionalStr(attrs, "event.dataset", &event.Dataset)
	PutOptionalStr(attrs, "event.kind", &event.Kind)
	PutOptionalStr(attrs, "event.category", &event.Category)
	PutOptionalStr(attrs, "event.type", &event.Type)
	parseSummaryMetric(event.SuccessCount, attrs)
	PutOptionalInt(attrs, "event.severity", &event.Severity)
	// TODO: event.Received
	// TODO: event.Duration
}

func parseSummaryMetric(metric *modelpb.SummaryMetric, attrs pcommon.Map) {
	if metric == nil {
		return
	}

	PutOptionalInt(attrs, "event.success_count.count", &metric.Count)
	PutOptionalFloat(attrs, "event.success_count.sum", &metric.Sum)
}
