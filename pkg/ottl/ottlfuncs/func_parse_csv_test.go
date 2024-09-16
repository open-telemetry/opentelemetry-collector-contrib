// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseCSV(t *testing.T) {
	tests := []struct {
		name        string
		oArgs       ottl.Arguments
		want        map[string]any
		createError string
		parseError  string
	}{
		/* Test default mode */
		{
			name: "Parse comma separated values",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Parse with newline in first field",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1\nnewline,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1\nnewline",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Parse with newline in middle field",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2\nnewline,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2\nnewline",
				"col3": "val3",
			},
		},
		{
			name: "Parse with newline in last field",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3\nnewline", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3\nnewline",
			},
		},
		{
			name: "Parse with newline in multiple fields",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1\nnewline1,val2\nnewline2,val3\nnewline3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1\nnewline1",
				"col2": "val2\nnewline2",
				"col3": "val3\nnewline3",
			},
		},
		{
			name: "Parse with leading newline",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "\nval1,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Parse with trailing newline",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3\n", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Parse with newline at end of field",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1\n,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			want: map[string]any{
				"col1": "val1\n",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Parse comma separated values with explicit mode",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
				Mode: ottl.NewTestingOptional("strict"),
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Parse tab separated values",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1\tval2\tval3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1\tcol2\tcol3", nil
					},
				},
				Delimiter: ottl.NewTestingOptional("\t"),
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Header delimiter is different from row delimiter",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1\tval2\tval3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1 col2 col3", nil
					},
				},
				Delimiter:       ottl.NewTestingOptional("\t"),
				HeaderDelimiter: ottl.NewTestingOptional(" "),
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": "val3",
			},
		},
		{
			name: "Invalid target (strict mode)",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, errors.New("cannot get")
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2", nil
					},
				},
			},
			parseError: "error getting value for target in ParseCSV: error getting value in ottl.StandardStringGetter[interface {}]: cannot get",
		},
		{
			name: "Invalid header (strict mode)",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,val2`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, errors.New("cannot get")
					},
				},
			},
			parseError: "error getting value for header in ParseCSV: error getting value in ottl.StandardStringGetter[interface {}]: cannot get",
		},
		{
			name:        "Invalid args",
			oArgs:       nil,
			createError: "ParseCSVFactory args must be of type *ParseCSVArguments[K]",
		},
		{
			name: "Parse fails due to header/row column mismatch",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,val2,val3`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2", nil
					},
				},
			},
			parseError: "wrong number of fields: expected 2, found 3",
		},
		{
			name: "Parse fails due to header/row column mismatch",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,val2,val3`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2", nil
					},
				},
			},
			parseError: "wrong number of fields: expected 2, found 3",
		},
		{
			name: "Empty header string (strict)",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", nil
					},
				},
			},
			parseError: "headers must not be an empty string",
		},
		{
			name: "Parse fails due to empty row",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			parseError: "no csv lines found",
		},
		{
			name: "Parse fails for row with bare quotes",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,val2,v"al3`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
			},
			parseError: "wrong number of fields: expected 3, found 2",
		},

		/* Test parsing with lazy quotes */
		{
			name: "Parse lazyQuotes with quote in row",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,val2,v"al3`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
				Mode: ottl.NewTestingOptional("lazyQuotes"),
			},
			want: map[string]any{
				"col1": "val1",
				"col2": "val2",
				"col3": `v"al3`,
			},
		},
		{
			name: "Parse lazyQuotes invalid csv",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,"val2,"val3,val4"`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3,col4", nil
					},
				},
				Mode: ottl.NewTestingOptional("lazyQuotes"),
			},
			parseError: "wrong number of fields: expected 4, found 2",
		},
		/* Test parsing ignoring quotes */
		{
			name: "Parse quotes invalid csv",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,"val2,"val3,val4"`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3,col4", nil
					},
				},
				Mode: ottl.NewTestingOptional("ignoreQuotes"),
			},
			want: map[string]any{
				"col1": "val1",
				"col2": `"val2`,
				"col3": `"val3`,
				"col4": `val4"`,
			},
		},
		{
			name: "Invalid target (ignoreQuotes mode)",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, errors.New("cannot get")
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2", nil
					},
				},
				Mode: ottl.NewTestingOptional("ignoreQuotes"),
			},
			parseError: "error getting value for target in ParseCSV: error getting value in ottl.StandardStringGetter[interface {}]: cannot get",
		},
		{
			name: "Invalid header (ignoreQuotes mode)",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1,val2`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return nil, errors.New("cannot get")
					},
				},
				Mode: ottl.NewTestingOptional("ignoreQuotes"),
			},
			parseError: "error getting value for header in ParseCSV: error getting value in ottl.StandardStringGetter[interface {}]: cannot get",
		},
		{
			name: "Empty header string (ignoreQuotes)",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `val1`, nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", nil
					},
				},
				Mode: ottl.NewTestingOptional("ignoreQuotes"),
			},
			parseError: "headers must not be an empty string",
		},
		/* Validation tests */
		{
			name: "Delimiter is greater than one character",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
				Delimiter: ottl.NewTestingOptional("bad_delim"),
			},
			createError: "invalid arguments: delimiter must be a single character",
		},
		{
			name: "HeaderDelimiter is greater than one character",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
				HeaderDelimiter: ottl.NewTestingOptional("bad_delim"),
			},
			createError: "invalid arguments: header_delimiter must be a single character",
		},
		{
			name: "Bad mode",
			oArgs: &ParseCSVArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "val1,val2,val3", nil
					},
				},
				Header: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "col1,col2,col3", nil
					},
				},
				Mode: ottl.NewTestingOptional("fake-mode"),
			},
			createError: "unknown mode: fake-mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := createParseCSVFunction[any](ottl.FunctionContext{}, tt.oArgs)
			if tt.createError != "" {
				require.ErrorContains(t, err, tt.createError)
				return
			}

			require.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			if tt.parseError != "" {
				require.ErrorContains(t, err, tt.parseError)
				return
			}

			assert.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)

			require.Equal(t, tt.want, resultMap.AsRaw())
		})
	}
}
