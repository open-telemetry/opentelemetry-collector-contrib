// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestASTExportsAreUsable(t *testing.T) {
	var (
		_ BooleanExpression
		_ Term
		_ OpOrTerm
		_ BooleanValue
		_ OpAndBooleanValue
		_ Comparison
		_ ConstExpr
		_ Value
		_ Argument
		_ MathExprLiteral
		_ ASTPath
		_ PathStruct
		_ Converter
		_ Editor
		_ MapValue
		_ List
		_ MathExpression
		_ AddSubTerm
		_ OpAddSubTerm
		_ MathValue
		_ Field
		_ ASTKey
		_ GrammerKey
		_ IsNil
		_ ByteSlice
		_ String
		_ Boolean
		_ OpMultDivValue
		_ MapItem
		_ CompareOp
		_ MathOp
	)

	table := CompareOpTable
	tests := map[string]CompareOp{
		"==": Eq,
		"!=": Ne,
		"<":  Lt,
		"<=": Lte,
		">":  Gt,
		">=": Gte,
	}
	for token, expected := range tests {
		got, ok := table[token]
		require.True(t, ok, "missing CompareOpTable entry for %q", token)
		require.Equal(t, expected, got)
	}
	require.Equal(t, []MathOp{Add, Sub, Mult, Div}, []MathOp{add, sub, mult, div})

	condition, err := ParseCondition("true")
	require.NoError(t, err)
	require.NotNil(t, condition)
}
