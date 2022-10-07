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

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"github.com/alecthomas/participle/v2"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
)

// Statement holds a top level statement for processing telemetry data. A Statement is a combination of a function
// invocation and the expression to match telemetry for invoking the function.
type Statement[K any] struct {
	Function  ExprFunc[K]
	Condition BoolExpressionEvaluator[K]
}

type Parser[K any] struct {
	functions            map[string]interface{}
	pathParser           PathExpressionParser[K]
	enumParser           EnumParser
	telemetrySettings    component.TelemetrySettings
	transformationParser *participle.Parser[transformationStatement]
	conditionParser      *participle.Parser[conditionStatement]
}

func NewParser[K any](functions map[string]interface{}, pathParser PathExpressionParser[K], enumParser EnumParser, telemetrySettings component.TelemetrySettings) Parser[K] {
	return Parser[K]{
		functions:            functions,
		pathParser:           pathParser,
		enumParser:           enumParser,
		telemetrySettings:    telemetrySettings,
		transformationParser: newParser[transformationStatement](),
		conditionParser:      newParser[conditionStatement](),
	}
}

func (p *Parser[K]) ParseConditions(rawStatements []string) ([]BoolExpressionEvaluator[K], error) {
	var statements []BoolExpressionEvaluator[K]
	var errors error

	for _, statement := range rawStatements {
		parsed, err := parseStatement(p.conditionParser, statement)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		expression, err := p.newBooleanExpressionEvaluator(parsed.BooleanExpression)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		statements = append(statements, expression)
	}

	if errors != nil {
		return nil, errors
	}
	return statements, nil
}

func (p *Parser[K]) ParseStatements(rawStatements []string) ([]Statement[K], error) {
	var statements []Statement[K]
	var errors error

	for _, statement := range rawStatements {
		parsed, err := parseStatement(p.transformationParser, statement)
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
		statements = append(statements, Statement[K]{
			Function:  function,
			Condition: expression,
		})
	}

	if errors != nil {
		return nil, errors
	}
	return statements, nil
}

func parseStatement[G transformationStatement | conditionStatement](parser *participle.Parser[G], raw string) (*G, error) {
	parsed, err := parser.ParseString("", raw)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

// newParser returns a parser that can be used to read a string into a transformationStatement. An error will be returned if the string
// is not formatted for the DSL.
func newParser[K any]() *participle.Parser[K] {
	lex := buildLexer()
	parser, err := participle.Build[K](
		participle.Lexer(lex),
		participle.Unquote("String"),
		participle.Elide("whitespace"),
	)
	if err != nil {
		panic("Unable to initialize parser; this is a programming error in the transformprocessor:" + err.Error())
	}
	return parser
}
