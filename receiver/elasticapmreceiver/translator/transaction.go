package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func ConvertTransaction(event *modelpb.APMEvent, dest ptrace.Span) {
	if event == nil {
		return
	}

	attrs := dest.Attributes()
	parseBaseEvent(event, attrs)

	if event.Transaction == nil {
		return
	}

	transaction := event.Transaction

	parseTrace(event.Trace, dest)
	dest.SetSpanID(ConvertSpanId(transaction.Id))
	if event.GetParentId() != "" {
		dest.SetParentSpanID(ConvertSpanId(event.ParentId))
	}
	dest.SetName(transaction.Name)
	dest.SetKind(ConvertSpanKind(transaction.Type))
	start, end := GetStartAndEndTimestamps(event.Timestamp, event.GetEvent().GetDuration())
	if start != nil && end != nil {
		dest.SetStartTimestamp(*start)
		dest.SetEndTimestamp(*end)
	}

	parseSpanCount(transaction.SpanCount, attrs)
	parseMessage(transaction.Message, attrs)
	parseUserExperience(transaction.UserExperience, attrs)
	parseMarks(transaction.Marks, attrs)

	PutOptionalStr(attrs, "transaction.result", &transaction.Result)
	PutOptionalFloat(attrs, "transaction.representative_count", &transaction.RepresentativeCount)
	PutOptionalBool(attrs, "transaction.sampled", &transaction.Sampled)
	PutOptionalStr(attrs, "transaction.result", &transaction.Result)

	// TODO: transaction.Custom
	// TODO: transaction.DroppedSpansStats
	// TODO: transaction.DurationSummary
}

func parseSpanCount(spanCount *modelpb.SpanCount, attrs pcommon.Map) {
	if spanCount == nil {
		return
	}

	PutOptionalInt(attrs, "transaction.span_count.dropped", spanCount.Dropped)
	PutOptionalInt(attrs, "transaction.span_count.started", spanCount.Started)
}

func parseUserExperience(ux *modelpb.UserExperience, attrs pcommon.Map) {
	if ux == nil {
		return
	}

	PutOptionalFloat(attrs, "transaction.user_experience.cls", &ux.CumulativeLayoutShift)
	PutOptionalFloat(attrs, "transaction.user_experience.fid", &ux.FirstInputDelay)
	PutOptionalFloat(attrs, "transaction.user_experience.tbt", &ux.TotalBlockingTime)
	parseLongTask(ux.LongTask, attrs)
}

func parseLongTask(longtask *modelpb.LongtaskMetrics, attrs pcommon.Map) {
	if longtask == nil {
		return
	}

	PutOptionalInt(attrs, "transaction.longtask.count", &longtask.Count)
	PutOptionalFloat(attrs, "transaction.longtask.sum", &longtask.Sum)
	PutOptionalFloat(attrs, "transaction.longtask.max", &longtask.Max)
}

func parseMarks(marks map[string]*modelpb.TransactionMark, attrs pcommon.Map) {
	if marks == nil {
		return
	}

	m := attrs.PutEmptyMap("transaction.marks")
	for markName, mark := range marks {
		mm := m.PutEmptyMap(markName)
		for k, v := range mark.Measurements {
			mm.PutDouble(k, v)
		}
	}
}
