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

package processor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/processor"

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

var _ TracesContext = &TraceStatements{}

type TracesContext interface {
	ProcessTraces(td ptrace.Traces) error
}

type TraceStatements struct {
	statements []*ottl.Statement[ottltraces.TransformContext]
}

func (t *TraceStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				ctx := ottltraces.NewTransformContext(spans.At(k), sspans.Scope(), rspans.Resource())
				for _, statement := range t.statements {
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

type TraceParserCollection struct {
	parserCollection
	traceParser ottl.Parser[ottltraces.TransformContext]
}

func NewTraceParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) TraceParserCollection {
	return TraceParserCollection{
		parserCollection: parserCollection{
			resourceParser: ottlresource.NewParser(common.ResourceFunctions(), settings),
			scopeParser:    ottlscope.NewParser(common.ScopeFunctions(), settings),
		},
		traceParser: ottltraces.NewParser(functions, settings),
	}
}

func (pc TraceParserCollection) ParseContextStatements(contextStatements []ContextStatements) ([]TracesContext, error) {
	contexts := make([]TracesContext, len(contextStatements))
	var errors error

	for i, s := range contextStatements {
		switch s.Context {
		case Resource:
			statements, err := pc.resourceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &ResourceStatements{
				Statements: statements,
			}
		case Scope:
			statements, err := pc.scopeParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &ScopeStatements{
				Statements: statements,
			}
		case Trace:
			statements, err := pc.traceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &TraceStatements{
				statements: statements,
			}
		default:
			errors = multierr.Append(errors, fmt.Errorf("context, %v, is not a valid context", s.Context))
		}
	}

	if errors != nil {
		return nil, errors
	}
	return contexts, nil
}
