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

package tqlconfig

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
	"github.com/stretchr/testify/assert"
)

func Test_Interpret(t *testing.T) {
	tests := []struct {
		name     string
		query    []DeclarativeQuery
		expected []string
	}{
		{
			name: "one query",
			query: []DeclarativeQuery{
				{
					Function: "set",
					Arguments: []Argument{
						{
							Other: tqltest.Strp("attributes[\"test\"]"),
						},
						{
							String: tqltest.Strp("pass"),
						},
					},
				},
			},
			expected: []string{
				"set(attributes[\"test\"], \"pass\")",
			},
		},
		{
			name: "multiple queries",
			query: []DeclarativeQuery{
				{
					Function: "singleArgument",
					Arguments: []Argument{
						{
							Other: tqltest.Strp("true"),
						},
					},
				},
				{
					Function: "limit",
					Arguments: []Argument{
						{
							Other: tqltest.Strp("attributes"),
						},
						{
							Other: tqltest.Strp("100"),
						},
					},
				},
				{
					Function: "noArguments",
				},
			},
			expected: []string{
				"singleArgument(true)",
				"limit(attributes, 100)",
				"noArguments()",
			},
		},
		{
			name: "one query",
			query: []DeclarativeQuery{
				{
					Function: "set",
					Arguments: []Argument{
						{
							Other: tqltest.Strp("span_id"),
						},
						{
							Invocation: &Invocation{
								Function: "SpanID",
								Arguments: []Argument{
									{
										Other: tqltest.Strp("0x0102030405060708"),
									},
								},
							},
						},
					},
				},
			},
			expected: []string{
				"set(span_id, SpanID(0x0102030405060708))",
			},
		},
		{
			name: "simple condition",
			query: []DeclarativeQuery{
				{
					Function: "set",
					Arguments: []Argument{
						{
							Other: tqltest.Strp("span_id"),
						},
						{
							Invocation: &Invocation{
								Function: "SpanID",
								Arguments: []Argument{
									{
										Other: tqltest.Strp("0x0102030405060708"),
									},
								},
							},
						},
					},
					Condition: tqltest.Strp("name == \"a name\""),
				},
			},
			expected: []string{
				"set(span_id, SpanID(0x0102030405060708)) where name == \"a name\"",
			},
		},
		{
			name: "complex condition",
			query: []DeclarativeQuery{
				{
					Function: "set",
					Arguments: []Argument{
						{
							Other: tqltest.Strp("span_id"),
						},
						{
							Invocation: &Invocation{
								Function: "SpanID",
								Arguments: []Argument{
									{
										Other: tqltest.Strp("0x0102030405060708"),
									},
								},
							},
						},
					},
					Condition: tqltest.Strp("(name == \"a name\" or thing == false) and other_thing == true"),
				},
			},
			expected: []string{
				"set(span_id, SpanID(0x0102030405060708)) where (name == \"a name\" or thing == false) and other_thing == true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Interpret(tt.query)
			assert.EqualValues(t, tt.expected, actual)
		})
	}
}
