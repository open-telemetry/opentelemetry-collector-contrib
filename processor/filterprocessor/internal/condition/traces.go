// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package condition // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

type TracesConsumer struct {
	resourceExpr  expr.BoolExpr[*ottlresource.TransformContext]
	scopeExpr     expr.BoolExpr[*ottlscope.TransformContext]
	spanExpr      expr.BoolExpr[*ottlspan.TransformContext]
	spanEventExpr expr.BoolExpr[*ottlspanevent.TransformContext]
}

// parsedTraceConditions is the type R for ParserCollection[R] that holds parsed OTTL conditions
type parsedTraceConditions struct {
	resourceConditions  []*ottl.Condition[*ottlresource.TransformContext]
	scopeConditions     []*ottl.Condition[*ottlscope.TransformContext]
	spanConditions      []*ottl.Condition[*ottlspan.TransformContext]
	spanEventConditions []*ottl.Condition[*ottlspanevent.TransformContext]
	telemetrySettings   component.TelemetrySettings
	errorMode           ottl.ErrorMode
	action              Action
}

func (tc TracesConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var condErr error
	td.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		if tc.resourceExpr != nil {
			rCtx := ottlresource.NewTransformContextPtr(rs.Resource(), rs)
			rCond, rErr := tc.resourceExpr.Eval(ctx, rCtx)
			rCtx.Close()
			if rErr != nil {
				condErr = multierr.Append(condErr, rErr)
				return false
			}
			if rCond {
				return true
			}
		}

		if tc.scopeExpr == nil && tc.spanExpr == nil && tc.spanEventExpr == nil {
			return rs.ScopeSpans().Len() == 0
		}

		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			if tc.scopeExpr != nil {
				sCtx := ottlscope.NewTransformContextPtr(ss.Scope(), rs.Resource(), ss, rs)
				sCond, sErr := tc.scopeExpr.Eval(ctx, sCtx)
				sCtx.Close()
				if sErr != nil {
					condErr = multierr.Append(condErr, sErr)
					return false
				}
				if sCond {
					return true
				}
			}

			if tc.spanExpr == nil && tc.spanEventExpr == nil {
				return ss.Spans().Len() == 0
			}

			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				if tc.spanExpr != nil {
					spanCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
					spanCond, err := tc.spanExpr.Eval(ctx, spanCtx)
					spanCtx.Close()
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					if spanCond {
						return true
					}
				}

				if tc.spanEventExpr != nil {
					span.Events().RemoveIf(func(spanEvent ptrace.SpanEvent) bool {
						seCtx := ottlspanevent.NewTransformContextPtr(rs, ss, span, spanEvent)
						seCond, err := tc.spanEventExpr.Eval(ctx, seCtx)
						seCtx.Close()
						if err != nil {
							condErr = multierr.Append(condErr, err)
							return false
						}
						return seCond
					})
				}
				return false
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
	if td.ResourceSpans().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func newTraceConditionsFromResource(rc []*ottl.Condition[*ottlresource.TransformContext], telemetrySettings component.TelemetrySettings, errorMode ottl.ErrorMode, action Action) parsedTraceConditions {
	return parsedTraceConditions{
		resourceConditions: rc,
		telemetrySettings:  telemetrySettings,
		errorMode:          errorMode,
		action:             action,
	}
}

func newTraceConditionsFromScope(sc []*ottl.Condition[*ottlscope.TransformContext], telemetrySettings component.TelemetrySettings, errorMode ottl.ErrorMode, action Action) parsedTraceConditions {
	return parsedTraceConditions{
		scopeConditions:   sc,
		telemetrySettings: telemetrySettings,
		errorMode:         errorMode,
		action:            action,
	}
}

func newTracesConsumer(tc *parsedTraceConditions) TracesConsumer {
	var rExpr expr.BoolExpr[*ottlresource.TransformContext]
	var sExpr expr.BoolExpr[*ottlscope.TransformContext]
	var spanExpr expr.BoolExpr[*ottlspan.TransformContext]
	var spanEventExpr expr.BoolExpr[*ottlspanevent.TransformContext]

	if len(tc.resourceConditions) > 0 {
		cs := ottlresource.NewConditionSequence(tc.resourceConditions, tc.telemetrySettings, ottlresource.WithConditionSequenceErrorMode(tc.errorMode))
		rExpr = &cs
	}

	if len(tc.scopeConditions) > 0 {
		cs := ottlscope.NewConditionSequence(tc.scopeConditions, tc.telemetrySettings, ottlscope.WithConditionSequenceErrorMode(tc.errorMode))
		sExpr = &cs
	}

	if len(tc.spanConditions) > 0 {
		cs := ottlspan.NewConditionSequence(tc.spanConditions, tc.telemetrySettings, ottlspan.WithConditionSequenceErrorMode(tc.errorMode))
		spanExpr = &cs
	}

	if len(tc.spanEventConditions) > 0 {
		cs := ottlspanevent.NewConditionSequence(tc.spanEventConditions, tc.telemetrySettings, ottlspanevent.WithConditionSequenceErrorMode(tc.errorMode))
		spanEventExpr = &cs
	}

	if tc.action == ActionKeep {
		if rExpr != nil {
			rExpr = expr.Not(rExpr)
		}
		if sExpr != nil {
			sExpr = expr.Not(sExpr)
		}
		if spanExpr != nil {
			spanExpr = expr.Not(spanExpr)
		}
		if spanEventExpr != nil {
			spanEventExpr = expr.Not(spanEventExpr)
		}
	}

	return TracesConsumer{
		resourceExpr:  rExpr,
		scopeExpr:     sExpr,
		spanExpr:      spanExpr,
		spanEventExpr: spanEventExpr,
	}
}

type TraceParserCollection struct {
	ottl.ParserCollection[parsedTraceConditions]
	action Action
}

type TraceParserCollectionOption func(*TraceParserCollection) error

func WithSpanParser(functions map[string]ottl.Factory[*ottlspan.TransformContext]) TraceParserCollectionOption {
	return func(tpc *TraceParserCollection) error {
		parser, err := ottlspan.NewParser(functions, tpc.Settings, ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspan.ContextName, &parser, ottl.WithConditionConverter(convertSpanConditions))(&tpc.ParserCollection)
	}
}

func WithSpanEventParser(functions map[string]ottl.Factory[*ottlspanevent.TransformContext]) TraceParserCollectionOption {
	return func(tpc *TraceParserCollection) error {
		parser, err := ottlspanevent.NewParser(functions, tpc.Settings, ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspanevent.ContextName, &parser, ottl.WithConditionConverter(convertSpanEventConditions))(&tpc.ParserCollection)
	}
}

func WithTraceErrorMode(errorMode ottl.ErrorMode) TraceParserCollectionOption {
	return func(tpc *TraceParserCollection) error {
		return ottl.WithParserCollectionErrorMode[parsedTraceConditions](errorMode)(&tpc.ParserCollection)
	}
}

func WithTraceAction(action Action) TraceParserCollectionOption {
	return func(tpc *TraceParserCollection) error {
		tpc.action = action
		return nil
	}
}

func WithTraceCommonParsers(functions map[string]ottl.Factory[*ottlresource.TransformContext]) TraceParserCollectionOption {
	return func(tpc *TraceParserCollection) error {
		return withCommonParsers(functions, newTraceConditionsFromResource, newTraceConditionsFromScope)(&tpc.ParserCollection)
	}
}

func NewTraceParserCollection(settings component.TelemetrySettings, options ...TraceParserCollectionOption) (*TraceParserCollection, error) {
	pc, err := ottl.NewParserCollection(settings, ottl.EnableParserCollectionModifiedPathsLogging[parsedTraceConditions](true))
	if err != nil {
		return nil, err
	}

	tpc := &TraceParserCollection{
		ParserCollection: *pc,
	}

	for _, option := range options {
		if err := option(tpc); err != nil {
			return nil, err
		}
	}

	return tpc, nil
}

func convertSpanConditions(pc *ottl.ParserCollection[parsedTraceConditions], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottlspan.TransformContext]) (parsedTraceConditions, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return parsedTraceConditions{}, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	return parsedTraceConditions{
		spanConditions:    parsedConditions,
		telemetrySettings: pc.Settings,
		errorMode:         errorMode,
		action:            contextConditions.Action,
	}, nil
}

