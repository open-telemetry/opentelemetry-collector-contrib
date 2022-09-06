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

package tql // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"

import (
	"encoding/hex"
	"fmt"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"go.uber.org/multierr"
)

// ParsedQuery represents a parsed query. It is the entry point into the query DSL.
type ParsedQuery struct {
	Invocation  Invocation         `parser:"@@"`
	WhereClause *BooleanExpression `parser:"( 'where' @@ )?"`
}

// BooleanValue represents something that evaluates to a boolean --
// either an equality or inequality, explicit true or false, or
// a parenthesized subexpression.
type BooleanValue struct {
	Comparison *Comparison        `parser:"( @@"`
	ConstExpr  *Boolean           `parser:"| @Boolean"`
	SubExpr    *BooleanExpression `parser:"| '(' @@ ')' )"`
}

// OpAndBooleanValue represents the right side of an AND boolean expression.
type OpAndBooleanValue struct {
	Operator string        `parser:"@OpAnd"`
	Value    *BooleanValue `parser:"@@"`
}

// Term represents an arbitrary number of boolean values joined by AND.
type Term struct {
	Left  *BooleanValue        `parser:"@@"`
	Right []*OpAndBooleanValue `parser:"@@*"`
}

// OpOrTerm represents the right side of an OR boolean expression.
type OpOrTerm struct {
	Operator string `parser:"@OpOr"`
	Term     *Term  `parser:"@@"`
}

// BooleanExpression represents a true/false decision expressed
// as an arbitrary number of terms separated by OR.
type BooleanExpression struct {
	Left  *Term       `parser:"@@"`
	Right []*OpOrTerm `parser:"@@*"`
}

// CompareOp is the type of a comparison operator.
type CompareOp int

// These are the allowed values of a CompareOp
const (
	EQ CompareOp = iota
	NE
	LT
	LTE
	GTE
	GT
)

// a fast way to get from a string to a compareOp
var compareOpTable = map[string]CompareOp{
	"==": EQ,
	"!=": NE,
	"<":  LT,
	"<=": LTE,
	">":  GT,
	">=": GTE,
}

// Capture is how the parser converts an operator string to a CompareOp.
func (c *CompareOp) Capture(values []string) error {
	op, ok := compareOpTable[values[0]]
	if !ok {
		return fmt.Errorf("'%s' is not a valid operator", values[0])
	}
	*c = op
	return nil
}

// String() for CompareOp gives us more legible test results and error messages.
func (c CompareOp) String() string {
	switch c {
	case EQ:
		return "EQ"
	case NE:
		return "NE"
	case LT:
		return "LT"
	case LTE:
		return "LTE"
	case GTE:
		return "GTE"
	case GT:
		return "GT"
	default:
		return "UNKNOWN OP!"
	}
}

// Comparison represents an optional boolean condition.
type Comparison struct {
	Left  Value     `parser:"@@"`
	Op    CompareOp `parser:"@OpComparison"`
	Right Value     `parser:"@@"`
}

// Invocation represents a function call.
type Invocation struct {
	Function  string  `parser:"@(Uppercase | Lowercase)+"`
	Arguments []Value `parser:"'(' ( @@ ( ',' @@ )* )? ')'"`
}

// Value represents a part of a parsed query which is resolved to a value of some sort. This can be a telemetry path
// expression, function call, or literal.
type Value struct {
	Invocation *Invocation `parser:"( @@"`
	Bytes      *Bytes      `parser:"| @Bytes"`
	String     *string     `parser:"| @String"`
	Float      *float64    `parser:"| @Float"`
	Int        *int64      `parser:"| @Int"`
	Bool       *Boolean    `parser:"| @Boolean"`
	IsNil      *IsNil      `parser:"| @'nil'"`
	Enum       *EnumSymbol `parser:"| @Uppercase"`
	Path       *Path       `parser:"| @@ )"`
}

// Path represents a telemetry path expression.
type Path struct {
	Fields []Field `parser:"@@ ( '.' @@ )*"`
}

// Field is an item within a Path.
type Field struct {
	Name   string  `parser:"@Lowercase"`
	MapKey *string `parser:"( '[' @String ']' )?"`
}

// Query holds a top level Query for processing telemetry data. A Query is a combination of a function
// invocation and the expression to match telemetry for invoking the function.
type Query struct {
	Function  ExprFunc
	Condition BoolExpressionEvaluator
}

// Bytes type for capturing byte arrays
type Bytes []byte

func (b *Bytes) Capture(values []string) error {
	rawStr := values[0][2:]
	bytes, err := hex.DecodeString(rawStr)
	if err != nil {
		return err
	}
	*b = bytes
	return nil
}

// Boolean Type for capturing booleans, see:
// https://github.com/alecthomas/participle#capturing-boolean-value
type Boolean bool

func (b *Boolean) Capture(values []string) error {
	*b = values[0] == "true"
	return nil
}

type IsNil bool

func (n *IsNil) Capture(_ []string) error {
	*n = true
	return nil
}

type EnumSymbol string

func ParseQueries(statements []string, functions map[string]interface{}, pathParser PathExpressionParser, enumParser EnumParser) ([]Query, error) {
	var queries []Query
	var errors error

	for _, statement := range statements {
		parsed, err := parseQuery(statement)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		function, err := NewFunctionCall(parsed.Invocation, functions, pathParser, enumParser)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		expression, err := newBooleanExpressionEvaluator(parsed.WhereClause, functions, pathParser, enumParser)
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

var parser = newParser()

func parseQuery(raw string) (*ParsedQuery, error) {
	parsed, err := parser.ParseString("", raw)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

// buildLexer constructs a SimpleLexer definition.
// Note that the ordering of these rules matters.
// It's in a separate function so it can be easily tested alone (see lexer_test.go).
func buildLexer() *lexer.StatefulDefinition {
	return lexer.MustSimple([]lexer.SimpleRule{
		{Name: `Bytes`, Pattern: `0x[a-fA-F0-9]+`},
		{Name: `Float`, Pattern: `[-+]?\d*\.\d+([eE][-+]?\d+)?`},
		{Name: `Int`, Pattern: `[-+]?\d+`},
		{Name: `String`, Pattern: `"(\\"|[^"])*"`},
		{Name: `OpOr`, Pattern: `\b(or)\b`},
		{Name: `OpAnd`, Pattern: `\b(and)\b`},
		{Name: `OpComparison`, Pattern: `==|!=|>=|<=|>|<`},
		{Name: `Boolean`, Pattern: `\b(true|false)\b`},
		{Name: `LParen`, Pattern: `\(`},
		{Name: `RParen`, Pattern: `\)`},
		{Name: `Punct`, Pattern: `[,.\[\]]`},
		{Name: `Uppercase`, Pattern: `[A-Z_][A-Z0-9_]*`},
		{Name: `Lowercase`, Pattern: `[a-z_][a-z0-9_]*`},
		{Name: "whitespace", Pattern: `\s+`},
	})
}

// newParser returns a parser that can be used to read a string into a ParsedQuery. An error will be returned if the string
// is not formatted for the DSL.
func newParser() *participle.Parser[ParsedQuery] {
	lex := buildLexer()
	parser, err := participle.Build[ParsedQuery](
		participle.Lexer(lex),
		participle.Unquote("String"),
		participle.Elide("whitespace"),
	)
	if err != nil {
		panic("Unable to initialize parser; this is a programming error in the transformprocessor:" + err.Error())
	}
	return parser
}
