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

package tqlconfig // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tqlconfig"

import "strings"

func Interpret(queries []DeclarativeQuery) []string {
	strQueries := make([]string, 0, len(queries))
	for _, query := range queries {
		strQueries = append(strQueries, interpretQuery(query))
	}
	return strQueries
}

func interpretQuery(query DeclarativeQuery) string {
	var sb strings.Builder
	sb.WriteString(interpretInvocation(query.Function, query.Arguments))
	if query.Condition != nil {
		sb.WriteString(" where ")
		sb.WriteString(interpretExpression(*query.Condition))
	}
	return sb.String()
}

func interpretInvocation(function string, arguments []Argument) string {
	var sb strings.Builder
	sb.WriteString(function)
	sb.WriteString("(")
	for i, arg := range arguments {
		sb.WriteString(interpretArgument(arg))
		if i < len(arguments)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")
	return sb.String()
}

func interpretArgument(arg Argument) string {
	if arg.Invocation != nil {
		return interpretInvocation(arg.Invocation.Function, arg.Invocation.Arguments)
	}

	if arg.String != nil {
		var sb strings.Builder
		sb.WriteRune('"')
		sb.WriteString(*arg.String)
		sb.WriteRune('"')
		return sb.String()
	}

	return *arg.Other
}

func interpretExpression(expression Expression) string {
	var sb strings.Builder

	if expression.Comparison != nil {
		return interpretComparison(*expression.Comparison)
	}

	if expression.Or != nil {
		sb.WriteRune('(')
		for i, expr := range expression.Or {
			sb.WriteString(interpretExpression(expr))
			if i < len(expression.Or)-1 {
				sb.WriteString(" or ")
			}
		}
		sb.WriteRune(')')
		return sb.String()
	}

	sb.WriteRune('(')
	for i, expr := range expression.And {
		sb.WriteString(interpretExpression(expr))
		if i < len(expression.And)-1 {
			sb.WriteString(" and ")
		}
	}
	sb.WriteRune(')')
	return sb.String()
}

func interpretComparison(comparison Comparison) string {
	var sb strings.Builder
	sb.WriteString(interpretArgument(comparison.Arguments[0]))
	sb.WriteRune(' ')
	sb.WriteString(comparison.Operator)
	sb.WriteRune(' ')
	sb.WriteString(interpretArgument(comparison.Arguments[1]))
	return sb.String()
}
