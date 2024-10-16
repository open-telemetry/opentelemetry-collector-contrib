// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package transformer contains various Transformers that represent a high level operation - typically a single "change" block from the schema change file.  They rely on Migrators to do the actual work of applying the change to the data.  Transformers accept and operate on a specific type of pdata (logs, metrics, etc)
package transformer // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/transformer"

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

// MetricAttributes is a Transformer that acts on [pmetric.Metric]'s DataPoint's attributes.  It is part of the [AllAttributes].
type MetricAttributes struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o MetricAttributes) IsMigrator() {}

func (o MetricAttributes) Do(ss migrate.StateSelector, metric pmetric.Metric) error {
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

// LogAttributes is a Transformer that acts on [plog.LogRecord] attributes.  It powers the [Log's rename_attributes] transformation.  It also powers the [AllAttributes].
// [Log's rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_attributes-transformation-3
type LogAttributes struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o LogAttributes) IsMigrator() {}

func (o LogAttributes) Do(ss migrate.StateSelector, log plog.LogRecord) error {
	return o.AttributeChange.Do(ss, log.Attributes())
}

// SpanAttributes is a Transformer that acts on [ptrace.Span] attributes.  It powers the [Span's rename_attributes] transformation.  It also powers the [AllAttributes].
// [Span's rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#rename_attributes-transformation
type SpanAttributes struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o SpanAttributes) IsMigrator() {}

func (o SpanAttributes) Do(ss migrate.StateSelector, span ptrace.Span) error {
	return o.AttributeChange.Do(ss, span.Attributes())
}

// SpanEventAttributes is a Transformer that acts on [ptrace.SpanEvent] attributes.  It is part of the [AllAttributes].
type SpanEventAttributes struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o SpanEventAttributes) IsMigrator() {}

func (o SpanEventAttributes) Do(ss migrate.StateSelector, spanEvent ptrace.SpanEvent) error {
	return o.AttributeChange.Do(ss, spanEvent.Attributes())
}

// ResourceAttributes is a Transformer that acts on [pcommon.Resource] attributes.  It powers the [Resource's rename_attributes] transformation.  It also powers the [AllAttributes].
// [Resource's rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#resources-section
type ResourceAttributes struct {
	AttributeChange migrate.AttributeChangeSet
}

func (o ResourceAttributes) IsMigrator() {}

func (o ResourceAttributes) Do(ss migrate.StateSelector, resource pcommon.Resource) error {
	return o.AttributeChange.Do(ss, resource.Attributes())
}

// AllAttributes is a Transformer that acts on .  It is a wrapper around the other attribute transformers.  It powers the [All rename_attributes] transformation.
// [All rename_attributes]: https://opentelemetry.io/docs/specs/otel/schemas/file_format_v1.1.0/#all-section
type AllAttributes struct {
	MetricAttributes    MetricAttributes
	LogAttributes       LogAttributes
	SpanAttributes      SpanAttributes
	SpanEventAttributes SpanEventAttributes
	ResourceAttributes  ResourceAttributes
}

func NewAllAttributesTransformer(set migrate.AttributeChangeSet) AllAttributes {
	return AllAttributes{
		MetricAttributes:    MetricAttributes{AttributeChange: set},
		LogAttributes:       LogAttributes{AttributeChange: set},
		SpanAttributes:      SpanAttributes{AttributeChange: set},
		SpanEventAttributes: SpanEventAttributes{AttributeChange: set},
		ResourceAttributes:  ResourceAttributes{AttributeChange: set},
	}
}

func (o AllAttributes) IsMigrator() {}

func (o AllAttributes) Do(ss migrate.StateSelector, data any) error {
	switch typedData := data.(type) {
	case pmetric.Metric:
		return o.MetricAttributes.Do(ss, typedData)
	case plog.LogRecord:
		return o.LogAttributes.Do(ss, typedData)
	case ptrace.Span:
		return o.SpanAttributes.Do(ss, typedData)
	case ptrace.SpanEvent:
		return o.SpanEventAttributes.Do(ss, typedData)
	case pcommon.Resource:
		return o.ResourceAttributes.Do(ss, typedData)
	default:
		return fmt.Errorf("AllAttributes can't act on %T", typedData)
	}
}
