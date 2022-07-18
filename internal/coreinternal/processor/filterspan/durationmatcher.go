package filterspan

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type durationMatcher interface {
	Match(span ptrace.Span) bool
}
type SpanDurationMatcher struct {
	durationMatcher
	Operation string
	Value     int64
}

func (ma SpanDurationMatcher) Match(span ptrace.Span) bool {
	duration := span.EndTimestamp().AsTime().Unix() - span.StartTimestamp().AsTime().Unix()

	switch ma.Operation {
	case ">":
		return duration > ma.Value
	case "<":
		return duration < ma.Value
	case "<=":
		return duration <= ma.Value
	case ">=":
		return duration >= ma.Value
	}
	return false
}

func NewDurationMatcher(config filterconfig.DurationProperties) (SpanDurationMatcher, error) {
	entry := SpanDurationMatcher{
		Operation: config.Operator,
		Value:     config.Duration,
	}
	return entry, nil
}
