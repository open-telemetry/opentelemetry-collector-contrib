// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
import (
	"fmt"
	"strings"
)

// grammarPathVisitor is used to extract all path from a parsedStatement or booleanExpression
type grammarPathVisitor struct {
	paths []path
}

func (v *grammarPathVisitor) visitEditor(_ *editor)                   {}
func (v *grammarPathVisitor) visitValue(_ *value)                     {}
func (v *grammarPathVisitor) visitMathExprLiteral(_ *mathExprLiteral) {}

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
