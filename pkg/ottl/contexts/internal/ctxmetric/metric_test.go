// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxmetric_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func TestPathGetSetter(t *testing.T) {
	refMetric := createTelemetry()

	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	newMetadata := pcommon.NewMap()
	newMetadata.PutStr("new_k", "new_v")

	newDataPoints := pmetric.NewNumberDataPointSlice()
	dataPoint := newDataPoints.AppendEmpty()
	dataPoint.SetIntValue(1)

	tests := []struct {
		name     string
		path     ottl.Path[*testContext]
		orig     any
		newVal   any
		modified func(metric pmetric.Metric)
	}{
		{
			name: "metric name",
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "type",
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(_ pmetric.Metric) {
			},
		},
		{
			name: "metric aggregation_temporality",
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
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
			path: &pathtest.Path[*testContext]{
				N: "data_points",
			},
			orig:   refMetric.Sum().DataPoints(),
			newVal: newDataPoints,
			modified: func(metric pmetric.Metric) {
				newDataPoints.CopyTo(metric.Sum().DataPoints())
			},
		},
		{
			name: "metric metadata",
			path: &pathtest.Path[*testContext]{
				N: "metadata",
			},
			orig:   pcommon.NewMap(),
			newVal: newMetadata,
			modified: func(metric pmetric.Metric) {
				newMetadata.CopyTo(metric.Metadata())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := ctxmetric.PathGetSetter(tt.path)
			assert.NoError(t, err)

			metric := createTelemetry()

			got, err := accessor.Get(context.Background(), newTestContext(metric))
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), newTestContext(metric), tt.newVal)
			assert.NoError(t, err)

			expectedMetric := createTelemetry()
			tt.modified(expectedMetric)

			assert.Equal(t, expectedMetric, metric)
		})
	}
}

func createTelemetry() pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName("name")
	metric.SetDescription("description")
	metric.SetUnit("unit")
	metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	metric.Sum().SetIsMonotonic(true)
	return metric
}

type testContext struct {
	metric pmetric.Metric
}

func (m *testContext) GetMetric() pmetric.Metric {
	return m.metric
}

func newTestContext(metric pmetric.Metric) *testContext {
	return &testContext{metric: metric}
}
