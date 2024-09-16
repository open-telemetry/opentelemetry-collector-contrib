// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package operator contains various Operators that represent a high level operation - typically a single "change" block from the schema change file.  They rely on Migrators to do the actual work of applying the change to the data.  Operators accept and operate on a specific type of pdata (logs, metrics, etc)
package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

// MetricDataPointAttributeOperator is a conditional Operator that acts on Metric DataPoints' attributes
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

// MetricAttributeOperator is an Operator that acts on Metric DataPoints' attributes
type MetricAttributeOperator struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o MetricAttributeOperator) IsMigrator() {}

func (o MetricAttributeOperator) Do(ss migrate.StateSelector, metric pmetric.Metric) error {
	// todo(ankit) handle MetricTypeEmpty
	var datam alias.Attributed
	switch metric.Type() {
	case pmetric.MetricTypeExponentialHistogram:
		for dp := 0; dp < metric.ExponentialHistogram().DataPoints().Len(); dp++ {
			datam = metric.ExponentialHistogram().DataPoints().At(dp)
			if err := o.AttributeChange.Do(ss, datam.Attributes()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeHistogram:
		for dp := 0; dp < metric.Histogram().DataPoints().Len(); dp++ {
			datam = metric.Histogram().DataPoints().At(dp)
			if err := o.AttributeChange.Do(ss, datam.Attributes()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeGauge:
		for dp := 0; dp < metric.Gauge().DataPoints().Len(); dp++ {
			datam = metric.Gauge().DataPoints().At(dp)
			if err := o.AttributeChange.Do(ss, datam.Attributes()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSum:
		for dp := 0; dp < metric.Sum().DataPoints().Len(); dp++ {
			datam = metric.Sum().DataPoints().At(dp)
			if err := o.AttributeChange.Do(ss, datam.Attributes()); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSummary:
		for dp := 0; dp < metric.Summary().DataPoints().Len(); dp++ {
			datam = metric.Summary().DataPoints().At(dp)
			if err := o.AttributeChange.Do(ss, datam.Attributes()); err != nil {
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

func (o LogAttributeOperator) IsMigrator() {}

func (o LogAttributeOperator) Do(ss migrate.StateSelector, log plog.LogRecord) error {
	return o.AttributeChange.Do(ss, log.Attributes())
}

type SpanAttributeOperator struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o SpanAttributeOperator) IsMigrator() {}

func (o SpanAttributeOperator) Do(ss migrate.StateSelector, span ptrace.Span) error {
	return o.AttributeChange.Do(ss, span.Attributes())
}

type SpanEventAttributeOperator struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o SpanEventAttributeOperator) IsMigrator() {}

func (o SpanEventAttributeOperator) Do(ss migrate.StateSelector, spanEvent ptrace.SpanEvent) error {
	return o.AttributeChange.Do(ss, spanEvent.Attributes())
}

type AllOperator struct {
	MetricOperator    MetricAttributeOperator
	LogOperator       LogAttributeOperator
	SpanOperator      SpanAttributeOperator
	SpanEventOperator SpanEventAttributeOperator
	ResourceMigrator  migrate.AttributeChangeSet
}

func NewAllOperator(set migrate.AttributeChangeSet) AllOperator {
	return AllOperator{
		MetricOperator:    MetricAttributeOperator{AttributeChange: set},
		LogOperator:       LogAttributeOperator{AttributeChange: set},
		SpanOperator:      SpanAttributeOperator{AttributeChange: set},
		SpanEventOperator: SpanEventAttributeOperator{AttributeChange: set},
		ResourceMigrator:  set,
	}
}

func (o AllOperator) IsMigrator() {}

func (o AllOperator) Do(ss migrate.StateSelector, data any) error {
	switch typedData := data.(type) {
	case pmetric.Metric:
		return o.MetricOperator.Do(ss, typedData)
	case plog.LogRecord:
		return o.LogOperator.Do(ss, typedData)
	case ptrace.Span:
		return o.SpanOperator.Do(ss, typedData)
	case ptrace.SpanEvent:
		return o.SpanEventOperator.Do(ss, typedData)
	case pcommon.Resource:
		return o.ResourceMigrator.Do(ss, typedData.Attributes())
	default:
		return fmt.Errorf("AllOperator can't act on %T", typedData)
	}
}
