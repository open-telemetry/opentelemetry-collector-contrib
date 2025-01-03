// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

var _ consumer.Metrics = &metricStatements{}

type metricStatements struct {
	ottl.StatementSequence[ottlmetric.TransformContext]
	expr.BoolExpr[ottlmetric.TransformContext]
}

func (m metricStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (m metricStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				tCtx := ottlmetric.NewTransformContextWithCache(metrics.At(k), smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, newCacheFrom(ctx))
				condition, err := m.BoolExpr.Eval(ctx, tCtx)
				if err != nil {
					return err
				}
				if condition {
					err := m.Execute(ctx, tCtx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ consumer.Metrics = &dataPointStatements{}

type dataPointStatements struct {
	ottl.StatementSequence[ottldatapoint.TransformContext]
	expr.BoolExpr[ottldatapoint.TransformContext]
}

func (d dataPointStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (d dataPointStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				var err error
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					err = d.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
				case pmetric.MetricTypeGauge:
					err = d.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
				case pmetric.MetricTypeHistogram:
					err = d.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
				case pmetric.MetricTypeExponentialHistogram:
					err = d.handleExponetialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
				case pmetric.MetricTypeSummary:
					err = d.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContextWithCache(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, newCacheFrom(ctx))
		condition, err := d.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := d.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContextWithCache(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, newCacheFrom(ctx))
		condition, err := d.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := d.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleExponetialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContextWithCache(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, newCacheFrom(ctx))
		condition, err := d.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := d.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContextWithCache(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, newCacheFrom(ctx))
		condition, err := d.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := d.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type MetricParserCollection ottl.ParserCollection[consumer.Metrics]

type MetricParserCollectionOption ottl.ParserCollectionOption[consumer.Metrics]

func WithMetricParser(functions map[string]ottl.Factory[ottlmetric.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[consumer.Metrics]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlmetric.ContextName, &metricParser, convertMetricStatements)(pc)
	}
}

func WithDataPointParser(functions map[string]ottl.Factory[ottldatapoint.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[consumer.Metrics]) error {
		dataPointParser, err := ottldatapoint.NewParser(functions, pc.Settings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottldatapoint.ContextName, &dataPointParser, convertDataPointStatements)(pc)
	}
}

func WithMetricErrorMode(errorMode ottl.ErrorMode) MetricParserCollectionOption {
	return MetricParserCollectionOption(ottl.WithParserCollectionErrorMode[consumer.Metrics](errorMode))
}

func NewMetricParserCollection(settings component.TelemetrySettings, options ...MetricParserCollectionOption) (*MetricParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[consumer.Metrics]{
		withCommonContextParsers[consumer.Metrics](),
		ottl.EnableParserCollectionModifiedStatementLogging[consumer.Metrics](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[consumer.Metrics](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	mpc := MetricParserCollection(*pc)
	return &mpc, nil
}

func convertMetricStatements(pc *ottl.ParserCollection[consumer.Metrics], _ *ottl.Parser[ottlmetric.TransformContext], _ string, statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottlmetric.TransformContext]) (consumer.Metrics, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForMetric, contextStatements.Conditions, pc.ErrorMode, pc.Settings, filterottl.StandardMetricFuncs())
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	mStatements := ottlmetric.NewStatementSequence(parsedStatements, pc.Settings, ottlmetric.WithStatementSequenceErrorMode(pc.ErrorMode))
	return metricStatements{mStatements, globalExpr}, nil
}

func convertDataPointStatements(pc *ottl.ParserCollection[consumer.Metrics], _ *ottl.Parser[ottldatapoint.TransformContext], _ string, statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottldatapoint.TransformContext]) (consumer.Metrics, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForDataPoint, contextStatements.Conditions, pc.ErrorMode, pc.Settings, filterottl.StandardDataPointFuncs())
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	dpStatements := ottldatapoint.NewStatementSequence(parsedStatements, pc.Settings, ottldatapoint.WithStatementSequenceErrorMode(pc.ErrorMode))
	return dataPointStatements{dpStatements, globalExpr}, nil
}

func (mpc *MetricParserCollection) ParseContextStatements(contextStatements ContextStatements) (consumer.Metrics, error) {
	pc := ottl.ParserCollection[consumer.Metrics](*mpc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements)
}
