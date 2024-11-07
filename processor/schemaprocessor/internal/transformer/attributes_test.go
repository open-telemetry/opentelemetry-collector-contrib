// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

func TestAttributeTransformers(t *testing.T) {
	attrChange := migrate.NewAttributeChangeSet(map[string]string{
		"service_version": "service.version",
	})
	allTransformer := AllAttributes{
		MetricAttributes:    MetricAttributes{attrChange},
		LogAttributes:       LogAttributes{attrChange},
		SpanAttributes:      SpanAttributes{attrChange},
		SpanEventAttributes: SpanEventAttributes{attrChange},
		ResourceAttributes:  ResourceAttributes{attrChange},
	}

	tests := []struct {
		name        string
		transformer func() (alias.Attributed, error)
	}{
		{
			name: "MetricTransformerExponentialHistogram",
			transformer: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.ExponentialHistogram().DataPoints().At(0), allTransformer.MetricAttributes.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricTransformerGauge",
			transformer: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Gauge().DataPoints().At(0), allTransformer.MetricAttributes.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricTransformerHistogram",
			transformer: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Histogram().DataPoints().At(0), allTransformer.MetricAttributes.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricTransformerSum",
			transformer: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Sum().DataPoints().At(0), allTransformer.MetricAttributes.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricTransformerSummary",
			transformer: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Summary().DataPoints().At(0), allTransformer.MetricAttributes.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "LogAttributes",
			transformer: func() (alias.Attributed, error) {
				log := plog.NewLogRecord()
				log.Attributes().PutStr("service_version", "1.0.0")
				return log, allTransformer.LogAttributes.Do(migrate.StateSelectorApply, log)
			},
		},
		{
			name: "SpanAttributes",
			transformer: func() (alias.Attributed, error) {
				span := ptrace.NewSpan()
				span.Attributes().PutStr("service_version", "1.0.0")
				return span, allTransformer.SpanAttributes.Do(migrate.StateSelectorApply, span)
			},
		},
		{
			name: "SpanEventAttributes",
			transformer: func() (alias.Attributed, error) {
				spanEvent := ptrace.NewSpanEvent()
				spanEvent.Attributes().PutStr("service_version", "1.0.0")
				return spanEvent, allTransformer.SpanEventAttributes.Do(migrate.StateSelectorApply, spanEvent)
			},
		},
		{
			name: "ResourceAttributes",
			transformer: func() (alias.Attributed, error) {
				resource := pcommon.NewResource()
				resource.Attributes().PutStr("service_version", "1.0.0")
				return resource, allTransformer.ResourceAttributes.Do(migrate.StateSelectorApply, resource)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := tt.transformer()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			attrs := item.Attributes()
			val, ok := attrs.Get("service.version")
			assert.True(t, ok)
			assert.Equal(t, "1.0.0", val.Str())
		})
	}
}
