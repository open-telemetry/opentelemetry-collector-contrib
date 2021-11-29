// Copyright  The OpenTelemetry Authors
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

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type Processor struct {
	statements []statement
	logger     *zap.Logger
}

func NewProcessor(queries []string, settings component.ProcessorCreateSettings) (*Processor, error) {
	statements, err := parse(queries)
	if err != nil {
		return nil, err
	}
	return &Processor{
		statements: statements,
		logger:     settings.Logger,
	}, nil
}

func (p *Processor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	process(td, p.statements)
	return td, nil
}

func process(td pdata.Traces, statements []statement) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
			il := rspans.InstrumentationLibrarySpans().At(j).InstrumentationLibrary()
			spans := rspans.InstrumentationLibrarySpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				for _, statement := range statements {
					if statement.condition(span, il, rspans.Resource()) {
						statement.function(span, il, rspans.Resource())
					}
				}
			}
		}
	}
}

func parse(queries []string) ([]statement, error) {
	parser, err := common.NewParser()
	if err != nil {
		return nil, err
	}
	statements := make([]statement, 0)
	for _, q := range queries {
		parsed := common.Query{}
		err := parser.ParseString("", q, &parsed)
		if err != nil {
			return nil, err
		}
		function, err := newFunctionCall(parsed.Invocation)
		if err != nil {
			return nil, err
		}
		condition, err := newConditionEvaluator(parsed.Condition)
		if err != nil {
			return nil, err
		}
		statements = append(statements, statement{
			function:  function,
			condition: condition,
		})
	}

	return statements, nil
}
