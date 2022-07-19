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

package tql // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"

import (
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/assert"
)

func nameOf(def *lexer.StatefulDefinition, val lexer.TokenType) string {
	for name, value := range def.Symbols() {
		if val == value {
			return name
		}
	}
	return "unknown"
}

func Test_lexer(t *testing.T) {
	type result struct {
		typ string
		val string
	}
	tests := []struct {
		name      string
		input     string
		expectErr bool
		output    []result
	}{
		{"basic_lowercase", "abc", false, []result{
			{"Lowercase", "abc"},
		}},
		{"basic_uppercase", "ABC", false, []result{
			{"Uppercase", "ABC"},
		}},
		{"basic_equality", "3==4.9", false, []result{
			{"Int", "3"},
			{"OpComparison", "=="},
			{"Float", "4.9"},
		}},
		{"basic_inequality", "3!=4.9", false, []result{
			{"Int", "3"},
			{"OpComparison", "!="},
			{"Float", "4.9"},
		}},
		{"unambiguous_names", "foo bar BAZZ", false, []result{
			{"Lowercase", "foo"},
			{"Lowercase", "bar"},
			{"Uppercase", "BAZZ"},
		}},
		{"name_containing_and", "iphone android", false, []result{
			{"Lowercase", "iphone"},
			{"Lowercase", "android"}, // should not parse "and" as an operator
		}},
		{"parse_and", "iphone and roid", false, []result{
			{"Lowercase", "iphone"},
			{"OpAnd", "and"}, // should parse "and" as an operator
			{"Lowercase", "roid"},
		}},
		{"name_containing_or", "oreo corn", false, []result{
			{"Lowercase", "oreo"},
			{"Lowercase", "corn"}, // should not parse "or" as an operator
		}},
		{"parse_and_or", "if, and, or but", false, []result{
			{"Lowercase", "if"},
			{"Punct", ","},
			{"OpAnd", "and"},
			{"Punct", ","},
			{"OpOr", "or"},
			{"Lowercase", "but"},
		}},
		{"nothing_recognizable", "{}", true, []result{
			{"", ""},
		}},
		{"basic_ident_expr", `set(attributes["bytes"], 0x0102030405060708)`, false, []result{
			{"Lowercase", "set"},
			{"LParen", "("},
			{"Lowercase", "attributes"},
			{"Punct", "["},
			{"String", `"bytes"`},
			{"Punct", "]"},
			{"Punct", ","},
			{"Bytes", "0x0102030405060708"},
			{"RParen", ")"},
		}},
		{"Mixing case", `aBCd`, false, []result{
			{"Lowercase", "a"},
			{"Uppercase", "BC"},
			{"Lowercase", "d"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexDef := buildLexer()
			symbols := lexDef.Symbols()
			x, err := lexDef.LexString(tt.name, tt.input)
			assert.NoError(t, err)
			i := 0
			for tok, err := x.Next(); tok.EOF() != true; tok, err = x.Next() {
				if tt.expectErr {
					// if we expected an error, just make sure there was one and don't check anything else
					assert.Error(t, err)
					break
				}
				assert.NoError(t, err)
				assert.Equal(t, tt.output[i].val, tok.String())
				assert.Equal(t, symbols[tt.output[i].typ], tok.Type,
					"expected '%s' to be %s, got %v", tok.String(), tt.output[i].typ, nameOf(lexDef, tok.Type))
				i++
			}
		})
	}
}
