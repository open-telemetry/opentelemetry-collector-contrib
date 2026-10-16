// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"
	"iter"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type metricsPartitioner interface {
	partitionMetrics(ctx context.Context, md pmetric.Metrics) ([]partitionedMetrics, error)
}

type partitionedMetrics = partitioned[pmetric.Metrics]

// otelcolMetricsPartitioner evaluates expressions against the request-scoped otelcol context.
type otelcolMetricsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlotelcol.TransformContext]
}

func (p *otelcolMetricsPartitioner) partitionMetrics(ctx context.Context, md pmetric.Metrics) ([]partitionedMetrics, error) {
	if md.ResourceMetrics().Len() == 0 {
		return nil, nil
	}
	values := make([]keyValue, len(p.expressions))
	if err := evaluateStringExpressions(ctx, p.expressions, ottlotelcol.NewTransformContext(), values); err != nil {
		return nil, err
	}
	return []partitionedMetrics{{values: values, data: md}}, nil
}

// resourceMetricsPartitioner evaluates expressions at the ResourceMetrics level.
type resourceMetricsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlresource.TransformContext]
}

func (p *resourceMetricsPartitioner) partitionMetrics(ctx context.Context, md pmetric.Metrics) ([]partitionedMetrics, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, md.ResourceMetrics().Len())
	for _, rm := range md.ResourceMetrics().All() {
		id, err := assign(ctx, g, p.expressions, ottlresource.NewTransformContext(rm.Resource(), rm))
		if err != nil {
			return nil, err
		}
		ids = append(ids, int32(id))
	}
	if parts, ok := single(g, md); ok {
		return parts, nil
	}

	parts := newPartitions(g, pmetric.NewMetrics)
	for i, rm := range md.ResourceMetrics().All() {
		rm.MoveTo(parts[ids[i]].data.ResourceMetrics().AppendEmpty())
	}
	return parts, nil
}

// scopeMetricsPartitioner evaluates expressions at the ScopeMetrics level.
type scopeMetricsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlscope.TransformContext]
}

func (p *scopeMetricsPartitioner) partitionMetrics(ctx context.Context, md pmetric.Metrics) ([]partitionedMetrics, error) {
	g := newGrouper(len(p.expressions))
	var ids []int32
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			id, err := assign(ctx, g, p.expressions, ottlscope.NewTransformContext(sm.Scope(), rm.Resource(), sm, rm))
			if err != nil {
				return nil, err
			}
			ids = append(ids, int32(id))
		}
	}
	if parts, ok := single(g, md); ok {
		return parts, nil
	}

	parts := newPartitions(g, pmetric.NewMetrics)
	dests := make([]metricsDest, len(parts))
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			id := ids[0]
			ids = ids[1:]
			sm.MoveTo(dests[id].resource(parts[id].data, rm).ScopeMetrics().AppendEmpty())
		}
	}
	return parts, nil
}

// metricMetricsPartitioner evaluates expressions at the Metric level.
type metricMetricsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlmetric.TransformContext]
}

func (p *metricMetricsPartitioner) partitionMetrics(ctx context.Context, md pmetric.Metrics) ([]partitionedMetrics, error) {
	g := newGrouper(len(p.expressions))
	var ids []int32
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				id, err := assign(ctx, g, p.expressions, ottlmetric.NewTransformContext(rm, sm, m))
				if err != nil {
					return nil, err
				}
				ids = append(ids, int32(id))
			}
		}
	}
	if parts, ok := single(g, md); ok {
		return parts, nil
	}

	parts := newPartitions(g, pmetric.NewMetrics)
	dests := make([]metricsDest, len(parts))
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				id := ids[0]
				ids = ids[1:]
				m.MoveTo(dests[id].scope(parts[id].data, rm, sm).Metrics().AppendEmpty())
			}
		}
	}
	return parts, nil
}

// datapointMetricsPartitioner evaluates expressions at the DataPoint level.
type datapointMetricsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottldatapoint.TransformContext]
}

