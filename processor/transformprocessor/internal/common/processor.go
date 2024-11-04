// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

var _ consumer.Traces = &resourceStatements{}
var _ consumer.Metrics = &resourceStatements{}
var _ consumer.Logs = &resourceStatements{}
var _ baseContext = &resourceStatements{}

type resourceStatements struct {
	ottl.StatementSequence[ottlresource.TransformContext]
	expr.BoolExpr[ottlresource.TransformContext]
}

func (r resourceStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (r resourceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		tCtx := ottlresource.NewTransformContext(rspans.Resource(), rspans)
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

func (r resourceStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		tCtx := ottlresource.NewTransformContext(rmetrics.Resource(), rmetrics)
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

func (r resourceStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		tCtx := ottlresource.NewTransformContext(rlogs.Resource(), rlogs)
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

var _ consumer.Traces = &scopeStatements{}
var _ consumer.Metrics = &scopeStatements{}
var _ consumer.Logs = &scopeStatements{}
var _ baseContext = &scopeStatements{}

type scopeStatements struct {
	ottl.StatementSequence[ottlscope.TransformContext]
	expr.BoolExpr[ottlscope.TransformContext]
}

func (s scopeStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (s scopeStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			tCtx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource(), sspans)
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

func (s scopeStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rmetrics := md.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			tCtx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource(), smetrics)
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

func (s scopeStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rlogs := ld.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			tCtx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource(), slogs)
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
	consumer.Traces
	consumer.Metrics
	consumer.Logs
}

func withCommonContextParsers[R any]() ottl.ParserCollectionOption[ContextStatements, R] {
	return func(pc *ottl.ParserCollection[ContextStatements, R]) error {
		rp, err := ottlresource.NewParser(ResourceFunctions(), pc.Settings, ottlresource.WithPathContextNames())
		if err != nil {
			return err
		}
		sp, err := ottlscope.NewParser(ScopeFunctions(), pc.Settings, ottlscope.WithPathContextNames())
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext[ottlresource.TransformContext, ContextStatements, R](ottlresource.PathContextName, &rp, parseResourceContextStatements)(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext[ottlscope.TransformContext, ContextStatements, R](ottlscope.PathContextName, &sp, parseScopeContextStatements)(pc)
		if err != nil {
			return err
		}

		return nil
	}
}

func parseResourceContextStatements[R any](
	collection *ottl.ParserCollection[ContextStatements, R],
	_ *ottl.Parser[ottlresource.TransformContext],
	_ string,
	statements ContextStatements,
	parsedStatements []*ottl.Statement[ottlresource.TransformContext],
) (R, error) {
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForResource, statements.Conditions, collection.ErrorMode, collection.Settings, filterottl.StandardResourceFuncs())
	if errGlobalBoolExpr != nil {
		return *new(R), errGlobalBoolExpr
	}
	rStatements := ottlresource.NewStatementSequence(parsedStatements, collection.Settings, ottlresource.WithStatementSequenceErrorMode(collection.ErrorMode))
	result := (baseContext)(resourceStatements{rStatements, globalExpr})
	return result.(R), nil
}

func parseScopeContextStatements[R any](
	collection *ottl.ParserCollection[ContextStatements, R],
	_ *ottl.Parser[ottlscope.TransformContext],
	_ string,
	statements ContextStatements,
	parsedStatements []*ottl.Statement[ottlscope.TransformContext],
) (R, error) {
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForScope, statements.Conditions, collection.ErrorMode, collection.Settings, filterottl.StandardScopeFuncs())
	if errGlobalBoolExpr != nil {
		return *new(R), errGlobalBoolExpr
	}
	sStatements := ottlscope.NewStatementSequence(parsedStatements, collection.Settings, ottlscope.WithStatementSequenceErrorMode(collection.ErrorMode))
	result := (baseContext)(scopeStatements{sStatements, globalExpr})
	return result.(R), nil
}

func parseGlobalExpr[K any](
	boolExprFunc func([]string, map[string]ottl.Factory[K], ottl.ErrorMode, component.TelemetrySettings) (expr.BoolExpr[K], error),
	conditions []string,
	errorMode ottl.ErrorMode,
	settings component.TelemetrySettings,
	standardFuncs map[string]ottl.Factory[K]) (expr.BoolExpr[K], error) {

	if len(conditions) > 0 {
		return boolExprFunc(conditions, standardFuncs, errorMode, settings)
	}
	// By default, set the global expression to always true unless conditions are specified.
	return expr.AlwaysTrue[K](), nil
}
