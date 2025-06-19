// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlmetric

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_newPathGetSetter(t *testing.T) {
	refMetric := createTelemetry()

	newCache := pcommon.NewMap()
	newCache.PutStr("temp", "value")

	newMetadata := pcommon.NewMap()
	newMetadata.PutStr("new_k", "new_v")

	newMetric := pmetric.NewMetric()
	newMetric.SetName("new name")

	newDataPoints := pmetric.NewNumberDataPointSlice()
	dataPoint := newDataPoints.AppendEmpty()
	dataPoint.SetIntValue(1)

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		orig         any
		newVal       any
		modified     func(metric pmetric.Metric, cache pcommon.Map)
		setStatement string
		getStatement string
	}{
		{
			name: "metric name",
			path: &pathtest.Path[TransformContext]{
				N: "name",
			},
			orig:   "name",
			newVal: "new name",
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				metric.SetName("new name")
			},
			setStatement: `set(name, "new name")`,
			getStatement: `name`,
		},
		{
			name: "metric description",
			path: &pathtest.Path[TransformContext]{
				N: "description",
			},
			orig:   "description",
			newVal: "new description",
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				metric.SetDescription("new description")
			},
			setStatement: `set(description, "new description")`,
			getStatement: `description`,
		},
		{
			name: "metric unit",
			path: &pathtest.Path[TransformContext]{
				N: "unit",
			},
			orig:   "unit",
			newVal: "new unit",
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				metric.SetUnit("new unit")
			},
			setStatement: `set(unit, "new unit")`,
			getStatement: `unit`,
		},
		{
			name: "metric type",
			path: &pathtest.Path[TransformContext]{
				N: "type",
			},
			orig:   int64(pmetric.MetricTypeSum),
			newVal: int64(pmetric.MetricTypeSum),
			modified: func(_ pmetric.Metric, _ pcommon.Map) {
			},
			setStatement: fmt.Sprintf("set(type, %d)", int64(pmetric.MetricTypeSum)),
			getStatement: `type`,
		},
		{
			name: "metric aggregation_temporality",
			path: &pathtest.Path[TransformContext]{
				N: "aggregation_temporality",
			},
			orig:   int64(2),
			newVal: int64(1),
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			},
			setStatement: `set(aggregation_temporality, 1)`,
			getStatement: `aggregation_temporality`,
		},
		{
			name: "metric is_monotonic",
			path: &pathtest.Path[TransformContext]{
				N: "is_monotonic",
			},
			orig:   true,
			newVal: false,
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				metric.Sum().SetIsMonotonic(false)
			},
			setStatement: `set(is_monotonic, false)`,
			getStatement: `is_monotonic`,
		},
		{
			name: "metric data points",
			path: &pathtest.Path[TransformContext]{
				N: "data_points",
			},
			orig:   refMetric.Sum().DataPoints(),
			newVal: newDataPoints,
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				newDataPoints.CopyTo(metric.Sum().DataPoints())
			},
		},
		{
			name: "metadata",
			path: &pathtest.Path[TransformContext]{
				N: "metadata",
			},
			orig:   pcommon.NewMap(),
			newVal: newMetadata,
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				newMetadata.CopyTo(metric.Metadata())
			},
		},
		{
			name: "metadata access",
			path: &pathtest.Path[TransformContext]{
				N: "metadata",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(metric pmetric.Metric, _ pcommon.Map) {
				metric.Metadata().PutStr("temp", "new value")
			},
			setStatement: `set(metadata["temp"], "new value")`,
			getStatement: `metadata["temp"]`,
		},
		{
			name: "cache",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
			},
			orig:   pcommon.NewMap(),
			newVal: newCache,
			modified: func(_ pmetric.Metric, cache pcommon.Map) {
				newCache.CopyTo(cache)
			},
			setStatement: `set(cache, {"temp": "value"})`,
			getStatement: `cache`,
		},
		{
			name: "cache access",
			path: &pathtest.Path[TransformContext]{
				N: "cache",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("temp"),
					},
				},
			},
			orig:   nil,
			newVal: "new value",
			modified: func(_ pmetric.Metric, cache pcommon.Map) {
				cache.PutStr("temp", "new value")
			},
			setStatement: `set(cache["temp"], "new value")`,
			getStatement: `cache["temp"]`,
		},
	}
	// Copy all tests cases and sets the path.Context value to the generated ones.
	// It ensures all exiting field access also work when the path context is set.
	for _, tt := range slices.Clone(tests) {
		testWithContext := tt
		testWithContext.name = "with_path_context:" + tt.name
		pathWithContext := *tt.path.(*pathtest.Path[TransformContext])
		pathWithContext.C = ctxmetric.Name
		testWithContext.path = ottl.Path[TransformContext](&pathWithContext)
		tests = append(tests, testWithContext)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a controlled test cache
			testCache := pcommon.NewMap()
			cacheGetter := func(_ TransformContext) pcommon.Map {
				return testCache
			}

			accessor, err := pathExpressionParser(cacheGetter)(tt.path)
			assert.NoError(t, err)

			metric := createTelemetry()

			ctx := NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.orig, got)

			err = accessor.Set(context.Background(), ctx, tt.newVal)
			assert.NoError(t, err)

			exMetric := createTelemetry()
			exCache := pcommon.NewMap()
			tt.modified(exMetric, exCache)

			assert.Equal(t, exMetric, metric)
			assert.Equal(t, exCache, testCache)
		})
	}
	stmtParser := createParser(t)

	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			if tt.setStatement != "" {
				statement, err := stmtParser.ParseStatement(tt.setStatement)
				require.NoError(t, err)

				metric := createTelemetry()

				ctx := NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

				_, executed, err := statement.Execute(context.Background(), ctx)
				require.NoError(t, err)
				assert.True(t, executed)

				getStatement, err := stmtParser.ParseValueExpression(tt.getStatement)
				require.NoError(t, err)

				metric = createTelemetry()

				ctx = NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

				getResult, err := getStatement.Eval(context.Background(), ctx)

				assert.NoError(t, err)
				assert.Equal(t, tt.orig, getResult)
			}
		})
	}
}

