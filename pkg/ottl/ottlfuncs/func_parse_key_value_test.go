// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_parseKeyValue(t *testing.T) {
	tests := []struct {
		name          string
		target        ottl.StringGetter[any]
		delimiter     ottl.Optional[string]
		pairDelimiter ottl.Optional[string]
		expected      map[string]any
	}{
		{
			name: "simple",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "name=ottl func=key_value", nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"name": "ottl",
				"func": "key_value",
			},
		},
		{
			name: "large",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `name=ottl age=1 job="software engineering" location="grand rapids michigan" src="10.3.3.76" dst=172.217.0.10 protocol=udp sport=57112 port=443 translated_src_ip=96.63.176.3 translated_port=57112`, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"age":               "1",
				"port":              "443",
				"dst":               "172.217.0.10",
				"job":               "software engineering",
				"location":          "grand rapids michigan",
				"name":              "ottl",
				"protocol":          "udp",
				"sport":             "57112",
				"src":               "10.3.3.76",
				"translated_port":   "57112",
				"translated_src_ip": "96.63.176.3",
			},
		},
		{
			name: "embedded double quotes in single quoted value",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `a=b c='this is a "co ol" value'`, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"a": "b",
				"c": "this is a \"co ol\" value",
			},
		},
		{
			name: "double quotes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `requestClientApplication="Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0"`, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"requestClientApplication": "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0",
			},
		},
		{
			name: "single quotes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "requestClientApplication='Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0'", nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"requestClientApplication": "Mozilla/5.0 (Windows NT 6.1; WOW64; rv:40.0) Gecko/20100101 Firefox/40.0",
			},
		},
		{
			name: "double quotes strip leading & trailing spaces",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `name="   ottl " func="  key_ value"`, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"name": "ottl",
				"func": "key_ value",
			},
		},
		{
			name: "! delimiter && whitespace pair delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "   name!ottl     func!key_value hello!world  ", nil
				},
			},
			delimiter:     ottl.NewTestingOptional[string]("!"),
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"name":  "ottl",
				"func":  "key_value",
				"hello": "world",
			},
		},
		{
			name: "!! delimiter && whitespace pair delimiter with newlines",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `   
name!!ottl     
func!!key_value                      hello!!world  `, nil
				},
			},
			delimiter:     ottl.NewTestingOptional[string]("!!"),
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"name":  "ottl",
				"func":  "key_value",
				"hello": "world",
			},
		},
		{
			name: "!! delimiter && newline pair delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `name!!ottl     
func!!      key_value another!!pair
hello!!world  `, nil
				},
			},
			delimiter:     ottl.NewTestingOptional[string]("!!"),
			pairDelimiter: ottl.NewTestingOptional[string]("\n"),
			expected: map[string]any{
				"name":  "ottl",
				"func":  "key_value another!!pair",
				"hello": "world",
			},
		},
		{
			name: "quoted value contains delimiter and pair delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `name="ottl="_func="=key_value"`, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.NewTestingOptional("_"),
			expected: map[string]any{
				"name": "ottl=",
				"func": "=key_value",
			},
		},
		{
			name: "complicated delimiters",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `k1@*v1_!_k2@**v2_!__k3@@*v3__`, nil
				},
			},
			delimiter:     ottl.NewTestingOptional("@*"),
			pairDelimiter: ottl.NewTestingOptional("_!_"),
			expected: map[string]any{
				"k1":   "v1",
				"k2":   "*v2",
				"_k3@": "v3__",
			},
		},
		{
			name: "leading and trailing pair delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "   k1=v1   k2==v2       k3=v3= ", nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"k1": "v1",
				"k2": "=v2",
				"k3": "v3=",
			},
		},
		{
			name: "embedded double quotes end single quoted value",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `a=b c='this is a "co ol"'`, nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"a": "b",
				"c": "this is a \"co ol\"",
			},
		},
		{
			name: "more quotes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "a=b c=d'='", nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.Optional[string]{},
			expected: map[string]any{
				"a": "b",
				"c": "d=",
			},
		},

		{
			name: "long pair delimiter",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "a=b c=d", nil
				},
			},
			delimiter:     ottl.Optional[string]{},
			pairDelimiter: ottl.NewTestingOptional("aaaaaaaaaaaaaaaa"),
			expected: map[string]any{
				"a": "b c=d", // occurs because `SplitString()` returns original string and `strings.SplitN` with N=2 will split on just the first instance of delimiter("=")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := parseKeyValue[any](tt.target, tt.delimiter, tt.pairDelimiter)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)

			actual, ok := result.(pcommon.Map)
			assert.True(t, ok)

			expected := pcommon.NewMap()
			assert.NoError(t, expected.FromRaw(tt.expected))

			assert.Equal(t, expected.Len(), actual.Len())
			for k := range expected.All() {
				ev, _ := expected.Get(k)
				av, ok := actual.Get(k)
				assert.True(t, ok)
				assert.Equal(t, ev, av)
			}
		})
	}
}

func Test_parseKeyValue_equal_delimiters(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "", nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("=")
	_, err := parseKeyValue[any](target, delimiter, pairDelimiter)
	assert.Error(t, err)

	delimiter = ottl.NewTestingOptional[string](" ")
	_, err = parseKeyValue[any](target, delimiter, ottl.Optional[string]{})
	assert.Error(t, err)
}

func Test_parseKeyValue_bad_target(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return 1, nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("!")
	exprFunc, err := parseKeyValue[any](target, delimiter, pairDelimiter)
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}

func Test_parseKeyValue_empty_target(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "", nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("!")
	exprFunc, err := parseKeyValue[any](target, delimiter, pairDelimiter)
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}

func Test_parseKeyValue_bad_split(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "name=ottl!hello_world", nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("=")
	pairDelimiter := ottl.NewTestingOptional[string]("!")
	exprFunc, err := parseKeyValue[any](target, delimiter, pairDelimiter)
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	assert.ErrorContains(t, err, "failed to split pairs into key-values: cannot split \"hello_world\" into 2 items, got 1 item(s)")
}

func Test_parseKeyValue_mismatch_quotes(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return `k1=v1 k2='v2"`, nil
		},
	}
	exprFunc, err := parseKeyValue[any](target, ottl.Optional[string]{}, ottl.Optional[string]{})
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}

func Test_parseKeyValue_bad_delimiter(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "a=b c=d", nil
		},
	}

	// covers too long of a delimiter && delimiter not found
	delimiter := ottl.NewTestingOptional[string]("=============")
	exprFunc, err := parseKeyValue[any](target, delimiter, ottl.Optional[string]{})
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	assert.ErrorContains(t, err, "failed to split pairs into key-values: cannot split \"a=b\" into 2 items, got 1 item(s)")
}

func Test_parseKeyValue_empty_delimiters(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "a=b c=d", nil
		},
	}
	delimiter := ottl.NewTestingOptional[string]("")

	_, err := parseKeyValue[any](target, delimiter, ottl.Optional[string]{})
	assert.ErrorContains(t, err, "delimiter cannot be set to an empty string")

	_, err = parseKeyValue[any](target, ottl.Optional[string]{}, delimiter)
	assert.ErrorContains(t, err, "pair delimiter cannot be set to an empty string")
}