func (p *datapointMetricsPartitioner) partitionMetrics(ctx context.Context, md pmetric.Metrics) ([]partitionedMetrics, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, md.DataPointCount())
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				err := iterateDatapoints(m, func(dp any) error {
					id, err := assign(ctx, g, p.expressions, ottldatapoint.NewTransformContext(rm, sm, m, dp))
					ids = append(ids, int32(id))
					return err
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if parts, ok := single(g, md); ok {
		return parts, nil
	}

	parts := newPartitions(g, pmetric.NewMetrics)
	dests := make([]metricsDest, len(parts))
	for _, rm := range md.ResourceMetrics().All() {
		for _, sm := range rm.ScopeMetrics().All() {
			for _, m := range sm.Metrics().All() {
				dest := func(id int32) pmetric.Metric { return dests[id].metric(parts[id].data, rm, sm, m) }
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					ids = moveDatapoints(m.Gauge().DataPoints(), ids, func(id int32) pmetric.NumberDataPointSlice { return dest(id).Gauge().DataPoints() })
				case pmetric.MetricTypeSum:
					ids = moveDatapoints(m.Sum().DataPoints(), ids, func(id int32) pmetric.NumberDataPointSlice { return dest(id).Sum().DataPoints() })
				case pmetric.MetricTypeHistogram:
					ids = moveDatapoints(m.Histogram().DataPoints(), ids, func(id int32) pmetric.HistogramDataPointSlice { return dest(id).Histogram().DataPoints() })
				case pmetric.MetricTypeExponentialHistogram:
					ids = moveDatapoints(m.ExponentialHistogram().DataPoints(), ids, func(id int32) pmetric.ExponentialHistogramDataPointSlice {
						return dest(id).ExponentialHistogram().DataPoints()
					})
				case pmetric.MetricTypeSummary:
					ids = moveDatapoints(m.Summary().DataPoints(), ids, func(id int32) pmetric.SummaryDataPointSlice { return dest(id).Summary().DataPoints() })
				}
			}
		}
	}
	return parts, nil
}

// moveDatapoints moves each datapoint of src to the slice returned by dest
// for its partition ID, consuming one ID per datapoint, and returns the
// remaining IDs.
func moveDatapoints[E interface{ MoveTo(E) }, S interface {
	All() iter.Seq2[int, E]
	AppendEmpty() E
}](src S, ids []int32, dest func(int32) S) []int32 {
	for _, dp := range src.All() {
		dp.MoveTo(dest(ids[0]).AppendEmpty())
		ids = ids[1:]
	}
	return ids
}

// metricsDest tracks a partition's destination containers for the source
// resource, scope, and metric currently being visited. Sources are visited
// in order and never revisited, so only the most recent ones can match.
type metricsDest struct {
	srcRM, rm pmetric.ResourceMetrics
	srcSM, sm pmetric.ScopeMetrics
	srcM, m   pmetric.Metric
}

func (d *metricsDest) resource(data pmetric.Metrics, src pmetric.ResourceMetrics) pmetric.ResourceMetrics {
	if d.srcRM != src {
		d.srcRM = src
		d.rm = data.ResourceMetrics().AppendEmpty()
		src.Resource().CopyTo(d.rm.Resource())
		d.rm.SetSchemaUrl(src.SchemaUrl())
	}
	return d.rm
}

func (d *metricsDest) scope(data pmetric.Metrics, srcRM pmetric.ResourceMetrics, src pmetric.ScopeMetrics) pmetric.ScopeMetrics {
	if d.srcSM != src {
		d.srcSM = src
		d.sm = d.resource(data, srcRM).ScopeMetrics().AppendEmpty()
		src.Scope().CopyTo(d.sm.Scope())
		d.sm.SetSchemaUrl(src.SchemaUrl())
	}
	return d.sm
}

func (d *metricsDest) metric(data pmetric.Metrics, srcRM pmetric.ResourceMetrics, srcSM pmetric.ScopeMetrics, src pmetric.Metric) pmetric.Metric {
	if d.srcM != src {
		d.srcM = src
		d.m = d.scope(data, srcRM, srcSM).Metrics().AppendEmpty()
		initDestMetric(src, d.m)
	}
	return d.m
}

