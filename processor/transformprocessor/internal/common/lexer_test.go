package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			{"Operators", "=="},
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
			{"Operators", "and"}, // should parse "and" as an operator
			{"Ident", "roid"},
		}},
		{"f", "oreo corn", false, []result{
			{"Ident", "oreo"},
			{"Ident", "corn"}, // should not parse "or" as an operator
		}},
		{"g", "if, and, or but", false, []result{
			{"Ident", "if"},
			{"Operators", ","},
			{"Operators", "and"}, // should parse "or" as an operator
			{"Operators", ","},
			{"Operators", "or"}, // should parse "or" as an operator
			{"Ident", "but"},
		}},
		{"h", "{}", true, []result{
			{"", ""},
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
				assert.Equal(t, symbols[tt.output[i].typ], tok.Type, "expected '%s' to be %s, symbols was %v", tok.String(), tt.output[i].typ, symbols)
				i++
			}
		})
	}
}
