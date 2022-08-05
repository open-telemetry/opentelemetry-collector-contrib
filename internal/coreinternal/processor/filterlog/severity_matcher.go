package filterlog

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// severtiyNumberMatcher is a Matcher that matches if the input log record has a severity higher than
// the minSeverityNumber.
type severityNumberMatcher struct {
	minSeverityNumber plog.SeverityNumber
}

func newSeverityNumberMatcher(sn plog.SeverityNumber) severityNumberMatcher {
	return severityNumberMatcher{
		minSeverityNumber: sn,
	}
}

func (snm severityNumberMatcher) MatchLogRecord(lr plog.LogRecord, _ pcommon.Resource, _ pcommon.InstrumentationScope) bool {
	// We explicitly do not match UNDEFINED severity.
	if lr.SeverityNumber() == plog.SeverityNumberUNDEFINED {
		return false
	}

	// If the log records severity is greater than or equal to the desired severity, it matches
	if lr.SeverityNumber() >= snm.minSeverityNumber {
		return true
	}

	return false
}
