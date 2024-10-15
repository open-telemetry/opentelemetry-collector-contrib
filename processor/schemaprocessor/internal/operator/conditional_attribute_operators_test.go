package operator

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"
)

func assertAttributeEquals(t *testing.T, attributes pcommon.Map, key string, value string) {
	t.Helper()
	val, ok := attributes.Get(key)
	require.True(t, ok)
	require.Equal(t, value, val.Str())
}

func TestMetricDataPointAttributeOperator(t *testing.T) {
	attrChange := migrate.NewConditionalAttributeSet(map[string]string{
		"service_version": "service.version",
	}, "http_request")
	metricDataPointAttributeOperator := MetricDataPointAttributeOperator{attrChange}

	tests := []struct {
		name      string
		generator func(metric pmetric.Metric) alias.Attributed
	}{
		{
			name: "MetricOperatorExponentialHistogram",
			generator: func(metric pmetric.Metric) alias.Attributed {
				metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.ExponentialHistogram().DataPoints().At(0)
			},
		},
		{
			name: "MetricOperatorGauge",
			generator: func(metric pmetric.Metric) alias.Attributed {
				metric.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Gauge().DataPoints().At(0)
			},
		},
		{
			name: "MetricOperatorHistogram",
			generator: func(metric pmetric.Metric) alias.Attributed {
				metric.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Histogram().DataPoints().At(0)
			},
		},
		{
			name: "MetricOperatorSum",
			generator: func(metric pmetric.Metric) alias.Attributed {
				metric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Sum().DataPoints().At(0)
			},
		},
		{
			name: "MetricOperatorSummary",
			generator: func(metric pmetric.Metric) alias.Attributed {
				metric.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("service_version", "1.0.0")
				return metric.Summary().DataPoints().At(0)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// generate new metric
			metric := pmetric.NewMetric()
			item := tt.generator(metric)
			// assert it was constructed correctly
			assertAttributeEquals(t, item.Attributes(), "service_version", "1.0.0")

			// name is blank - migrator shouldn't do anything
			err := metricDataPointAttributeOperator.Do(migrate.StateSelectorApply, metric)
			require.NoError(t, err)
			assertAttributeEquals(t, item.Attributes(), "service_version", "1.0.0")

			// name is http_request - migrator should change the attribute
			metric.SetName("http_request")
			err = metricDataPointAttributeOperator.Do(migrate.StateSelectorApply, metric)
			require.NoError(t, err)
			assertAttributeEquals(t, item.Attributes(), "service.version", "1.0.0")

		})
	}
}

func TestSpanConditionalAttributeOperator(t *testing.T) {
	attrChange := migrate.NewConditionalAttributeSet(map[string]string{
		"service_version": "service.version",
	}, "http_request")
	spanConditionalAttributeOperator := SpanConditionalAttributeOperator{attrChange}

	span := ptrace.NewSpan()
	span.Attributes().PutStr("service_version", "1.0.0")
	// name is blank, migrator shouldn't do anything
	err := spanConditionalAttributeOperator.Do(migrate.StateSelectorApply, span)
	require.NoError(t, err)
	assertAttributeEquals(t, span.Attributes(), "service_version", "1.0.0")

	span.SetName("http_request")
	err = spanConditionalAttributeOperator.Do(migrate.StateSelectorApply, span)
	require.NoError(t, err)
	assertAttributeEquals(t, span.Attributes(), "service.version", "1.0.0")

}
