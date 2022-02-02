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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type Processor struct {
	queries []Query
	logger  *zap.Logger
}

// Query holds a top level Query for processing trace data. A Query is a combination of a function
// invocation and the condition to match telemetry for invoking the function.
type Query struct {
	function  exprFunc
	condition condFunc
}

func NewProcessor(statements []string, functions map[string]interface{}, settings component.ProcessorCreateSettings) (*Processor, error) {
	queries, err := Parse(statements, functions)
	if err != nil {
		return nil, err
	}
	return &Processor{
		queries: queries,
		logger:  settings.Logger,
	}, nil
}

func (p *Processor) ProcessTraces(_ context.Context, td pdata.Traces) (pdata.Traces, error) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
			il := rspans.InstrumentationLibrarySpans().At(j).InstrumentationLibrary()
			spans := rspans.InstrumentationLibrarySpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				for _, statement := range p.queries {
					if statement.condition(span, il, rspans.Resource()) {
						statement.function(span, il, rspans.Resource())
					}
				}
			}
		}
	}
	return td, nil
}

func Parse(statements []string, functions map[string]interface{}) ([]Query, error) {
	queries := make([]Query, 0)
	var errors error

	for _, statement := range statements {
		parsed, err := common.Parse(statement)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		function, err := newFunctionCall(parsed.Invocation, functions)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		condition, err := newConditionEvaluator(parsed.Condition, functions)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		queries = append(queries, Query{
			function:  function,
			condition: condition,
		})
	}

	if errors != nil {
		return nil, errors
	}
	return queries, nil
}
