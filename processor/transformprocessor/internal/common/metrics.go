// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type MetricsConsumer interface {
	Context() ContextID
	ConsumeMetrics(ctx context.Context, md pmetric.Metrics, cache *pcommon.Map) error
}

type metricStatements struct {
	ottl.StatementSequence[ottlmetric.TransformContext]
	expr.BoolExpr[ottlmetric.TransformContext]
}

func (m metricStatements) Context() ContextID {
	return Metric
}

func (m metricStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics, cache *pcommon.Map) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				tCtx := ottlmetric.NewTransformContext(metrics.At(k), smetrics.Metrics(), smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, ottlmetric.WithCache(cache))
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

type dataPointStatements struct {
	ottl.StatementSequence[ottldatapoint.TransformContext]
	expr.BoolExpr[ottldatapoint.TransformContext]
}

func (d dataPointStatements) Context() ContextID {
	return DataPoint
}

func (d dataPointStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics, cache *pcommon.Map) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				transformContextOptions := []ottldatapoint.TransformContextOption{ottldatapoint.WithCache(cache)}
				var err error
				//exhaustive:enforce
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					err = d.handleNumberDataPoints(ctx, metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, transformContextOptions)
				case pmetric.MetricTypeGauge:
					err = d.handleNumberDataPoints(ctx, metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, transformContextOptions)
				case pmetric.MetricTypeHistogram:
					err = d.handleHistogramDataPoints(ctx, metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, transformContextOptions)
				case pmetric.MetricTypeExponentialHistogram:
					err = d.handleExponentialHistogramDataPoints(ctx, metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, transformContextOptions)
				case pmetric.MetricTypeSummary:
					err = d.handleSummaryDataPoints(ctx, metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource(), smetrics, rmetrics, transformContextOptions)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d dataPointStatements) handleNumberDataPoints(ctx context.Context, dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics, options []ottldatapoint.TransformContextOption) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, options...)
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

func (d dataPointStatements) handleHistogramDataPoints(ctx context.Context, dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics, options []ottldatapoint.TransformContextOption) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, options...)
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

func (d dataPointStatements) handleExponentialHistogramDataPoints(ctx context.Context, dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics, options []ottldatapoint.TransformContextOption) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, options...)
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

func (d dataPointStatements) handleSummaryDataPoints(ctx context.Context, dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource, scopeMetrics pmetric.ScopeMetrics, resourceMetrics pmetric.ResourceMetrics, options []ottldatapoint.TransformContextOption) error {
	for i := 0; i < dps.Len(); i++ {
		tCtx := ottldatapoint.NewTransformContext(dps.At(i), metric, metrics, is, resource, scopeMetrics, resourceMetrics, options...)
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

type MetricParserCollection ottl.ParserCollection[MetricsConsumer]

type MetricParserCollectionOption ottl.ParserCollectionOption[MetricsConsumer]

func WithMetricParser(functions map[string]ottl.Factory[ottlmetric.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[MetricsConsumer]) error {
		metricParser, err := ottlmetric.NewParser(functions, pc.Settings, ottlmetric.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlmetric.ContextName, &metricParser, ottl.WithStatementConverter(convertMetricStatements))(pc)
	}
}

func WithDataPointParser(functions map[string]ottl.Factory[ottldatapoint.TransformContext]) MetricParserCollectionOption {
	return func(pc *ottl.ParserCollection[MetricsConsumer]) error {
		dataPointParser, err := ottldatapoint.NewParser(functions, pc.Settings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottldatapoint.ContextName, &dataPointParser, ottl.WithStatementConverter(convertDataPointStatements))(pc)
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

func convertMetricStatements(pc *ottl.ParserCollection[MetricsConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottlmetric.TransformContext]) (MetricsConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottlmetric.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlmetric.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForMetricWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardMetricFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	mStatements := ottlmetric.NewStatementSequence(parsedStatements, pc.Settings, ottlmetric.WithStatementSequenceErrorMode(errorMode))
	return metricStatements{mStatements, globalExpr}, nil
}

func convertDataPointStatements(pc *ottl.ParserCollection[MetricsConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottldatapoint.TransformContext]) (MetricsConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottldatapoint.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottldatapoint.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForDataPointWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardDataPointFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	dpStatements := ottldatapoint.NewStatementSequence(parsedStatements, pc.Settings, ottldatapoint.WithStatementSequenceErrorMode(errorMode))
	return dataPointStatements{dpStatements, globalExpr}, nil
}

func (mpc *MetricParserCollection) ParseContextStatements(contextStatements ContextStatements) (MetricsConsumer, error) {
	pc := ottl.ParserCollection[MetricsConsumer](*mpc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements)
}
