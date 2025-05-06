// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

var _ baseContext = &resourceConditions{}

type resourceConditions struct {
	expr.BoolExpr[ottlresource.TransformContext]
}

func (r resourceConditions) Context() ContextID {
	return Resource
}

func (r resourceConditions) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		tCtx := ottlresource.NewTransformContext(rspans.Resource(), rspans)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			condErr = multierr.Append(condErr, err)
			return false
		}
		return condition
	})
	if td.ResourceSpans().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func (r resourceConditions) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var condErr error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		tCtx := ottlresource.NewTransformContext(rmetrics.Resource(), rmetrics)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			condErr = multierr.Append(condErr, err)
			return false
		}
		return condition
	})
	if md.ResourceMetrics().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func (r resourceConditions) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var condErr error
	ld.ResourceLogs().RemoveIf(func(rlogs plog.ResourceLogs) bool {
		tCtx := ottlresource.NewTransformContext(rlogs.Resource(), rlogs)
		condition, err := r.Eval(ctx, tCtx)
		if err != nil {
			condErr = multierr.Append(condErr, err)
			return false
		}
		return condition
	})
	if ld.ResourceLogs().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

var _ baseContext = &scopeConditions{}

type scopeConditions struct {
	expr.BoolExpr[ottlscope.TransformContext]
}

func (s scopeConditions) Context() ContextID {
	return Scope
}

func (s scopeConditions) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
			tCtx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource(), sspans)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				condErr = multierr.Append(condErr, err)
				return false
			}
			return condition
		})
		return rspans.ScopeSpans().Len() == 0
	})
	if td.ResourceSpans().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func (s scopeConditions) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var condErr error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
			tCtx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource(), smetrics)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				condErr = multierr.Append(condErr, err)
				return false
			}
			return condition
		})
		return rmetrics.ScopeMetrics().Len() == 0
	})
	if md.ResourceMetrics().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func (s scopeConditions) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var condErr error
	ld.ResourceLogs().RemoveIf(func(rlogs plog.ResourceLogs) bool {
		rlogs.ScopeLogs().RemoveIf(func(slogs plog.ScopeLogs) bool {
			tCtx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource(), slogs)
			condition, err := s.Eval(ctx, tCtx)
			if err != nil {
				condErr = multierr.Append(condErr, err)
				return false
			}
			return condition
		})
		return rlogs.ScopeLogs().Len() == 0
	})
	if ld.ResourceLogs().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
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

		err = ottl.WithParserCollectionContext(ottlresource.ContextName, &rp, ottl.WithConditionConverter[ottlresource.TransformContext, R](parseResourceContextConditions))(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(ottlscope.ContextName, &sp, ottl.WithConditionConverter[ottlscope.TransformContext, R](parseScopeContextConditions))(pc)
		if err != nil {
			return err
		}

		return nil
	}
}

func parseResourceContextConditions[R any](
	pc *ottl.ParserCollection[R],
	conditions ottl.ConditionsGetter,
	parsedConditions []*ottl.Condition[ottlresource.TransformContext],
) (R, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return *new(R), err
	}
	errorMode := getErrorMode(pc, contextConditions)
	rConditions := ottlresource.NewConditionSequence(parsedConditions, pc.Settings, ottlresource.WithConditionSequenceErrorMode(errorMode))
	result := (baseContext)(resourceConditions{&rConditions})
	return result.(R), nil
}

func parseScopeContextConditions[R any](
	pc *ottl.ParserCollection[R],
	conditions ottl.ConditionsGetter,
	parsedConditions []*ottl.Condition[ottlscope.TransformContext],
) (R, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return *new(R), err
	}
	errorMode := getErrorMode(pc, contextConditions)
	sConditions := ottlscope.NewConditionSequence(parsedConditions, pc.Settings, ottlscope.WithConditionSequenceErrorMode(errorMode))
	result := (baseContext)(scopeConditions{&sConditions})
	return result.(R), nil
}
