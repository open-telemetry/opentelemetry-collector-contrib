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
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// Query represents a parsed query. It is the entry point into the query DSL.
// nolint:govet
type Query struct {
	Invocation Invocation `@@`
	Condition  *Condition `( "where" @@ )?`
}

// Condition represents an optional boolean condition on the RHS of a query.
// nolint:govet
type Condition struct {
	Left  Value  `@@`
	Op    string `@("==" | "!=")`
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
	String     *string     `| @String`
	Float      *float64    `| @Float`
	Int        *int64      `| @Int`
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

// NewParser returns a parser that can be used to read a string into a Query. An error will be returned if the string
// is not formatted for the DSL.
func NewParser() *participle.Parser {
	lex := lexer.MustSimple([]lexer.Rule{
		{Name: `Ident`, Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`, Action: nil},
		{Name: `Float`, Pattern: `[-+]?\d*\.\d+([eE][-+]?\d+)?`, Action: nil},
		{Name: `Int`, Pattern: `[-+]?\d+`, Action: nil},
		{Name: `String`, Pattern: `"(\\"|[^"])*"`, Action: nil},
		{Name: `Operators`, Pattern: `==|!=|[,.()\[\]]`, Action: nil},
		{Name: "whitespace", Pattern: `\s+`, Action: nil},
	})
	parser, err := participle.Build(&Query{},
		participle.Lexer(lex),
		participle.Unquote("String"),
		participle.Elide("whitespace"),
	)
	// An error would be a fatal programming bug and cannot happen in runtime unless all the unit tests are
	// completely invalid.
	if err != nil {
		panic("Invalid parser configuration. This is a programming bug")
	}
	return parser
}
