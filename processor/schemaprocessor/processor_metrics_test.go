// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestMetrics_RenameAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              pmetric.Metrics
		out             pmetric.Metrics
		transformations string
		targetVersion   string
	}{
		{
			name: "one_version_downgrade",
			in: func() pmetric.Metrics {
				in := pmetric.NewMetrics()
				in.ResourceMetrics().AppendEmpty()
				in.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				in.ResourceMetrics().At(0).Resource().Attributes().PutStr("new.resource.name", "test-cluster")
				in.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()

				in.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				// Sum metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("new.sum.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetEmptySum()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				// Gauge metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetName("new.gauge.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetEmptyGauge()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				// Histogram metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetName("new.histogram.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetEmptyHistogram()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				// Summary metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetName("new.summary.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetEmptySummary()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				return in
			}(),
			out: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				out.ResourceMetrics().AppendEmpty()
				out.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				out.ResourceMetrics().At(0).Resource().Attributes().PutStr("old.resource.name", "test-cluster")

				out.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				// Sum metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("old.sum.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetEmptySum()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				// Gauge metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetName("old.gauge.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetEmptyGauge()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				// Histogram metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetName("old.histogram.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetEmptyHistogram()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				// Summary metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetName("old.summary.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetEmptySummary()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				return out
			}(),
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  metrics:
		changes:
			- rename_attributes:
				  attribute_map:
					old.attr.name: new.attr.name
	    	- rename_metrics:
				  old.sum.metric: new.sum.metric
				  old.gauge.metric: new.gauge.metric
			- rename_metrics:
				  old.histogram.metric: new.histogram.metric
				  old.summary.metric: new.summary.metric
	1.8.0:`,
			targetVersion: "1.8.0",
		},
		{
			name: "one_version_upgrade",
			in: func() pmetric.Metrics {
				in := pmetric.NewMetrics()
				in.ResourceMetrics().AppendEmpty()
				in.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				in.ResourceMetrics().At(0).Resource().Attributes().PutStr("old.resource.name", "test-cluster")
				in.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()

				// Sum metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("old.sum.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetEmptySum()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				// Gauge metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetName("old.gauge.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetEmptyGauge()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				// Histogram metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetName("old.histogram.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetEmptyHistogram()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				// Summary metric
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetName("old.summary.metric")
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetEmptySummary()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().AppendEmpty()
				in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("old.attr.name", "test-cluster")

				return in
			}(),
			out: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				out.ResourceMetrics().AppendEmpty()
				out.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				out.ResourceMetrics().At(0).Resource().Attributes().PutStr("new.resource.name", "test-cluster")
				out.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()

				out.ResourceMetrics().At(0).ScopeMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
				// Sum metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("new.sum.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetEmptySum()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				// Gauge metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetName("new.gauge.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).SetEmptyGauge()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				// Histogram metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetName("new.histogram.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).SetEmptyHistogram()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2).Histogram().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				// Summary metric
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetName("new.summary.metric")
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).SetEmptySummary()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().AppendEmpty()
				out.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3).Summary().DataPoints().At(0).Attributes().PutStr("new.attr.name", "test-cluster")

				return out
			}(),
			transformations: `
	1.9.0:
	  all:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.resource.name: new.resource.name
	  metrics:
		changes:
		  - rename_attributes:
			  attribute_map:
				old.attr.name: new.attr.name
		  - rename_metrics:
				old.sum.metric: new.sum.metric
				old.gauge.metric: new.gauge.metric
		  - rename_metrics:
				old.histogram.metric: new.histogram.metric
				old.summary.metric: new.summary.metric
	1.8.0:`,
			targetVersion: "1.9.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := newTestSchemaProcessor(t, tt.transformations, tt.targetVersion)
			ctx := context.Background()
			out, err := pr.processMetrics(ctx, tt.in)
			if err != nil {
				t.Errorf("Error while processing metrics: %v", err)
			}
			assert.Equal(t, tt.out, out, "Metrics transformation failed")
		})
	}
}
