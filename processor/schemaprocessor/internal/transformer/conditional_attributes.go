// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformer // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

// MetricDataPointAttributes is a conditional Transformer that acts on [pmetric.Metric]'s DataPoint's attributes.  It powers the [Metric's rename_attributes] transformation.
// [Metric's rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_attributes-transformation-2
type MetricDataPointAttributes struct {
	ConditionalAttributeChange migrate.ConditionalAttributeSet
}

func (o MetricDataPointAttributes) IsMigrator() {}

func (o MetricDataPointAttributes) Do(ss migrate.StateSelector, metric pmetric.Metric) error {
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

// SpanConditionalAttributes is a conditional Transformer that acts on [ptrace.Span]'s name.  It powers the [Span's rename_attributes] transformation.
// [Span's rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_attributes-transformation
type SpanConditionalAttributes struct {
	Migrator migrate.ConditionalAttributeSet
}

func (o SpanConditionalAttributes) IsMigrator() {}

func (o SpanConditionalAttributes) Do(ss migrate.StateSelector, span ptrace.Span) error {
	return o.Migrator.Do(ss, span.Attributes(), span.Name())
}
