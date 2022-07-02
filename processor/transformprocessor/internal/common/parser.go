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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"encoding/hex"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
	"go.uber.org/multierr"
)

// ParsedQuery represents a parsed query. It is the entry point into the query DSL.
// nolint:govet
type ParsedQuery struct {
	Invocation  Invocation `@@`
	WhereClause *Condition `( "where" @@ )?`
}

// BooleanValue represents something that evaluates to a boolean --
// either an equality or inequality, explicit true or false, or
// a parenthesized subexpression.
// nolint:govet
type BooleanValue struct {
	Condition *Condition    `( @@`
	Expr      *BooleanValue `| @@`
	SubExpr   *BooleanValue `| @Lparen @@ @RParen )`
}

// OpBooleanValue represents
// nolint:govet
type OpBooleanValue struct {
	Operator string        `@OpAnd`
	Value    *BooleanValue `@@`
}

// Term represents
// nolint:govet
type Term struct {
	Left  *BooleanValue     `@@`
	Right []*OpBooleanValue `@@*`
}

// OpTerm represents
// nolint:govet
type OpTerm struct {
	Operator string `@OpOr`
	Term     *Term  `@@`
}

// BooleanExpression represents a true/false decision.
// nolint:govet
type BooleanExpression struct {
	Left  *Term     `@@`
	Right []*OpTerm `@@*`
}

// Condition represents an optional boolean condition.
// nolint:govet
type Condition struct {
	Left  Value  `@@`
	Op    string `@OpEq`
	Right Value  `@@`
}

// Invocation represents a function call.
// nolint:govet
type Invocation struct {
	Function  string  `@Ident`
	Arguments []Value `"(" ( @@ ( "," @@ )* )? ")"`
}

// Value represents a part of a parsed query which is resolved to a value of some sort. This can be a telemetry path
// expression, function call, or literal.
// nolint:govet
type Value struct {
	Invocation *Invocation `( @@`
	Bytes      *Bytes      `| @Bytes`
	String     *string     `| @String`
	Float      *float64    `| @Float`
	Int        *int64      `| @Int`
	Bool       *Boolean    `| @Boolean`
	IsNil      *IsNil      `| @"nil"`
	Path       *Path       `| @@ )`
}

// Path represents a telemetry path expression.
// nolint:govet
type Path struct {
	Fields []Field `@@ ( "." @@ )*`
}

// Field is an item within a Path.
// nolint:govet
type Field struct {
	Name   string  `@Ident`
	MapKey *string `( "[" @String "]" )?`
}

// Query holds a top level Query for processing telemetry data. A Query is a combination of a function
// invocation and the condition to match telemetry for invoking the function.
type Query struct {
	Function  ExprFunc
	Condition condFunc
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
	*b = Boolean(strings.EqualFold(values[0], "true"))
	return nil
}

type IsNil bool

func (n *IsNil) Capture(_ []string) error {
	*n = true
	return nil
}

func ParseQueries(statements []string, functions map[string]interface{}, pathParser PathExpressionParser) ([]Query, error) {
	queries := make([]Query, 0)
	var errors error

	for _, statement := range statements {
		parsed, err := parseQuery(statement)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		function, err := NewFunctionCall(parsed.Invocation, functions, pathParser)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		condition, err := newConditionEvaluator(parsed.WhereClause, functions, pathParser)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		queries = append(queries, Query{
			Function:  function,
			Condition: condition,
		})
	}

	if errors != nil {
		return nil, errors
	}
	return queries, nil
}

var parser = newParser()

func parseQuery(raw string) (*ParsedQuery, error) {
	parsed := &ParsedQuery{}
	err := parser.ParseString("", raw, parsed)
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
		{Name: `OpOr`, Pattern: `\b(?i:or)\b`},
		{Name: `OpAnd`, Pattern: `\b(?i:and)\b`},
		{Name: `OpEq`, Pattern: `==|!=`},
		{Name: `Boolean`, Pattern: `\b(?i:true|false)\b`},
		{Name: `LParen`, Pattern: `\(`},
		{Name: `RParen`, Pattern: `\)`},
		{Name: `Punct`, Pattern: `[,.\[\]]`},
		{Name: `Ident`, Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
		{Name: "whitespace", Pattern: `\s+`},
	})
}

// newParser returns a parser that can be used to read a string into a ParsedQuery. An error will be returned if the string
// is not formatted for the DSL.
func newParser() *participle.Parser {
	lex := buildLexer()
	parser, err := participle.Build(&ParsedQuery{},
		participle.Lexer(lex),
		participle.Unquote("String"),
		participle.Elide("whitespace"),
	)
	if err != nil {
		panic("Unable to initialize parser; this is a programming error in the transformprocessor:" + err.Error())
	}
	return parser
}
