package operator

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

type SpanEventSignalNameChange struct {
	SignalNameChange migrate.SignalNameChange
}

func (c SpanEventSignalNameChange) IsMigrator() {}

func (c SpanEventSignalNameChange) Do(ss migrate.StateSelector, span ptrace.Span) error {
	for e := 0; e < span.Events().Len(); e++ {
		event := span.Events().At(e)
		c.SignalNameChange.Do(ss, event)
	}
	return nil
}