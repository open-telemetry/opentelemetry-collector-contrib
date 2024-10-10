package operator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

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
	// test for each type of operator within an AllOperator
	t.Run("MetricOperatorSum", func(t *testing.T) {
		// todo test for each type of metric
		metric := pmetric.NewMetric()
		metric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")

		if err := allOperator.MetricOperator.Do(migrate.StateSelectorApply, metric); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		val, ok := metric.Sum().DataPoints().At(0).Attributes().Get("service.version")
		assert.True(t, ok)
		assert.Equal(t, "1.0.0", val.Str())
	})

	t.Run("LogOperator", func(t *testing.T) {
		log := plog.NewLogRecord()
		log.Attributes().PutStr("service_version", "1.0.0")
		if err := allOperator.LogOperator.Do(migrate.StateSelectorApply, log); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		val, ok := log.Attributes().Get("service.version")
		assert.True(t, ok)
		assert.Equal(t, "1.0.0", val.Str())
	})
	t.Run("SpanOperator", func(t *testing.T) {
		span := ptrace.NewSpan()
		span.Attributes().PutStr("service_version", "1.0.0")
		if err := allOperator.SpanOperator.Do(migrate.StateSelectorApply, span); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		val, ok := span.Attributes().Get("service.version")
		assert.True(t, ok)
		assert.Equal(t, "1.0.0", val.Str())
	})
	t.Run("SpanEventOperator", func(t *testing.T) {
		spanEvent := ptrace.NewSpanEvent()
		spanEvent.Attributes().PutStr("service_version", "1.0.0")
		if err := allOperator.SpanEventOperator.Do(migrate.StateSelectorApply, spanEvent); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		val, ok := spanEvent.Attributes().Get("service.version")
		assert.True(t, ok)
		assert.Equal(t, "1.0.0", val.Str())
	})
	t.Run("ResourceMigrator", func(t *testing.T) {
		resource := pcommon.NewResource()
		resource.Attributes().PutStr("service_version", "1.0.0")
		if err := allOperator.ResourceMigrator.Do(migrate.StateSelectorApply, resource); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		val, ok := resource.Attributes().Get("service.version")
		assert.True(t, ok)
		assert.Equal(t, "1.0.0", val.Str())
	})
}
