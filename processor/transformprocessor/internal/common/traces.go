// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

var _ consumer.Traces = &traceStatements{}

type traceStatements []*ottl.Statement[ottlspan.TransformContext]

func (t traceStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (t traceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errors error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
			sspans.Spans().RemoveIf(func(span ptrace.Span) bool {
				tCtx := ottlspan.NewTransformContext(span, sspans.Scope(), rspans.Resource())
				remove, err := executeStatements(ctx, tCtx, t)
				if err != nil {
					errors = multierr.Append(errors, err)
					return false
				}
				return bool(remove)
			})
			return sspans.Spans().Len() == 0
		})
		return rspans.ScopeSpans().Len() == 0
	})
	return errors
}

var _ consumer.Traces = &spanEventStatements{}

type spanEventStatements []*ottl.Statement[ottlspanevent.TransformContext]

func (s spanEventStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (s spanEventStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errors error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
			sspans.Spans().RemoveIf(func(span ptrace.Span) bool {
				span.Events().RemoveIf(func(spanEvent ptrace.SpanEvent) bool {
					tCtx := ottlspanevent.NewTransformContext(spanEvent, span, sspans.Scope(), rspans.Resource())
					remove, err := executeStatements(ctx, tCtx, s)
					if err != nil {
						errors = multierr.Append(errors, err)
						return false
					}
					return bool(remove)
				})
				return false
			})
			return false
		})
		return false
	})
	return errors
}

type TraceParserCollection struct {
	parserCollection
	spanParser      ottl.Parser[ottlspan.TransformContext]
	spanEventParser ottl.Parser[ottlspanevent.TransformContext]
}

type TraceParserCollectionOption func(*TraceParserCollection) error

func WithSpanParser(functions map[string]interface{}) TraceParserCollectionOption {
	return func(tp *TraceParserCollection) error {
		tp.spanParser = ottlspan.NewParser(functions, tp.settings)
		return nil
	}
}

func WithSpanEventParser(functions map[string]interface{}) TraceParserCollectionOption {
	return func(tp *TraceParserCollection) error {
		tp.spanEventParser = ottlspanevent.NewParser(functions, tp.settings)
		return nil
	}
}

func NewTraceParserCollection(settings component.TelemetrySettings, options ...TraceParserCollectionOption) (*TraceParserCollection, error) {
	tpc := &TraceParserCollection{
		parserCollection: parserCollection{
			settings:       settings,
			resourceParser: ottlresource.NewParser(ResourceFunctions(), settings),
			scopeParser:    ottlscope.NewParser(ScopeFunctions(), settings),
		},
	}

	for _, op := range options {
		err := op(tpc)
		if err != nil {
			return nil, err
		}
	}

	return tpc, nil
}

func (pc TraceParserCollection) ParseContextStatements(contextStatements ContextStatements) (consumer.Traces, error) {
	switch contextStatements.Context {
	case Span:
		tStatements, err := pc.spanParser.ParseStatements(contextStatements.Statements)
		if err != nil {
			return nil, err
		}
		return traceStatements(tStatements), nil
	case SpanEvent:
		seStatements, err := pc.spanEventParser.ParseStatements(contextStatements.Statements)
		if err != nil {
			return nil, err
		}
		return spanEventStatements(seStatements), nil
	default:
		return pc.parseCommonContextStatements(contextStatements)
	}
}
