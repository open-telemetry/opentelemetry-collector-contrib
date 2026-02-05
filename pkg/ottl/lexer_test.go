// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"testing"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"basic_int", "12345", false, []result{
			{"Int", "12345"},
		}},
		{"basic_float", "3.14159", false, []result{
			{"Float", "3.14159"},
		}},
		{"Float decimal 0", "3.0", false, []result{
			{"Float", "3.0"},
		}},
		{"Float with trailing dot", "3.", false, []result{
			{"Float", "3."},
		}},
		{"float_fraction_only", ".3", false, []result{
			{"Float", ".3"},
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
		{"not", "true and not false", false, []result{
			{"Boolean", "true"},
			{"OpAnd", "and"},
			{"OpNot", "not"},
			{"Boolean", "false"},
		}},
		{"nothing_recognizable", "|", true, []result{
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
		{"string escape with trailing backslash", `a("\\", "b")`, false, []result{
			{"Lowercase", "a"},
			{"LParen", "("},
			{"String", `"\\"`},
			{"Punct", ","},
			{"String", `"b"`},
			{"RParen", ")"},
		}},
		{"string escape with mismatched backslash", `"\"`, true, nil},
		{"Mixing case numbers and underscores", `aBCd_123E_4`, false, []result{
			{"Lowercase", "a"},
			{"Uppercase", "BC"},
			{"Lowercase", "d_123"},
			{"Uppercase", "E_4"},
		}},
		{"Math Operations", `+-*/`, false, []result{
			{"OpAddSub", "+"},
			{"OpAddSub", "-"},
			{"OpMultDiv", "*"},
			{"OpMultDiv", "/"},
		}},
		{"Math Equation AddSub", `1000 - 600`, false, []result{
			{"Int", "1000"},
			{"OpAddSub", "-"},
			{"Int", "600"},
		}},
		{"Math Equation AddSub nospace", `3-5`, false, []result{
			{"Int", "3"},
			{typ: "OpAddSub", val: "-"},
			{"Int", "5"},
		}},
		{"Math Equation AddSub nospace", `3*-5`, false, []result{
			{"Int", "3"},
			{typ: "OpMultDiv", val: "*"},
			{typ: "OpAddSub", val: "-"},
			{"Int", "5"},
		}},
		{"Math Equation MulDiv", `1.1 * 2.9`, false, []result{
			{"Float", "1.1"},
			{"OpMultDiv", "*"},
			{"Float", "2.9"},
		}},
		{"Map", `{"foo":"bar"}`, false, []result{
			{"LBrace", "{"},
			{"String", `"foo"`},
			{"Colon", ":"},
			{"String", `"bar"`},
			{"RBrace", "}"},
		}},
		{"Dynamic path", `attributes[attributes["foo"]]`, false, []result{
			{"Lowercase", "attributes"},
			{"Punct", "["},
			{"Lowercase", "attributes"},
			{"Punct", "["},
			{"String", `"foo"`},
			{"Punct", "]"},
			{"Punct", "]"},
		}},
		{"Dynamic path with math expression", `attributes["foo"][Len(attributes["foo"]) - 1]`, false, []result{
			{"Lowercase", "attributes"},
			{"Punct", "["},
			{"String", `"foo"`},
			{"Punct", "]"},
			{"Punct", "["},
			{"Uppercase", "L"},
			{"Lowercase", "en"},
			{"LParen", "("},
			{"Lowercase", "attributes"},
			{"Punct", "["},
			{"String", `"foo"`},
			{"Punct", "]"},
			{"RParen", ")"},
			{"OpAddSub", "-"},
			{"Int", "1"},
			{"Punct", "]"},
		}},
		{"Dynamic path with unary operation", `-attributes["foo"]`, false, []result{
			{typ: "OpAddSub", val: "-"},
			{"Lowercase", "attributes"},
			{"Punct", "["},
			{"String", `"foo"`},
			{"Punct", "]"},
		}},
		{"Float variations with trailing dot", "1. 2. 3.", false, []result{
			{"Float", "1."},
			{"Float", "2."},
			{"Float", "3."},
		}},
		{"Mixed int float arithmetic", "10+3.5", false, []result{
			{"Int", "10"},
			{"OpAddSub", "+"},
			{"Float", "3.5"},
		}},
		{"Scientific notation float with trailing dot", "1.e5", false, []result{
			{"Float", "1.e5"},
		}},
		{"Scientific notation with positive exponent", "3.14e+10", false, []result{
			{"Float", "3.14e+10"},
		}},
		{"Scientific notation with negative exponent", "2.5e-3", false, []result{
			{"Float", "2.5e-3"},
		}},
		{"Decimal only float variations", ".0 .5 .99", false, []result{
			{"Float", ".0"},
			{"Float", ".5"},
			{"Float", ".99"},
		}},
		{"Unary minus in multiplication", "3*-5", false, []result{
			{"Int", "3"},
			{"OpMultDiv", "*"},
			{"OpAddSub", "-"},
			{"Int", "5"},
		}},
		{"Complex unary operations", "2+-3*-4", false, []result{
			{"Int", "2"},
			{"OpAddSub", "+"},
			{"OpAddSub", "-"},
			{"Int", "3"},
			{"OpMultDiv", "*"},
			{"OpAddSub", "-"},
			{"Int", "4"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexDef := buildLexer()
			symbols := lexDef.Symbols()
			x, err := lexDef.LexString(tt.name, tt.input)
			require.NoError(t, err)
			i := 0
			for tok, err := x.Next(); tok.EOF() != true; tok, err = x.Next() {
				if tt.expectErr {
					// if we expected an error, just make sure there was one and don't check anything else
					assert.Error(t, err)
					break
				}
				require.NoError(t, err)
				assert.Equal(t, tt.output[i].val, tok.String())
				assert.Equal(t, symbols[tt.output[i].typ], tok.Type,
					"expected '%s' to be %s, got %v", tok.String(), tt.output[i].typ, nameOf(lexDef, tok.Type))
				i++
			}
		})
	}
}
