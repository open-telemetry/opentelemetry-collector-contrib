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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
)

type TracesContext interface {
	ProcessTraces(td ptrace.Traces) error
}

var _ TracesContext = &traceStatements{}

type traceStatements []*ottl.Statement[ottltraces.TransformContext]

func (t traceStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				ctx := ottltraces.NewTransformContext(spans.At(k), sspans.Scope(), rspans.Resource())
				for _, statement := range t {
					_, _, err := statement.Execute(ctx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ TracesContext = &spanEventStatements{}

type spanEventStatements []*ottl.Statement[ottlspanevent.TransformContext]

func (s spanEventStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				spanEvents := span.Events()
				for n := 0; n < spanEvents.Len(); n++ {
					ctx := ottlspanevent.NewTransformContext(spanEvents.At(k), span, sspans.Scope(), rspans.Resource())
					for _, statement := range s {
						_, _, err := statement.Execute(ctx)
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
	traceParser     ottl.Parser[ottltraces.TransformContext]
	spanEventParser ottl.Parser[ottlspanevent.TransformContext]
}

func NewTraceParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) TraceParserCollection {
	return TraceParserCollection{
		parserCollection: parserCollection{
			settings:       settings,
			resourceParser: ottlresource.NewParser(ResourceFunctions(), settings),
			scopeParser:    ottlscope.NewParser(ScopeFunctions(), settings),
		},
		traceParser: ottltraces.NewParser(functions, settings),
	}
}

func (pc TraceParserCollection) ParseContextStatements(contextStatements []ContextStatements) ([]TracesContext, error) {
	contexts := make([]TracesContext, len(contextStatements))
	var errors error

	for i, s := range contextStatements {
		switch s.Context {
		case Trace:
			tStatements, err := pc.traceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = traceStatements(tStatements)
		case SpanEvent:
			seStatements, err := pc.spanEventParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = spanEventStatements(seStatements)
		default:
			statements, err := pc.parseCommonContextStatements(s)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = statements
		}
	}
	if errors != nil {
		return nil, errors
	}
	return contexts, nil
}