func convertSpanEventConditions(pc *ottl.ParserCollection[parsedTraceConditions], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[*ottlspanevent.TransformContext]) (parsedTraceConditions, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return parsedTraceConditions{}, err
	}
	errorMode := getErrorMode(pc, contextConditions)
	return parsedTraceConditions{
		spanEventConditions: parsedConditions,
		telemetrySettings:   pc.Settings,
		errorMode:           errorMode,
		action:              contextConditions.Action,
	}, nil
}

func (tpc *TraceParserCollection) ParseContextConditions(contextConditions ContextConditions) (TracesConsumer, error) {
	pc := &tpc.ParserCollection

	if contextConditions.Action == "" {
		contextConditions.Action = tpc.action
	}

	if contextConditions.Context != "" {
		tc, err := pc.ParseConditionsWithContext(string(contextConditions.Context), contextConditions, true)
		if err != nil {
			return TracesConsumer{}, err
		}
		return newTracesConsumer(&tc), nil
	}

	var rConditions []*ottl.Condition[*ottlresource.TransformContext]
	var sConditions []*ottl.Condition[*ottlscope.TransformContext]
	var spanConditions []*ottl.Condition[*ottlspan.TransformContext]
	var spanEventConditions []*ottl.Condition[*ottlspanevent.TransformContext]

	for _, cc := range contextConditions.GetConditions() {
		tc, err := pc.ParseConditions(ContextConditions{Conditions: []string{cc}})
		if err != nil {
			return TracesConsumer{}, err
		}

		if len(tc.resourceConditions) > 0 {
			rConditions = append(rConditions, tc.resourceConditions...)
		}
		if len(tc.scopeConditions) > 0 {
			sConditions = append(sConditions, tc.scopeConditions...)
		}
		if len(tc.spanConditions) > 0 {
			spanConditions = append(spanConditions, tc.spanConditions...)
		}
		if len(tc.spanEventConditions) > 0 {
			spanEventConditions = append(spanEventConditions, tc.spanEventConditions...)
		}
	}

	aggregatedConditions := parsedTraceConditions{
		resourceConditions:  rConditions,
		scopeConditions:     sConditions,
		spanConditions:      spanConditions,
		spanEventConditions: spanEventConditions,
		telemetrySettings:   pc.Settings,
		errorMode:           getErrorMode[parsedTraceConditions](pc, &contextConditions),
		action:              contextConditions.Action,
	}

	return newTracesConsumer(&aggregatedConditions), nil
}
