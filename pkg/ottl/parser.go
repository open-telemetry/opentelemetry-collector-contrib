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

// Statement holds a top level Statement for processing telemetry data. A Statement is a combination of a function
// invocation and the expression to match telemetry for invoking the function.
type Statement struct {
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

func (p *Parser) ParseStatements(statementStrs []string) ([]Statement, error) {
	var statements []Statement
	var errors error

	for _, statement := range statementStrs {
		parsed, err := parseStatement(statement)
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
		statements = append(statements, Statement{
			Function:  function,
			Condition: expression,
		})
	}

	if errors != nil {
		return nil, errors
	}
	return statements, nil
}

func parseStatement(raw string) (*ottlgrammar.ParsedStatement, error) {
	parsed, err := ottlgrammar.ParserSingleton.ParseString("", raw)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}
