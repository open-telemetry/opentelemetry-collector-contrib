package translator

import (
	"github.com/elastic/apm-data/model/modelpb"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func parseTrace(trace *modelpb.Trace, span ptrace.Span) {
	if trace == nil {
		return
	}

	span.SetTraceID(ConvertTraceId(trace.Id))
}
