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
	"context"
	"fmt"

	"github.com/alecthomas/participle/v2"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type ErrorMode int

const (
	IgnoreError ErrorMode = iota
	PropagateError
)

type Parser[K any] struct {
	functions         map[string]interface{}
	pathParser        PathExpressionParser[K]
	enumParser        EnumParser
	telemetrySettings component.TelemetrySettings
}

// Statement holds a top level Statement for processing telemetry data. A Statement is a combination of a function
// invocation and the boolean expression to match telemetry for invoking the function.
type Statement[K any] struct {
	function  Expr[K]
	condition BoolExpr[K]
}

// Execute is a function that will execute the statement's function if the statement's condition is met.
// Returns true if the function was run, returns false otherwise.
// If the statement contains no condition, the function will run and true will be returned.
// In addition, the functions return value is always returned.
func (s *Statement[K]) Execute(ctx context.Context, tCtx K) (any, bool, error) {
	condition, err := s.condition.Eval(ctx, tCtx)
	if err != nil {
		return nil, false, err
	}
	var result any
	if condition {
		result, err = s.function.Eval(ctx, tCtx)
		if err != nil {
			return nil, true, err
		}
	}
	return result, condition, nil
}

func NewParser[K any](
	functions map[string]interface{},
	pathParser PathExpressionParser[K],
	settings component.TelemetrySettings,
	options ...Option[K],
) Parser[K] {
	p := Parser[K]{
		functions:  functions,
		pathParser: pathParser,
		enumParser: func(*EnumSymbol) (*Enum, error) {
			return nil, fmt.Errorf("enums aren't supported for the current context: %T", new(K))
		},
		telemetrySettings: settings,
	}
	for _, opt := range options {
		opt(&p)
	}
	return p
}

type Option[K any] func(*Parser[K])

func WithEnumParser[K any](parser EnumParser) Option[K] {
	return func(p *Parser[K]) {
		p.enumParser = parser
	}
}

func (p *Parser[K]) ParseStatements(statements []string) ([]*Statement[K], error) {
	var parsedStatements []*Statement[K]
	var errors error

	for _, statement := range statements {
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
		expression, err := p.newBoolExpr(parsed.WhereClause)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		parsedStatements = append(parsedStatements, &Statement[K]{
			function:  function,
			condition: expression,
		})
	}

	if errors != nil {
		return nil, errors
	}
	return parsedStatements, nil
}

var parser = newParser[parsedStatement]()

func parseStatement(raw string) (*parsedStatement, error) {
	parsed, err := parser.ParseString("", raw)

	if err != nil {
		return nil, fmt.Errorf("unable to parse OTTL statement: %w", err)
	}
	err = parsed.checkForCustomError()
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

// newParser returns a parser that can be used to read a string into a parsedStatement. An error will be returned if the string
// is not formatted for the DSL.
func newParser[G any]() *participle.Parser[G] {
	lex := buildLexer()
	parser, err := participle.Build[G](
		participle.Lexer(lex),
		participle.Unquote("String"),
		participle.Elide("whitespace"),
		participle.UseLookahead(participle.MaxLookahead), // Allows negative lookahead to work properly in 'value' for 'mathExprLiteral'.
	)
	if err != nil {
		panic("Unable to initialize parser; this is a programming error in the transformprocessor:" + err.Error())
	}
	return parser
}

// Statements represents a list of statements that will be executed sequentially for a TransformContext.
type Statements[K any] struct {
	statements        []Statement[K]
	errorMode         ErrorMode
	telemetrySettings component.TelemetrySettings
}

// Execute is a function that will execute all the statements in the Statements list.
func (s *Statements[K]) Execute(ctx context.Context, tCtx K) error {
	for _, statement := range s.statements {
		_, _, err := statement.Execute(ctx, tCtx)
		if err != nil {
			if s.errorMode == PropagateError {
				err = fmt.Errorf("failed to execute statement: %w", err)
				return err
			}
			s.telemetrySettings.Logger.Error("failed to execute statement", zap.Error(err))
		}
	}
	return nil
}
