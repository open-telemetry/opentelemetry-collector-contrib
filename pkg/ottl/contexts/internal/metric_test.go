// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_MetricPathGetSetter(t *testing.T) {

	refMetric := createMetricTelemetry()

	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	newDataPoints := pmetric.NewNumberDataPointSlice()
	dataPoint := newDataPoints.AppendEmpty()
	dataPoint.SetIntValue(1)

	tests := []struct {
		name     string
		path     ottl.Path[*metricContext]
		orig     any
		newVal   any
		modified func(metric pmetric.Metric)
	}{
		{
			name: "metric name",
			path: &TestPath[*metricContext]{
				N: "name",
			},
			orig:   "name",
			newVal: "new name",
			modified: func(metric pmetric.Metric) {
				metric.SetName("new name")
			},
		},
		{
			name: "metric description",
			path: &TestPath[*metricContext]{
				N: "description",
			},
			orig:   "description",
			newVal: "new description",
			modified: func(metric pmetric.Metric) {
				metric.SetDescription("new description")
			},
		},
		{
			name: "metric unit",
			path: &TestPath[*metricContext]{
				N: "unit",
			},
			orig:   "unit",
			newVal: "new unit",
			modified: func(metric pmetric.Metric) {
				metric.SetUnit("new unit")
			},
		},
		{
			name: "metric type",
			path: &TestPath[*metricContext]{
				N: "type",
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(metric pmetric.Metric) {
			},
		},
		{
			name: "metric aggregation_temporality",
			path: &TestPath[*metricContext]{
				N: "aggregation_temporality",
			},
			orig:   int64(2),
			newVal: int64(1),
			modified: func(metric pmetric.Metric) {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
		},
		{
			name: "metric is_monotonic",
			path: &TestPath[*metricContext]{
				N: "is_monotonic",
			},
			orig:   true,
			newVal: false,
			modified: func(metric pmetric.Metric) {
				metric.Sum().SetIsMonotonic(false)
			},
		},
		{
			name: "metric data points",
			path: &TestPath[*metricContext]{
				N: "data_points",
			},
			orig:   refMetric.Sum().DataPoints(),
			newVal: newDataPoints,
			modified: func(metric pmetric.Metric) {
				newDataPoints.CopyTo(metric.Sum().DataPoints())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := MetricPathGetSetter[*metricContext](tt.path)
			assert.NoError(t, err)

			metric := createMetricTelemetry()

			got, err := accessor.Get(context.Background(), newMetricContext(metric))
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newMetricContext(metric), tt.newVal)
			assert.Nil(t, err)

			expectedMetric := createMetricTelemetry()
			tt.modified(expectedMetric)

			assert.Equal(t, expectedMetric, metric)
		})
	}
}

func createMetricTelemetry() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("name")
	metric.SetDescription("description")
	metric.SetUnit("unit")
	metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	metric.Sum().SetIsMonotonic(true)
	return metric
}

type metricContext struct {
	metric pmetric.Metric
}

func (m *metricContext) GetMetric() pmetric.Metric {
	return m.metric
}

func newMetricContext(metric pmetric.Metric) *metricContext {
	return &metricContext{metric: metric}
}
