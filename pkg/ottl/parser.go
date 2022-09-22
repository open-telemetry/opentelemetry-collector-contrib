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

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlgrammar"
)

// Query holds a top level Query for processing telemetry data. A Query is a combination of a function
// invocation and the expression to match telemetry for invoking the function.
type Query struct {
	Function  ExprFunc
	Condition boolExpressionEvaluator
}

type Parser struct {
	functions         map[string]interface{}
	pathParser        PathExpressionParser
	enumParser        EnumParser
	telemetrySettings component.TelemetrySettings
}

func NewParser(functions map[string]interface{}, pathParser PathExpressionParser, enumParser EnumParser, telemetrySettings component.TelemetrySettings) Parser {
	return Parser{
		functions:         functions,
		pathParser:        pathParser,
		enumParser:        enumParser,
		telemetrySettings: telemetrySettings,
	}
}

func (p *Parser) ParseQueries(statements []string) ([]Query, error) {
	var queries []Query
	var errors error

	for _, statement := range statements {
		parsed, err := parseQuery(statement)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		function, err := p.newFunctionCall(parsed.Invocation)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		expression, err := p.newBooleanExpressionEvaluator(parsed.WhereClause)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		queries = append(queries, Query{
			Function:  function,
			Condition: expression,
		})
	}

	if errors != nil {
		return nil, errors
	}
	return queries, nil
}

func parseQuery(raw string) (*ottlgrammar.ParsedQuery, error) {
	parsed, err := ottlgrammar.ParserSingleton.ParseString("", raw)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
