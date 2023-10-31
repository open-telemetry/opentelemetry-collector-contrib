// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestExprString(t *testing.T) {
	t.Setenv("TEST_EXPR_STRING_ENV_FOO", "foo")
	t.Setenv("TEST_EXPR_STRING_ENV_BAR", "bar")

	exampleEntry := func() *entry.Entry {
		e := entry.New()
		e.Body = map[string]interface{}{
			"test": "value",
		}
		e.Resource = map[string]interface{}{
			"id": "value",
		}
		return e
	}

	cases := []struct {
		config   ExprStringConfig
		expected string
	}{
		{
			"test",
			"test",
		},
		{
			"EXPR( 'test' )",
			"test",
		},
		{
			"prefix-EXPR( 'test' )",
			"prefix-test",
		},
		{
			"prefix-EXPR('test')-suffix",
			"prefix-test-suffix",
		},
		{
			"prefix-EXPR('test')-suffix-EXPR('test2' + 'test3')",
			"prefix-test-suffix-test2test3",
		},
		{
			"EXPR('test' )EXPR('asdf')",
			"testasdf",
		},
		{
			"EXPR",
			"EXPR",
		},
		{
			"EXPR(",
			"EXPR(",
		},
		{
			")EXPR(",
			")EXPR(",
		},
		{
			"my EXPR( body.test )",
			"my value",
		},
		{
			"my EXPR(body.test )",
			"my value",
		},
		{
			"my EXPR( body.test)",
			"my value",
		},
		{
			"my EXPR(body.test)",
			"my value",
		},
		{
			"my EXPR(env('TEST_EXPR_STRING_ENV_FOO'))",
			"my foo",
		},
		{
			"my EXPR( env('TEST_EXPR_STRING_ENV_FOO') )",
			"my foo",
		},
		{
			"my EXPR(env('TEST_EXPR_STRING_ENV_FOO')) EXPR(env('TEST_EXPR_STRING_ENV_BAR'))",
			"my foo bar",
		},
		{
			"EXPR( resource.id )",
			"value",
		},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			exprString, err := tc.config.Build()
			require.NoError(t, err)

			env := GetExprEnv(exampleEntry())
			defer PutExprEnv(env)

			result, err := exprString.Render(env)
			require.NoError(t, err)
			require.Equal(t, tc.expected, result)
		})
	}
}
