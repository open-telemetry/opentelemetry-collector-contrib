// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/operator"

import (
	"errors"

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

type SpanConditionalAttributeOperator struct {
	Migrator migrate.ConditionalAttributeSet
}

func (o SpanConditionalAttributeOperator) IsMigrator() {}

func (o SpanConditionalAttributeOperator) Do(ss migrate.StateSelector, span ptrace.Span) error {
	return o.Migrator.Do(ss, span.Attributes(), span.Name())
}
