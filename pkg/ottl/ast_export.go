// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

// Public AST type aliases for codegen
// These are type aliases to the internal AST types, making them accessible for code generation.
type (
	BooleanExpression = booleanExpression
	Term              = term
	OpOrTerm          = opOrTerm
	BooleanValue      = booleanValue
	OpAndBooleanValue = opAndBooleanValue
	Comparison        = comparison
	ConstExpr         = constExpr
	Value             = value
	Argument          = argument
	MathExprLiteral   = mathExprLiteral
	PathStruct        = path
	Converter         = converter
	Editor            = editor
	MapValue          = mapValue
	List              = list
	MathExpression    = mathExpression
	AddSubTerm        = addSubTerm
	OpAddSubTerm      = opAddSubTerm
	MathValue         = mathValue
	Field             = field
	GrammarKey        = key
	IsNil             = isNil
	ByteSlice         = byteSlice
	String            = string
	Boolean           = boolean
	OpMultDivValue    = opMultDivValue
	MapItem           = mapItem
)

const (
	Eq  = eq
	Ne  = ne
	Lt  = lt
	Lte = lte
	Gte = gte
	Gt  = gt
)

const (
	Add  = add
	Sub  = sub
	Mult = mult
	Div  = div
)

var (
	CompareOpTable = compareOpTable
	ParseCondition = parseCondition
)
