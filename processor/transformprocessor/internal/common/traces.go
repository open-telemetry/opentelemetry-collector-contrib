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
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				tCtx := ottlspan.NewTransformContext(spans.At(k), sspans.Scope(), rspans.Resource())
				for _, statement := range t {
					_, _, err := statement.Execute(ctx, tCtx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ consumer.Traces = &spanEventStatements{}

type spanEventStatements []*ottl.Statement[ottlspanevent.TransformContext]

func (s spanEventStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (s spanEventStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanEvents := span.Events()
				for n := 0; n < spanEvents.Len(); n++ {
					tCtx := ottlspanevent.NewTransformContext(spanEvents.At(n), span, sspans.Scope(), rspans.Resource())
					for _, statement := range s {
						_, _, err := statement.Execute(ctx, tCtx)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
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
