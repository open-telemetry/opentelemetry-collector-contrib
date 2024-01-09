// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlmetric

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {

	refMetric := createMetricTelemetry()

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	newDataPoints := pmetric.NewNumberDataPointSlice()
	dataPoint := newDataPoints.AppendEmpty()
	dataPoint.SetIntValue(1)

	tests := []struct {
		name     string
		path     ottl.Path[TransformContext]
		orig     any
		newVal   any
		modified func(metric pmetric.Metric, cache pcommon.Map)
	}{
		{
			name: "metric name",
			path: &internal.TestPath[TransformContext]{
				N: "name",
			},
			orig:   "name",
			newVal: "new name",
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				metric.SetName("new name")
			},
		},
		{
			name: "metric description",
			path: &internal.TestPath[TransformContext]{
				N: "description",
			},
			orig:   "description",
			newVal: "new description",
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				metric.SetDescription("new description")
			},
		},
		{
			name: "metric unit",
			path: &internal.TestPath[TransformContext]{
				N: "unit",
			},
			orig:   "unit",
			newVal: "new unit",
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				metric.SetUnit("new unit")
			},
		},
		{
			name: "metric type",
			path: &internal.TestPath[TransformContext]{
				N: "type",
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
			},
		},
		{
			name: "metric aggregation_temporality",
			path: &internal.TestPath[TransformContext]{
				N: "aggregation_temporality",
			},
			orig:   int64(2),
			newVal: int64(1),
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
		},
		{
			name: "metric is_monotonic",
			path: &internal.TestPath[TransformContext]{
				N: "is_monotonic",
			},
			orig:   true,
			newVal: false,
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				metric.Sum().SetIsMonotonic(false)
			},
		},
		{
			name: "metric data points",
			path: &internal.TestPath[TransformContext]{
				N: "data_points",
			},
			orig:   refMetric.Sum().DataPoints(),
			newVal: newDataPoints,
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				newDataPoints.CopyTo(metric.Sum().DataPoints())
			},
		},
		{
			name: "cache",
			path: &internal.TestPath[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
		},
		{
			name: "cache access",
			path: &internal.TestPath[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&internal.TestKey[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(metric pmetric.Metric, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pep := pathExpressionParser{}
			accessor, err := pep.parsePath(tt.path)
			assert.NoError(t, err)

			metric := createMetricTelemetry()

			ctx := NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource())

			got, err := accessor.Get(context.Background(), ctx)
			assert.Nil(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.Nil(t, err)

			exMetric := createMetricTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exMetric, exCache)

			assert.Equal(t, exMetric, metric)
			assert.Equal(t, exCache, ctx.getCache())
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

func Test_ParseEnum(t *testing.T) {
	tests := []struct {
		name string
		want ottl.Enum
	}{
		{
			name: "AGGREGATION_TEMPORALITY_UNSPECIFIED",
			want: ottl.Enum(pmetric.AggregationTemporalityUnspecified),
		},
		{
			name: "AGGREGATION_TEMPORALITY_DELTA",
			want: ottl.Enum(pmetric.AggregationTemporalityDelta),
		},
		{
			name: "AGGREGATION_TEMPORALITY_CUMULATIVE",
			want: ottl.Enum(pmetric.AggregationTemporalityCumulative),
		},
		{
			name: "METRIC_DATA_TYPE_NONE",
			want: ottl.Enum(pmetric.MetricTypeEmpty),
		},
		{
			name: "METRIC_DATA_TYPE_GAUGE",
			want: ottl.Enum(pmetric.MetricTypeGauge),
		},
		{
			name: "METRIC_DATA_TYPE_SUM",
			want: ottl.Enum(pmetric.MetricTypeSum),
		},
		{
			name: "METRIC_DATA_TYPE_HISTOGRAM",
			want: ottl.Enum(pmetric.MetricTypeHistogram),
		},
		{
			name: "METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM",
			want: ottl.Enum(pmetric.MetricTypeExponentialHistogram),
		},
		{
			name: "METRIC_DATA_TYPE_SUMMARY",
			want: ottl.Enum(pmetric.MetricTypeSummary),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseEnum((*ottl.EnumSymbol)(ottltest.Strp(tt.name)))
			assert.NoError(t, err)
			assert.Equal(t, *actual, tt.want)
		})
	}
}

func Test_ParseEnum_False(t *testing.T) {
	tests := []struct {
		name       string
		enumSymbol *ottl.EnumSymbol
	}{
		{
			name:       "unknown enum symbol",
			enumSymbol: (*ottl.EnumSymbol)(ottltest.Strp("not an enum")),
		},
		{
			name:       "nil enum symbol",
			enumSymbol: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseEnum(tt.enumSymbol)
			assert.Error(t, err)
			assert.Nil(t, actual)
		})
	}
}
