// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	xprofilefuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xspanlink/ottlspanlink"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	xottlfuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

type TracesConsumer interface {
	Context() ContextID
	ConsumeTraces(ctx context.Context, td ptrace.Traces, cache *pcommon.Map) error
}

type traceStatements struct {
	ottl.StatementSequence[*ottlspan.TransformContext]
	expr.BoolExpr[*ottlspan.TransformContext]
}

func (traceStatements) Context() ContextID {
	return Span
}

func (t traceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces, cache *pcommon.Map) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				tCtx := ottlspan.NewTransformContext(rspans, sspans, spans.At(k), ottlspan.WithCache(cache))
				condition, err := t.Eval(ctx, tCtx)
				if err != nil {
					tCtx.Close()
					return err
				}
				if condition {
					err = t.Execute(ctx, tCtx)
					if err != nil {
						tCtx.Close()
						return err
					}
				}
				tCtx.Close()
			}
		}
	}
	return nil
}

type spanEventStatements struct {
	ottl.StatementSequence[*ottlspanevent.TransformContext]
	expr.BoolExpr[*ottlspanevent.TransformContext]
}

func (spanEventStatements) Context() ContextID {
	return SpanEvent
}

func (s spanEventStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces, cache *pcommon.Map) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanEvents := span.Events()
				for n := 0; n < spanEvents.Len(); n++ {
					tCtx := ottlspanevent.NewTransformContext(rspans, sspans, span, spanEvents.At(n), ottlspanevent.WithCache(cache))
					condition, err := s.Eval(ctx, tCtx)
					if err != nil {
						tCtx.Close()
						return err
					}
					if condition {
						err = s.Execute(ctx, tCtx)
						if err != nil {
							tCtx.Close()
							return err
						}
					}
					tCtx.Close()
				}
			}
		}
	}
	return nil
}

type spanLinkStatements struct {
	ottl.StatementSequence[*ottlspanlink.TransformContext]
	expr.BoolExpr[*ottlspanlink.TransformContext]
}

func (spanLinkStatements) Context() ContextID {
	return SpanLink
}

func (s spanLinkStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces, cache *pcommon.Map) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanLinks := span.Links()
				for n := 0; n < spanLinks.Len(); n++ {
					tCtx := ottlspanlink.NewTransformContext(rspans, sspans, span, spanLinks.At(n), ottlspanlink.WithCache(cache))
					condition, err := s.Eval(ctx, tCtx)
					if err != nil {
						tCtx.Close()
						return err
					}
					if condition {
						err = s.Execute(ctx, tCtx)
						if err != nil {
							tCtx.Close()
							return err
						}
					}
					tCtx.Close()
				}
			}
		}
	}
	return nil
}

type TraceParserCollection ottl.ParserCollection[TracesConsumer]

type TraceParserCollectionOption ottl.ParserCollectionOption[TracesConsumer]

func WithSpanParser(functions map[string]ottl.Factory[*ottlspan.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspan.NewParser(functions, pc.Settings(), ottlspan.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspan.ContextName, &parser, ottl.WithStatementConverter[*ottlspan.TransformContext, TracesConsumer](convertSpanStatements))(pc)
	}
}

func WithSpanEventParser(functions map[string]ottl.Factory[*ottlspanevent.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspanevent.NewParser(functions, pc.Settings(), ottlspanevent.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspanevent.ContextName, &parser, ottl.WithStatementConverter(convertSpanEventStatements))(pc)
	}
}

// WithSpanLinkParser registers the experimental spanlink context. The functions available in it are not configurable.
func WithSpanLinkParser(functions map[string]ottl.Factory[*ottlspanlink.TransformContext]) TraceParserCollectionOption {
	return func(pc *ottl.ParserCollection[TracesConsumer]) error {
		parser, err := ottlspanlink.NewParser(functions, pc.Settings(), ottlspanlink.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlspanlink.ContextName, &parser, ottl.WithStatementConverter(convertSpanLinkStatements))(pc)
	}
}

func WithTraceErrorMode(errorMode ottl.ErrorMode) TraceParserCollectionOption {
	return TraceParserCollectionOption(ottl.WithParserCollectionErrorMode[TracesConsumer](errorMode))
}

func NewTraceParserCollection(settings component.TelemetrySettings, options ...TraceParserCollectionOption) (*TraceParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[TracesConsumer]{
		withCommonContextParsers[TracesConsumer](),
		ottl.EnableParserCollectionModifiedPathsLogging[TracesConsumer](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[TracesConsumer](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	tpc := TraceParserCollection(*pc)
	return &tpc, nil
}

func convertSpanStatements(pc *ottl.ParserCollection[TracesConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[*ottlspan.TransformContext]) (TracesConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode()
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[*ottlspan.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlspan.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForSpanWithOptions, contextStatements.Conditions, errorMode, pc.Settings(), filterottl.StandardSpanFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	sStatements := ottlspan.NewStatementSequence(parsedStatements, pc.Settings(), ottlspan.WithStatementSequenceErrorMode(errorMode))
	return traceStatements{sStatements, globalExpr}, nil
}

func convertSpanEventStatements(pc *ottl.ParserCollection[TracesConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[*ottlspanevent.TransformContext]) (TracesConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode()
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[*ottlspanevent.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlspanevent.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForSpanEventWithOptions, contextStatements.Conditions, errorMode, pc.Settings(), filterottl.StandardSpanEventFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	seStatements := ottlspanevent.NewStatementSequence(parsedStatements, pc.Settings(), ottlspanevent.WithStatementSequenceErrorMode(errorMode))
	return spanEventStatements{seStatements, globalExpr}, nil
}

func convertSpanLinkStatements(pc *ottl.ParserCollection[TracesConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[*ottlspanlink.TransformContext]) (TracesConsumer, error) {
	gate := metadata.ProcessorTransformEnableExperimentalFunctionsFeatureGate
	if !gate.IsEnabled() {
		return nil, fmt.Errorf("the spanlink context is experimental and requires the `%s` feature gate to be enabled", gate.ID())
	}
	pc.Settings().Logger.Warn("transform processor configuration uses the experimental spanlink context that is not covered by stability guarantees; its name, behavior, and fields may change or be removed")
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode()
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[*ottlspanlink.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlspanlink.EnablePathContextNames())
	}
	standardFuncs := xottlfuncs.WithExperimentalConverters(xprofilefuncs.WithProfileConverters(ottlfuncs.StandardConverters[*ottlspanlink.TransformContext]()))
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(newBoolExprForSpanLink, contextStatements.Conditions, errorMode, pc.Settings(), standardFuncs, parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	slStatements := ottlspanlink.NewStatementSequence(parsedStatements, pc.Settings(), ottlspanlink.WithStatementSequenceErrorMode(errorMode))
	return spanLinkStatements{slStatements, globalExpr}, nil
}

// newBoolExprForSpanLink is kept local, not in internal/filter/filterottl, to avoid adding the
// experimental xspanlink dependency to every module that depends on internal/filter.
func newBoolExprForSpanLink(conditions []string, functions map[string]ottl.Factory[*ottlspanlink.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlspanlink.TransformContext]) (*ottl.ConditionSequence[*ottlspanlink.TransformContext], error) {
	parser, err := ottlspanlink.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	parsed, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlspanlink.NewConditionSequence(parsed, set, ottlspanlink.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}

func (tpc *TraceParserCollection) ParseContextStatements(contextStatements ContextStatements) (TracesConsumer, error) {
	pc := ottl.ParserCollection[TracesConsumer](*tpc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements, ottl.WithContextInferenceConditions(contextStatements.Conditions))
}
