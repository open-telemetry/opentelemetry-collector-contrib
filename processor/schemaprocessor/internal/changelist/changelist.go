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

func (c ChangeList) Apply(signal any) error {
	for i := 0; i < len(c.Migrators); i++ {
		migrator := c.Migrators[i]
		switch thisMigrator := migrator.(type) {
		case *migrate.AttributeChangeSet:
			switch attributeSignal := signal.(type) {
			case alias.Attributed:
				if err := thisMigrator.Apply(attributeSignal.Attributes()); err != nil {
					return err
				}
			default:
				return errors.New("unsupported signal type")
			}
		case *operator.SpanEventConditionalAttributeOperator:
			if span, ok := signal.(ptrace.Span); ok {
				if err := thisMigrator.Apply(span); err != nil {
					return err
				}
			} else {
				return errors.New("unsupported signal type")
			}
		case *migrate.MultiConditionalAttributeSet:
			switch typedSignal := signal.(type) {
			case ptrace.Span:
				if err := thisMigrator.Apply(typedSignal.Attributes(), map[string]string{"span.name": typedSignal.Name()}); err != nil {
					return err
				}
			//case DataPoint:
			//	if err := thisMigrator.Apply(typedSignal.Attributes(), map[string][]string{"metric.name": {typedsignal.name()}}); err != nil {
			//		return err
			//	}
			default:
				return errors.New("unsupported signal type for Conditional Attribute Change")
			}
		case *migrate.SignalNameChange:
			switch namedSignal := signal.(type) {
			case alias.NamedSignal:
				thisMigrator.Apply(namedSignal)
			}
		default:
			return errors.New("unsupported migrator type")
		}
	}
		return nil
}


func (c ChangeList) Rollback(signal any) error {
	for i := 0; i < len(c.Migrators); i++ {
		migrator := c.Migrators[len(c.Migrators) - 1 -i]
		switch thisMigrator := migrator.(type) {
		case *migrate.AttributeChangeSet:
			switch attributeSignal := signal.(type) {
			case alias.Attributed:
				if err := thisMigrator.Rollback(attributeSignal.Attributes()); err != nil {
					return err
				}
			default:
				return errors.New("unsupported signal type")
			}
		case *operator.SpanEventConditionalAttributeOperator:
			if span, ok := signal.(ptrace.Span); ok {
				if err := thisMigrator.Rollback(span); err != nil {
					return err
				}
			} else {
				return errors.New("unsupported signal type")
			}
		case *migrate.MultiConditionalAttributeSet:
			switch typedSignal := signal.(type) {
			case ptrace.Span:
				if err := thisMigrator.Rollback(typedSignal.Attributes(), map[string]string{"span.name": typedSignal.Name()}); err != nil {
					return err
				}
			//case DataPoint:
			//	if err := thisMigrator.Rollback(typedSignal.Attributes(), map[string][]string{"metric.name": {typedsignal.name()}}); err != nil {
			//		return err
			//	}
			default:
				return errors.New("unsupported signal type for Conditional Attribute Change")
			}
		case *migrate.SignalNameChange:
			switch namedSignal := signal.(type) {
			case alias.NamedSignal:
				thisMigrator.Rollback(namedSignal)
			}
		default:
			return errors.New("unsupported migrator type")
		}
	}
	return nil
}