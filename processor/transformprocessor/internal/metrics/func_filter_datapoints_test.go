// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func TestFilterDatapoints(t *testing.T) {
	type testCase struct {
		name      string
		args      map[string]string
		valueFunc func() pmetric.Metric
		wantFunc  func() pmetric.Metric
		wantErr   bool
	}
	tests := []testCase{
		{
			name: "Keep datapoints that match attribute",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("key", "value")
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(15.0)
				metric.Gauge().DataPoints().At(1).Attributes().PutStr("key", "other_value")
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(20.0)
				metric.Gauge().DataPoints().At(2).Attributes().PutStr("other_key", "value")
				return metric
			},
			args: map[string]string{"key": "value"},
			wantFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("key", "value")
				return metric
			},
			wantErr: false,
		},
		{
			name: "Keep datapoints that match all expected attributes",
			valueFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("key1", "value1")
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("key2", "value2")
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(15.0)
				metric.Gauge().DataPoints().At(1).Attributes().PutStr("key1", "value1")
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(20.0)
				metric.Gauge().DataPoints().At(2).Attributes().PutStr("key2", "value2")
				return metric
			},
			args: map[string]string{"key1": "value1", "key2": "value2"},
			wantFunc: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test-metric")
				metric.SetEmptyGauge()
				metric.Gauge().DataPoints().AppendEmpty().SetDoubleValue(10.0)
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("key1", "value1")
				metric.Gauge().DataPoints().At(0).Attributes().PutStr("key2", "value2")
				return metric
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := ottlmetric.NewTransformContext(
				tt.valueFunc(),
				pmetric.NewMetricSlice(),
				pcommon.NewInstrumentationScope(),
				pcommon.NewResource(),
				pmetric.NewScopeMetrics(),
				pmetric.NewResourceMetrics(),
			)

			expressionFunc, _ := filterDatapoints(tt.args)
			_, err := expressionFunc(context.Background(), target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.EqualValues(t, tt.wantFunc(), target.GetMetric())
		})
	}
}
