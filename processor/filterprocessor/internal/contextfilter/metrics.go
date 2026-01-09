// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package contextfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/contextfilter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

type MetricsConsumer interface {
	Context() ContextID
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error
}

type metricConditions struct {
	expr.BoolExpr[*ottlmetric.TransformContext]
}

func (metricConditions) Context() ContextID {
	return Metric
}

func (m metricConditions) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var condErr error
	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(metric pmetric.Metric) bool {
				tCtx := ottlmetric.NewTransformContextPtr(rm, sm, metric)
				cond, err := m.Eval(ctx, tCtx)
				tCtx.Close()
				if err != nil {
					condErr = multierr.Append(condErr, err)
					return false
				}
				return cond
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
	if md.ResourceMetrics().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

type dataPointConditions struct {
	expr.BoolExpr[*ottldatapoint.TransformContext]
}

func (dataPointConditions) Context() ContextID {
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
					err := d.handleNumberDataPoints(ctx, rmetrics, smetrics, metric, metric.Sum().DataPoints())
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Sum().DataPoints().Len() == 0
				case pmetric.MetricTypeGauge:
					err := d.handleNumberDataPoints(ctx, rmetrics, smetrics, metric, metric.Gauge().DataPoints())
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Gauge().DataPoints().Len() == 0
				case pmetric.MetricTypeHistogram:
					err := d.handleHistogramDataPoints(ctx, rmetrics, smetrics, metric, metric.Histogram().DataPoints())
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.Histogram().DataPoints().Len() == 0
				case pmetric.MetricTypeExponentialHistogram:
					err := d.handleExponentialHistogramDataPoints(ctx, rmetrics, smetrics, metric, metric.ExponentialHistogram().DataPoints())
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return metric.ExponentialHistogram().DataPoints().Len() == 0
				case pmetric.MetricTypeSummary:
					err := d.handleSummaryDataPoints(ctx, rmetrics, smetrics, metric, metric.Summary().DataPoints())
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

func (d dataPointConditions) handleNumberDataPoints(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dps pmetric.NumberDataPointSlice) error {
	var errors error
	dps.RemoveIf(func(datapoint pmetric.NumberDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, m, datapoint)
		cond, err := d.Eval(ctx, tCtx)
		tCtx.Close()
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

func (d dataPointConditions) handleHistogramDataPoints(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dps pmetric.HistogramDataPointSlice) error {
	var errors error
	dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, m, dp)
		cond, err := d.Eval(ctx, tCtx)
		tCtx.Close()
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

func (d dataPointConditions) handleExponentialHistogramDataPoints(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dps pmetric.ExponentialHistogramDataPointSlice) error {
	var errors error
	dps.RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, m, dp)
		cond, err := d.Eval(ctx, tCtx)
		tCtx.Close()
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return cond
	})
	return errors
}

func (d dataPointConditions) handleSummaryDataPoints(ctx context.Context, rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric, dps pmetric.SummaryDataPointSlice) error {
	var errors error
	dps.RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
		tCtx := ottldatapoint.NewTransformContextPtr(rm, sm, m, dp)
		cond, err := d.Eval(ctx, tCtx)
		tCtx.Close()
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

func WithMetricParser(functions map[string]ottl.Factory[*ottlmetric.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[MetricsConsumer]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlmetric.ContextName, &metricParser, ottl.WithConditionConverter(convertMetricConditions))(pc)
	}
}

func WithDataPointParser(functions map[string]ottl.Factory[*ottldatapoint.TransformContext]) MetricParserCollectionOption {
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

func WithMetricCommonParsers(functions map[string]ottl.Factory[*ottlresource.TransformContext]) MetricParserCollectionOption {
	return MetricParserCollectionOption(withCommonParsers[MetricsConsumer](functions))
}

func NewMetricParserCollection(settings component.TelemetrySettings, options ...MetricParserCollectionOption) (*MetricParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[MetricsConsumer]{
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

func convertMetricConditions(pc *ottl.ParserCollection[MetricsConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottlmetric.TransformContext]) (MetricsConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	mConditions := ottlmetric.NewConditionSequence(parsedConditions, pc.Settings, ottlmetric.WithConditionSequenceErrorMode(errorMode))
	return metricConditions{&mConditions}, nil
}

func convertDataPointConditions(pc *ottl.ParserCollection[MetricsConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottldatapoint.TransformContext]) (MetricsConsumer, error) {
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
	if contextConditions.Context != "" {
		return pc.ParseConditionsWithContext(string(contextConditions.Context), contextConditions, true)
	}
	return pc.ParseConditions(contextConditions)
}
