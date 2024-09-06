// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"fmt"
	"strings"
)

type booleanValuePathVisitor booleanValue

func (r booleanValuePathVisitor) visit(paths *[]path) {
	if r.Comparison != nil {
		comparisonPathVisitor(*r.Comparison).visit(paths)
	}
	if r.ConstExpr != nil && r.ConstExpr.Converter != nil {
		converterPathVisitor(*r.ConstExpr.Converter).visit(paths)
	}
	if r.SubExpr != nil {
		booleanExpressionPathVisitor(*r.SubExpr).visit(paths)
	}
}

type opAndBooleanValuePathVisitor opAndBooleanValue

func (r opAndBooleanValuePathVisitor) visit(paths *[]path) {
	if r.Value != nil {
		booleanValuePathVisitor(*r.Value).visit(paths)
	}
}

type termPathVisitor term

func (r termPathVisitor) visit(paths *[]path) {
	if r.Left != nil {
		booleanValuePathVisitor(*r.Left).visit(paths)
	}
	for _, r := range r.Right {
		opAndBooleanValuePathVisitor(*r).visit(paths)
	}
}

type opOrTermPathVisitor opOrTerm

func (r opOrTermPathVisitor) visit(paths *[]path) {
	if r.Term != nil {
		termPathVisitor(*r.Term).visit(paths)
	}
}

type booleanExpressionPathVisitor booleanExpression

func (r booleanExpressionPathVisitor) visit(paths *[]path) {
	if r.Left != nil {
		termPathVisitor(*r.Left).visit(paths)
	}
	for _, r := range r.Right {
		opOrTermPathVisitor(*r).visit(paths)
	}
}

type comparisonPathVisitor comparison

func (r comparisonPathVisitor) visit(paths *[]path) {
	valuePathVisitor(r.Left).visit(paths)
	valuePathVisitor(r.Right).visit(paths)
}

type editorPathVisitor editor

func (r editorPathVisitor) visit(paths *[]path) {
	for _, arg := range r.Arguments {
		argumentPathVisitor(arg).visit(paths)
	}
}

type converterPathVisitor converter

func (r converterPathVisitor) visit(paths *[]path) {
	if r.Arguments != nil {
		for _, a := range r.Arguments {
			argumentPathVisitor(a).visit(paths)
		}
	}
}

type argumentPathVisitor argument

func (r argumentPathVisitor) visit(paths *[]path) {
	valuePathVisitor(r.Value).visit(paths)
}

type valuePathVisitor value

func (r valuePathVisitor) visit(paths *[]path) {
	if r.Literal != nil {
		mathExprLiteralPathVisitor(*r.Literal).visit(paths)
	}
	if r.MathExpression != nil {
		mathExpressionPathVisitor(*r.MathExpression).visit(paths)
	}
	if r.Map != nil {
		mapValuePathVisitor(*r.Map).visit(paths)
	}
	if r.List != nil {
		for _, i := range r.List.Values {
			valuePathVisitor(i).visit(paths)
		}
	}
}

type mapValuePathVisitor mapValue

func (r mapValuePathVisitor) visit(paths *[]path) {
	for _, v := range r.Values {
		valuePathVisitor(*v.Value).visit(paths)
	}
}

type mathExprLiteralPathVisitor mathExprLiteral

func (r mathExprLiteralPathVisitor) visit(paths *[]path) {
	if r.Path != nil {
		*paths = append(*paths, *r.Path)
	}
	if r.Editor != nil {
		editorPathVisitor(*r.Editor).visit(paths)
	}
	if r.Converter != nil {
		converterPathVisitor(*r.Converter).visit(paths)
	}
}

type mathValuePathVisitor mathValue

func (r mathValuePathVisitor) visit(paths *[]path) {
	if r.Literal != nil {
		mathExprLiteralPathVisitor(*r.Literal).visit(paths)
	}
	if r.SubExpression != nil {
		mathExpressionPathVisitor(*r.SubExpression).visit(paths)
	}
}

type opMultDivValuePathVisitor opMultDivValue

func (r opMultDivValuePathVisitor) visit(paths *[]path) {
	if r.Value != nil {
		mathValuePathVisitor(*r.Value).visit(paths)
	}
}

type addSubTermPathVisitor addSubTerm

func (r addSubTermPathVisitor) visit(paths *[]path) {
	if r.Left != nil {
		mathValuePathVisitor(*r.Left).visit(paths)
	}
	for _, r := range r.Right {
		opMultDivValuePathVisitor(*r).visit(paths)
	}
}

type opAddSubTermPathVisitor opAddSubTerm

func (r opAddSubTermPathVisitor) visit(paths *[]path) {
	if r.Term != nil {
		addSubTermPathVisitor(*r.Term).visit(paths)
	}
}

type mathExpressionPathVisitor mathExpression

func (r mathExpressionPathVisitor) visit(paths *[]path) {
	if r.Left != nil {
		addSubTermPathVisitor(*r.Left).visit(paths)
	}
	if r.Right != nil {
		for _, r := range r.Right {
			opAddSubTermPathVisitor(*r).visit(paths)
		}
	}
}

func getParsedStatementPaths(ps *parsedStatement) []path {
	var paths []path
	editorPathVisitor(ps.Editor).visit(&paths)
	if ps.WhereClause != nil {
		booleanExpressionPathVisitor(*ps.WhereClause).visit(&paths)
	}
	return paths
}

func getBooleanExpressionPaths(ce *booleanExpression) []path {
	var paths []path
	booleanExpressionPathVisitor(*ce).visit(&paths)
	return paths
}

// AppendStatementPathsContext changes the given statement adding the missing path's context prefix.
// Path's witch context value matches any WithPathContextNames value are not modified.
// It returns an error if the context argument is not a valid WithPathContextNames value.
func (p *Parser[K]) AppendStatementPathsContext(context string, statement string) (string, error) {
	if _, ok := p.pathContextNames[context]; !ok {
		return statement, fmt.Errorf(`unknown context "%s" for parser %T, valid options are: %s`, context, p, p.buildPathContextNamesText(""))
	}
	parsed, err := parseStatement(statement)
	if err != nil {
		return "", err
	}
	paths := getParsedStatementPaths(parsed)
	if len(paths) == 0 {
		return statement, nil
	}

	var missingContextOffsets []int
	for _, it := range paths {
		if _, ok := p.pathContextNames[it.Context]; !ok {
			missingContextOffsets = append(missingContextOffsets, it.Pos.Offset)
		}
	}

	return writeStatementWithPathsContext(context, statement, missingContextOffsets), nil
}

func writeStatementWithPathsContext(context string, statement string, offsets []int) string {
	if len(offsets) == 0 {
		return statement
	}
	contextPrefix := context + "."
	sb := strings.Builder{}
	left := 0
	for i, offset := range offsets {
		sb.WriteString(statement[left:offset])
		sb.WriteString(contextPrefix)
		if i+1 >= len(offsets) {
			sb.WriteString(statement[offset:])
		} else {
			left = offset
		}
	}
	return sb.String()
}