// initDestMetric copies metric-level metadata (name, description, unit, type structure)
// from src to dest without copying any datapoints.
func initDestMetric(src, dest pmetric.Metric) {
	dest.SetName(src.Name())
	dest.SetDescription(src.Description())
	dest.SetUnit(src.Unit())
	src.Metadata().CopyTo(dest.Metadata())
	switch src.Type() {
	case pmetric.MetricTypeGauge:
		dest.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		s := dest.SetEmptySum()
		s.SetAggregationTemporality(src.Sum().AggregationTemporality())
		s.SetIsMonotonic(src.Sum().IsMonotonic())
	case pmetric.MetricTypeHistogram:
		h := dest.SetEmptyHistogram()
		h.SetAggregationTemporality(src.Histogram().AggregationTemporality())
	case pmetric.MetricTypeExponentialHistogram:
		eh := dest.SetEmptyExponentialHistogram()
		eh.SetAggregationTemporality(src.ExponentialHistogram().AggregationTemporality())
	case pmetric.MetricTypeSummary:
		dest.SetEmptySummary()
	}
}

// iterateDatapoints calls f for each datapoint in m, regardless of metric type.
func iterateDatapoints(m pmetric.Metric, f func(dp any) error) error {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		for _, dp := range m.Gauge().DataPoints().All() {
			if err := f(dp); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSum:
		for _, dp := range m.Sum().DataPoints().All() {
			if err := f(dp); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeHistogram:
		for _, dp := range m.Histogram().DataPoints().All() {
			if err := f(dp); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		for _, dp := range m.ExponentialHistogram().DataPoints().All() {
			if err := f(dp); err != nil {
				return err
			}
		}
	case pmetric.MetricTypeSummary:
		for _, dp := range m.Summary().DataPoints().All() {
			if err := f(dp); err != nil {
				return err
			}
		}
	}
	return nil
}

func newMetricsPartitioner(expressions []string, settings component.TelemetrySettings) (metricsPartitioner, error) {
	otelcolParser, err := ottlotelcol.NewParser(
		ottlfuncs.StandardConverters[*ottlotelcol.TransformContext](),
		settings,
		ottlotelcol.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	resourceParser, err := ottlresource.NewParser(
		ottlfuncs.StandardConverters[*ottlresource.TransformContext](),
		settings,
		ottlresource.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	scopeParser, err := ottlscope.NewParser(
		ottlfuncs.StandardConverters[*ottlscope.TransformContext](),
		settings,
		ottlscope.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	metricParser, err := ottlmetric.NewParser(
		ottlfuncs.StandardConverters[*ottlmetric.TransformContext](),
		settings,
		ottlmetric.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	datapointParser, err := ottldatapoint.NewParser(
		ottlfuncs.StandardConverters[*ottldatapoint.TransformContext](),
		settings,
		ottldatapoint.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection[metricsPartitioner](
		settings,
		ottl.WithParserCollectionContext[*ottlotelcol.TransformContext, metricsPartitioner](
			ottlotelcol.ContextName,
			&otelcolParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[metricsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlotelcol.TransformContext],
			) (metricsPartitioner, error) {
				return &otelcolMetricsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlresource.TransformContext, metricsPartitioner](
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[metricsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlresource.TransformContext],
			) (metricsPartitioner, error) {
				return &resourceMetricsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlscope.TransformContext, metricsPartitioner](
			ottlscope.ContextName,
			&scopeParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[metricsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlscope.TransformContext],
			) (metricsPartitioner, error) {
				return &scopeMetricsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlmetric.TransformContext, metricsPartitioner](
			ottlmetric.ContextName,
			&metricParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[metricsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlmetric.TransformContext],
			) (metricsPartitioner, error) {
				return &metricMetricsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottldatapoint.TransformContext, metricsPartitioner](
			ottldatapoint.ContextName,
			&datapointParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[metricsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottldatapoint.TransformContext],
			) (metricsPartitioner, error) {
				return &datapointMetricsPartitioner{expressions: parsed}, nil
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	return pc.ParseValueExpressions(ottl.NewValueExpressionsGetter(expressions))
}
