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

package common

import (
	"fmt"
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
		{"a", "abc", false, []result{
			{"Ident", "abc"},
		}},
		{"b", "3==4.9", false, []result{
			{"Int", "3"},
			{"OpEq", "=="},
			{"Float", "4.9"},
		}},
		{"c", "foo bar bazz", false, []result{
			{"Ident", "foo"},
			{"Ident", "bar"},
			{"Ident", "bazz"},
		}},
		{"d", "iphone android", false, []result{
			{"Ident", "iphone"},
			{"Ident", "android"}, // should not parse "and" as an operator
		}},
		{"e", "iphone and roid", false, []result{
			{"Ident", "iphone"},
			{"OpAnd", "and"}, // should parse "and" as an operator
			{"Ident", "roid"},
		}},
		{"f", "oreo corn", false, []result{
			{"Ident", "oreo"},
			{"Ident", "corn"}, // should not parse "or" as an operator
		}},
		{"g", "if, and, or but", false, []result{
			{"Ident", "if"},
			{"Punct", ","},
			{"OpAnd", "and"}, // should parse "or" as an operator
			{"Punct", ","},
			{"OpOr", "or"}, // should parse "or" as an operator
			{"Ident", "but"},
		}},
		{"h", "{}", true, []result{
			{"", ""},
		}},
		{"i", `set(attributes["bytes"], 0x0102030405060708)`, false, []result{
			{"Ident", "set"},
			{"LParen", "("},
			{"Ident", "attributes"},
			{"Punct", "["},
			{"String", `"bytes"`},
			{"Punct", "]"},
			{"Punct", ","},
			{"Bytes", "0x0102030405060708"},
			{"RParen", ")"},
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
				fmt.Println(tok)
				if tt.expectErr {
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
