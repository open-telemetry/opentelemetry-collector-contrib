package changelist

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"
)

type ChangeList struct {
	Migrators []migrate.Migrator
}

func (c ChangeList) Do(ss migrate.StateSelector, signal any) error {
	for i := 0; i < len(c.Migrators); i++ {
		var migrator migrate.Migrator
		if ss == migrate.StateSelectorApply {
			migrator = c.Migrators[i]
		} else {
			migrator = c.Migrators[len(c.Migrators) - 1 -i]
		}
		switch thisMigrator := migrator.(type) {
		case *migrate.AttributeChangeSet:
			switch attributeSignal := signal.(type) {
			case alias.Attributed:
				if err := thisMigrator.Do(ss, attributeSignal.Attributes()); err != nil {
					return err
				}
			default:
				return errors.New("unsupported signal type")
			}
		case *operator.SpanEventConditionalAttributeOperator:
			if span, ok := signal.(ptrace.Span); ok {
				if err := thisMigrator.Do(ss, span); err != nil {
					return err
				}
			} else {
				return errors.New("unsupported signal type")
			}
		case *migrate.SignalNameChange:
			switch namedSignal := signal.(type) {
			case ptrace.Span:
				for e := 0; e < namedSignal.Events().Len(); e++ {
					event := namedSignal.Events().At(e)
					thisMigrator.Do(ss, event)
				}
			case alias.NamedSignal:
				thisMigrator.Do(ss, namedSignal)
			}
		case *migrate.ConditionalAttributeSet:
			if span, ok := signal.(ptrace.Span); ok {
				if err := thisMigrator.Do(ss, span.Attributes(), span.Name()); err != nil {
					return err
				}
			} else {
				return errors.New("unsupported signal type")
			}

		default:
			return errors.New("unsupported migrator type")
		}
	}
		return nil
}

func (c ChangeList) Apply(signal any) error {
	return c.Do(migrate.StateSelectorApply, signal)
}

func (c ChangeList) Rollback(signal any) error {
	return c.Do(migrate.StateSelectorRollback, signal)
}