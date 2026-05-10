// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

// Public AST type aliases for downstream codegen and compatibility.
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
	ASTPath           = path
	PathStruct        = path // Deprecated: use ASTPath
	Converter         = converter
	Editor            = editor
	MapValue          = mapValue
	List              = list
	MathExpression    = mathExpression
	AddSubTerm        = addSubTerm
	OpAddSubTerm      = opAddSubTerm
	MathValue         = mathValue
	Field             = field
	ASTKey            = key
	GrammerKey        = key // Deprecated: use ASTKey
	IsNil             = isNil
	ByteSlice         = byteSlice
	String            = string
	Boolean           = boolean
	OpMultDivValue    = opMultDivValue
	MapItem           = mapItem
	CompareOp         = compareOp
	MathOp            = mathOp
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
