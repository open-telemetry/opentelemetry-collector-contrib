package operator

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

func TestAttributeOperators(t *testing.T) {
	attrChange := migrate.NewAttributeChangeSet(map[string]string{
		"service_version": "service.version",
	})
	allOperator := AllOperator{
		MetricOperator:    MetricAttributeOperator{attrChange},
		LogOperator:       LogAttributeOperator{attrChange},
		SpanOperator:      SpanAttributeOperator{attrChange},
		SpanEventOperator: SpanEventAttributeOperator{attrChange},
		ResourceMigrator:  ResourceAttributeOperator{attrChange},
	}

	tests := []struct {
		name     string
		operator func() (alias.Attributed, error)
	}{
		{
			name: "MetricOperatorExponentialHistogram",
			operator: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.ExponentialHistogram().DataPoints().At(0), allOperator.MetricOperator.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricOperatorGauge",
			operator: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Gauge().DataPoints().At(0), allOperator.MetricOperator.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricOperatorHistogram",
			operator: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Histogram().DataPoints().At(0), allOperator.MetricOperator.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricOperatorSum",
			operator: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Sum().DataPoints().At(0), allOperator.MetricOperator.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "MetricOperatorSummary",
			operator: func() (alias.Attributed, error) {
				metric := pmetric.NewMetric()
				metric.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Summary().DataPoints().At(0), allOperator.MetricOperator.Do(migrate.StateSelectorApply, metric)
			},
		},
		{
			name: "LogOperator",
			operator: func() (alias.Attributed, error) {
				log := plog.NewLogRecord()
				log.Attributes().PutStr("service_version", "1.0.0")
				return log, allOperator.LogOperator.Do(migrate.StateSelectorApply, log)
			},
		},
		{
			name: "SpanOperator",
			operator: func() (alias.Attributed, error) {
				span := ptrace.NewSpan()
				span.Attributes().PutStr("service_version", "1.0.0")
				return span, allOperator.SpanOperator.Do(migrate.StateSelectorApply, span)
			},
		},
		{
			name: "SpanEventOperator",
			operator: func() (alias.Attributed, error) {
				spanEvent := ptrace.NewSpanEvent()
				spanEvent.Attributes().PutStr("service_version", "1.0.0")
				return spanEvent, allOperator.SpanEventOperator.Do(migrate.StateSelectorApply, spanEvent)
			},
		},
		{
			name: "ResourceMigrator",
			operator: func() (alias.Attributed, error) {
				resource := pcommon.NewResource()
				resource.Attributes().PutStr("service_version", "1.0.0")
				return resource, allOperator.ResourceMigrator.Do(migrate.StateSelectorApply, resource)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := tt.operator()
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
