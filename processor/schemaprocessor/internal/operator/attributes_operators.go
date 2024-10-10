// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package operator contains various Operators that represent a high level operation - typically a single "change" block from the schema change file.  They rely on Migrators to do the actual work of applying the change to the data.  Operators accept and operate on a specific type of pdata (logs, metrics, etc)
package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

// MetricDataPointAttributeOperator is an Operator that acts on Metric DataPoints' attributes
type MetricDataPointAttributeOperator struct {
	ConditionalAttributeChange migrate.ConditionalAttributeSet
}

func (o MetricDataPointAttributeOperator) IsMigrator() {}

func (o MetricDataPointAttributeOperator) Do(ss migrate.StateSelector, metric pmetric.Metric) error {
	// todo(ankit) handle MetricTypeEmpty
	var datam alias.Attributed
	switch metric.Type() {
	case pmetric.MetricTypeExponentialHistogram:
		for dp := 0; dp < metric.ExponentialHistogram().DataPoints().Len(); dp++ {
			datam = metric.ExponentialHistogram().DataPoints().At(dp)
			if err := o.ConditionalAttributeChange.Do(ss, datam.Attributes(), metric.Name()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeHistogram:
		for dp := 0; dp < metric.Histogram().DataPoints().Len(); dp++ {
			datam = metric.Histogram().DataPoints().At(dp)
			if err := o.ConditionalAttributeChange.Do(ss, datam.Attributes(), metric.Name()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeGauge:
		for dp := 0; dp < metric.Gauge().DataPoints().Len(); dp++ {
			datam = metric.Gauge().DataPoints().At(dp)
			if err := o.ConditionalAttributeChange.Do(ss, datam.Attributes(), metric.Name()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSum:
		for dp := 0; dp < metric.Sum().DataPoints().Len(); dp++ {
			datam = metric.Sum().DataPoints().At(dp)
			if err := o.ConditionalAttributeChange.Do(ss, datam.Attributes(), metric.Name()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSummary:
		for dp := 0; dp < metric.Summary().DataPoints().Len(); dp++ {
			datam = metric.Summary().DataPoints().At(dp)
			if err := o.ConditionalAttributeChange.Do(ss, datam.Attributes(), metric.Name()); err != nil {
				return err
			}
		}
	default:
		return errors.New("unsupported metric type")
	}

	return nil
}

// LogAttributeOperator is an Operator that acts on LogRecords' attributes
type LogAttributeOperator struct {
	AttributeChange migrate.AttributeChangeSet
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
	AttributeChange migrate.AttributeChangeSet
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