func createParser(t *testing.T) ottl.Parser[TransformContext] {
	settings := componenttest.NewNopTelemetrySettings()
	stmtParser, err := NewParser(ottlfuncs.StandardFuncs[TransformContext](), settings)
	require.NoError(t, err)
	return stmtParser
}

func Test_newPathGetSetter_higherContextPath(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("foo", "bar")

	instrumentationScope := pcommon.NewInstrumentationScope()
	instrumentationScope.SetName("instrumentation_scope")

	ctx := NewTransformContext(pmetric.NewMetric(), pmetric.NewMetricSlice(), instrumentationScope, resource, pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

	tests := []struct {
		name         string
		path         ottl.Path[TransformContext]
		expected     any
		getStatement string
	}{
		{
			name: "resource",
			path: &pathtest.Path[TransformContext]{C: "", N: "resource", NextPath: &pathtest.Path[TransformContext]{
				N: "attributes",
				KeySlice: []ottl.Key[TransformContext]{
					&pathtest.Key[TransformContext]{
						S: ottltest.Strp("foo"),
					},
				},
			}},
			expected:     "bar",
			getStatement: `resource.attributes["foo"]`,
		},
		{
			name: "resource with context",
			path: &pathtest.Path[TransformContext]{C: "resource", N: "attributes", KeySlice: []ottl.Key[TransformContext]{
				&pathtest.Key[TransformContext]{
					S: ottltest.Strp("foo"),
				},
			}},
			expected:     "bar",
			getStatement: `resource.attributes["foo"]`,
		},
		{
			name:         "instrumentation_scope",
			path:         &pathtest.Path[TransformContext]{N: "instrumentation_scope", NextPath: &pathtest.Path[TransformContext]{N: "name"}},
			expected:     instrumentationScope.Name(),
			getStatement: `instrumentation_scope.name`,
		},
		{
			name:         "instrumentation_scope with context",
			path:         &pathtest.Path[TransformContext]{C: "instrumentation_scope", N: "name"},
			expected:     instrumentationScope.Name(),
			getStatement: `instrumentation_scope.name`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessor, err := pathExpressionParser(getCache)(tt.path)
			require.NoError(t, err)

			got, err := accessor.Get(context.Background(), ctx)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
	stmtParser := createParser(t)
	for _, tt := range tests {
		t.Run(tt.name+"_conversion", func(t *testing.T) {
			getExpression, err := stmtParser.ParseValueExpression(tt.getStatement)
			require.NoError(t, err)
			require.NotNil(t, getExpression)
			getResult, err := getExpression.Eval(context.Background(), ctx)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, getResult)
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
			assert.Equal(t, tt.want, *actual)
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
