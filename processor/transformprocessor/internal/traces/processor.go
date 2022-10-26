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
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
)

type Processor struct {
	statements []*ottl.Statement[ottltraces.TransformContext]
}

func NewProcessor(statements []string, settings component.TelemetrySettings) (*Processor, error) {
	ottlp := ottltraces.NewParser(Functions(), settings)
	parsedStatements, err := ottlp.ParseStatements(statements)
	if err != nil {
		return nil, err
	}
	return &Processor{
		statements: parsedStatements,
	}, nil
}

func (p *Processor) ProcessTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspan := rspans.ScopeSpans().At(j)
			spans := sspan.Spans()
			for k := 0; k < spans.Len(); k++ {
				ctx := ottltraces.NewTransformContext(spans.At(k), sspan.Scope(), rspans.Resource())
				for _, statement := range p.statements {
					_, _, err := statement.Execute(ctx)
					if err != nil {
						return td, err
					}
				}
			}
		}
	}
	return td, nil
}
