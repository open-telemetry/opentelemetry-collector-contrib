// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

var _ baseContext = &resourceStatements{}

type resourceStatements struct {
	ottl.StatementSequence[ottlresource.TransformContext]
	expr.BoolExpr[ottlresource.TransformContext]
}

func (r resourceStatements) Context() ContextID {
	return Resource
}

func (r resourceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces, cache *pcommon.Map) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		tCtx := ottlresource.NewTransformContext(rspans.Resource(), rspans, ottlresource.WithCache(cache))
		condition, err := r.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics, cache *pcommon.Map) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		tCtx := ottlresource.NewTransformContext(rmetrics.Resource(), rmetrics, ottlresource.WithCache(cache))
		condition, err := r.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ConsumeLogs(ctx context.Context, ld plog.Logs, cache *pcommon.Map) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		tCtx := ottlresource.NewTransformContext(rlogs.Resource(), rlogs, ottlresource.WithCache(cache))
		condition, err := r.BoolExpr.Eval(ctx, tCtx)
		if err != nil {
			return err
		}
		if condition {
			err := r.Execute(ctx, tCtx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var _ baseContext = &scopeStatements{}

type scopeStatements struct {
	ottl.StatementSequence[ottlscope.TransformContext]
	expr.BoolExpr[ottlscope.TransformContext]
}

func (s scopeStatements) Context() ContextID {
	return Scope
}

func (s scopeStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces, cache *pcommon.Map) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			tCtx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource(), sspans, ottlscope.WithCache(cache))
			condition, err := s.BoolExpr.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics, cache *pcommon.Map) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			tCtx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource(), smetrics, ottlscope.WithCache(cache))
			condition, err := s.BoolExpr.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ConsumeLogs(ctx context.Context, ld plog.Logs, cache *pcommon.Map) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			tCtx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource(), slogs, ottlscope.WithCache(cache))
			condition, err := s.BoolExpr.Eval(ctx, tCtx)
			if err != nil {
				return err
			}
			if condition {
				err := s.Execute(ctx, tCtx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type baseContext interface {
	TracesConsumer
	MetricsConsumer
	LogsConsumer
}

func withCommonContextParsers[R any]() ottl.ParserCollectionOption[R] {
	return func(pc *ottl.ParserCollection[R]) error {
		rp, err := ottlresource.NewParser(ResourceFunctions(), pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}
		sp, err := ottlscope.NewParser(ScopeFunctions(), pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext[ottlresource.TransformContext, R](ottlresource.ContextName, &rp, parseResourceContextStatements)(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext[ottlscope.TransformContext, R](ottlscope.ContextName, &sp, parseScopeContextStatements)(pc)
		if err != nil {
			return err
		}

		return nil
	}
}

func parseResourceContextStatements[R any](
	pc *ottl.ParserCollection[R],
	_ *ottl.Parser[ottlresource.TransformContext],
	_ string,
	statements ottl.StatementsGetter,
	parsedStatements []*ottl.Statement[ottlresource.TransformContext],
) (R, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return *new(R), err
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForResource, contextStatements.Conditions, pc.ErrorMode, pc.Settings, filterottl.StandardResourceFuncs())
	if errGlobalBoolExpr != nil {
		return *new(R), errGlobalBoolExpr
	}
	rStatements := ottlresource.NewStatementSequence(parsedStatements, pc.Settings, ottlresource.WithStatementSequenceErrorMode(pc.ErrorMode))
	result := (baseContext)(resourceStatements{rStatements, globalExpr})
	return result.(R), nil
}

func parseScopeContextStatements[R any](
	pc *ottl.ParserCollection[R],
	_ *ottl.Parser[ottlscope.TransformContext],
	_ string,
	statements ottl.StatementsGetter,
	parsedStatements []*ottl.Statement[ottlscope.TransformContext],
) (R, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return *new(R), err
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForScope, contextStatements.Conditions, pc.ErrorMode, pc.Settings, filterottl.StandardScopeFuncs())
	if errGlobalBoolExpr != nil {
		return *new(R), errGlobalBoolExpr
	}
	sStatements := ottlscope.NewStatementSequence(parsedStatements, pc.Settings, ottlscope.WithStatementSequenceErrorMode(pc.ErrorMode))
	result := (baseContext)(scopeStatements{sStatements, globalExpr})
	return result.(R), nil
}

func parseGlobalExpr[K any](
	boolExprFunc func([]string, map[string]ottl.Factory[K], ottl.ErrorMode, component.TelemetrySettings) (*ottl.ConditionSequence[K], error),
	conditions []string,
	errorMode ottl.ErrorMode,
	settings component.TelemetrySettings,
	standardFuncs map[string]ottl.Factory[K],
) (expr.BoolExpr[K], error) {
	if len(conditions) > 0 {
		return boolExprFunc(conditions, standardFuncs, errorMode, settings)
	}
	// By default, set the global expression to always true unless conditions are specified.
	return expr.AlwaysTrue[K](), nil
}
