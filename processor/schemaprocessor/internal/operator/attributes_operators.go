package operator

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

type LogAttributeOperator struct {
	AttributeChange *migrate.AttributeChangeSet
}

func (o LogAttributeOperator) Apply(data any) error {
	logs, ok := data.(plog.ScopeLogs)
	if !ok {
		return errors.New("invalid data type for LogAttributeOperator")
	}
	for l := 0; l < logs.LogRecords().Len(); l++ {
		log := logs.LogRecords().At(l)
		if err := o.AttributeChange.Apply(log.Attributes()); err != nil {
			return err
		}
	}
	return nil
}


func (o LogAttributeOperator) Rollback(data any) error {
	logs, ok := data.(plog.ScopeLogs)
	if !ok {
		return errors.New("invalid data type for LogAttributeOperator")
	}

	for l := 0; l < logs.LogRecords().Len(); l++ {
		log := logs.LogRecords().At(l)
		if err := o.AttributeChange.Rollback(log.Attributes()); err != nil {
			return err
		}
	}
	return nil
}

type SpanAttributeOperator struct {
	AttributeChange *migrate.AttributeChangeSet
}

func (o SpanAttributeOperator) Apply(data any) error {
	traces, ok := data.(ptrace.ScopeSpans)
	if !ok {
		return errors.New("invalid data type for SpanAttributeOperator")
	}
	for l := 0; l < traces.Spans().Len(); l++ {
		span := traces.Spans().At(l)
		if err := o.AttributeChange.Apply(span.Attributes()); err != nil {
			return err
		}
	}
	return nil
}


func (o SpanAttributeOperator) Rollback(data any) error {
	traces, ok := data.(ptrace.ScopeSpans)
	if !ok {
		return errors.New("invalid data type for SpanAttributeOperator")
	}
	for l := 0; l < traces.Spans().Len(); l++ {
		span := traces.Spans().At(l)
		if err := o.AttributeChange.Rollback(span.Attributes()); err != nil {
			return err
		}
	}
	return nil
}

type SpanEventAttributeOperator struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o SpanEventAttributeOperator) Apply(span ptrace.Span) error {
	for l := 0; l < span.Events().Len(); l++ {
		span := span.Events().At(l)
		if err := o.AttributeChange.Apply(span.Attributes()); err != nil {
			return err
		}
	}
	return nil
}

func (o SpanEventAttributeOperator) Rollback(span ptrace.Span) error {
	for l := 0; l < span.Events().Len(); l++ {
		span := span.Events().At(l)
		if err := o.AttributeChange.Rollback(span.Attributes()); err != nil {
			return err
		}
	}
	return nil
}