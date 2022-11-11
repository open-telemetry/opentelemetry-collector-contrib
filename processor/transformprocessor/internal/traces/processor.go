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

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type Processor struct {
	contexts []consumer.Traces
	// Deprecated.  Use contexts instead
	statements []*ottl.Statement[ottlspan.TransformContext]
}

func NewProcessor(statements []string, contextStatements []common.ContextStatements, settings component.TelemetrySettings) (*Processor, error) {
	if len(statements) > 0 {
		ottlp := ottlspan.NewParser(SpanFunctions(), settings)
		parsedStatements, err := ottlp.ParseStatements(statements)
		if err != nil {
			return nil, err
		}
		return &Processor{
			statements: parsedStatements,
		}, nil
	}

	pc, err := common.NewTraceParserCollection(settings, common.WithSpanParser(SpanFunctions()), common.WithSpanEventParser(SpanEventFunctions()))
	if err != nil {
		return nil, err
	}

	contexts := make([]consumer.Traces, len(contextStatements))
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			return nil, err
		}
		contexts[i] = context
	}

	return &Processor{
		contexts: contexts,
	}, nil
}

func (p *Processor) ProcessTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	if len(p.statements) > 0 {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			rspans := td.ResourceSpans().At(i)
			for j := 0; j < rspans.ScopeSpans().Len(); j++ {
				sspan := rspans.ScopeSpans().At(j)
				spans := sspan.Spans()
				for k := 0; k < spans.Len(); k++ {
					tCtx := ottlspan.NewTransformContext(spans.At(k), sspan.Scope(), rspans.Resource())
					for _, statement := range p.statements {
						_, _, err := statement.Execute(ctx, tCtx)
						if err != nil {
							return td, err
						}
					}
				}
			}
		}
	} else {
		for _, c := range p.contexts {
			err := c.ConsumeTraces(ctx, td)
			if err != nil {
				return td, err
			}
		}
	}
	return td, nil
}
