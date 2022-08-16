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
	sb.WriteString(query.Function)
	sb.WriteString("(")
	for i, str := range query.Arguments {
		sb.WriteString(str)
		if i < len(query.Arguments)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString(")")
	if query.Condition != nil {
		sb.WriteString(" where ")
		sb.WriteString(*query.Condition)
	}
	return sb.String()
}
