// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

// grammarPathVisitor is used to extract all path from a parsedStatement or booleanExpression
type grammarPathVisitor struct {
	paths []path
}

func (v *grammarPathVisitor) visitEditor(_ *editor)                   {}
func (v *grammarPathVisitor) visitConverter(_ *converter)             {}
func (v *grammarPathVisitor) visitValue(_ *value)                     {}
func (v *grammarPathVisitor) visitMathExprLiteral(_ *mathExprLiteral) {}
func (v *grammarPathVisitor) visitListComprehension(c *listComprehension) {
	// THIS IS A HACK: Ignore loop variables here.
	c.List.accept(v)

	// path to ignore ->
	visitor := &grammarPathVisitor{[]path{}}
	c.Yield.accept(visitor)
	if c.Cond != nil {
		c.Cond.accept(visitor)
	}
	for _, p := range visitor.paths {
		// Skip loop variable
		if !(p.Context == "" && len(p.Fields) == 1 && p.Fields[0].Name == c.Ident) {
			v.paths = append(v.paths, p)
		}
	}
}
func (v *grammarPathVisitor) visitPath(value *path) {
	v.paths = append(v.paths, *value)
}

func getParsedStatementPaths(ps *parsedStatement) []path {
	visitor := &grammarPathVisitor{}
	ps.Editor.accept(visitor)
	if ps.WhereClause != nil {
		ps.WhereClause.accept(visitor)
	}
	return visitor.paths
}

func getBooleanExpressionPaths(be *booleanExpression) []path {
	visitor := &grammarPathVisitor{}
	be.accept(visitor)
	return visitor.paths
}
