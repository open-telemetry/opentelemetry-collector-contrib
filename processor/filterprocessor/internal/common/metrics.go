// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type MetricsConsumer interface {
	Context() ContextID
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error
}

type metricConditions struct {
	expr.BoolExpr[ottlmetric.TransformContext]
}

func (m metricConditions) Context() ContextID {
	return Metric
}

func (m metricConditions) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var condErr error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
			smetrics.Metrics().RemoveIf(func(metrics pmetric.Metric) bool {
				tCtx := ottlmetric.NewTransformContext(metrics, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
				cond, err := m.Eval(ctx, tCtx)
				if err != nil {
					condErr = multierr.Append(condErr, err)
					return false
				}
				return cond
			})
			return smetrics.Metrics().Len() == 0
		})
		return rmetrics.ScopeMetrics().Len() == 0
	})
	if md.ResourceMetrics().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

type dataPointConditions struct {
	expr.BoolExpr[ottldatapoint.TransformContext]
}

func (d dataPointConditions) Context() ContextID {
	return DataPoint
}

func (d dataPointConditions) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var condErr error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
			smetrics.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					err := d.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Sum().DataPoints().Len() == 0
				case pmetric.MetricTypeGauge:
					err := d.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Gauge().DataPoints().Len() == 0
				case pmetric.MetricTypeHistogram:
					err := d.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Histogram().DataPoints().Len() == 0
				case pmetric.MetricTypeExponentialHistogram:
					err := d.handleExponentialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.ExponentialHistogram().DataPoints().Len() == 0
				case pmetric.MetricTypeSummary:
					err := d.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metric, smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Summary().DataPoints().Len() == 0
				default:
					return false
				}
			})
			return smetrics.Metrics().Len() == 0
		})
		return rmetrics.ScopeMetrics().Len() == 0
	})
	if md.ResourceMetrics().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func (d dataPointConditions) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource, scopeMetrics, resourceMetrics)
		cond, err := d.Eval(ctx, tCtx)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

func (d dataPointConditions) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.HistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource, scopeMetrics, resourceMetrics)
		cond, err := d.Eval(ctx, tCtx)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

func (d dataPointConditions) handleExponentialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.ExponentialHistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource, scopeMetrics, resourceMetrics)
		cond, err := d.Eval(ctx, tCtx)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

func (d dataPointConditions) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.SummaryDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContext(datapoint, metric, metrics, is, resource, scopeMetrics, resourceMetrics)
		cond, err := d.Eval(ctx, tCtx)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

type MetricParserCollection ottl.ParserCollection[MetricsConsumer]

type MetricParserCollectionOption ottl.ParserCollectionOption[MetricsConsumer]

func WithMetricParser(functions map[string]ottl.Factory[ottlmetric.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[MetricsConsumer]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlmetric.ContextName, &metricParser, ottl.WithConditionConverter(convertMetricConditions))(pc)
	}
}

func WithDataPointParser(functions map[string]ottl.Factory[ottldatapoint.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[MetricsConsumer]) error {
		dataPointParser, err := ottldatapoint.NewParser(functions, pc.Settings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottldatapoint.ContextName, &dataPointParser, ottl.WithConditionConverter(convertDataPointConditions))(pc)
	}
}

func WithMetricErrorMode(errorMode ottl.ErrorMode) MetricParserCollectionOption {
	return MetricParserCollectionOption(ottl.WithParserCollectionErrorMode[MetricsConsumer](errorMode))
}

func NewMetricParserCollection(settings component.TelemetrySettings, options ...MetricParserCollectionOption) (*MetricParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[MetricsConsumer]{
		withCommonContextParsers[MetricsConsumer](),
		ottl.EnableParserCollectionModifiedPathsLogging[MetricsConsumer](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[MetricsConsumer](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	mpc := MetricParserCollection(*pc)
	return &mpc, nil
}

func convertMetricConditions(pc *ottl.ParserCollection[MetricsConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[ottlmetric.TransformContext]) (MetricsConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	mConditions := ottlmetric.NewConditionSequence(parsedConditions, pc.Settings, ottlmetric.WithConditionSequenceErrorMode(errorMode))
	return metricConditions{&mConditions}, nil
}

func convertDataPointConditions(pc *ottl.ParserCollection[MetricsConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[ottldatapoint.TransformContext]) (MetricsConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	dpConditions := ottldatapoint.NewConditionSequence(parsedConditions, pc.Settings, ottldatapoint.WithConditionSequenceErrorMode(errorMode))
	return dataPointConditions{&dpConditions}, nil
}

func (mpc *MetricParserCollection) ParseContextConditions(contextConditions ContextConditions) (MetricsConsumer, error) {
	pc := ottl.ParserCollection[MetricsConsumer](*mpc)
	return pc.ParseConditions(contextConditions)
}
