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

func Test_ParseXML(t *testing.T) {
	tests := []struct {
		name        string
		oArgs       ottl.Arguments
		want        map[string]any
		createError string
		parseError  string
	}{
		{
			name: "Text values in nested elements",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "<Log><User><ID>00001</ID><Name>Joe</Name><Email>joe.smith@example.com</Email></User><Text>User did a thing</Text></Log>", nil
					},
				},
			},
			want: map[string]any{
				"tag": "Log",
				"children": []any{
					map[string]any{
						"tag": "User",
						"children": []any{
							map[string]any{
								"tag":     "ID",
								"content": "00001",
							},
							map[string]any{
								"tag":     "Name",
								"content": "Joe",
							},
							map[string]any{
								"tag":     "Email",
								"content": "joe.smith@example.com",
							},
						},
					},
					map[string]any{
						"tag":     "Text",
						"content": "User did a thing",
					},
				},
			},
		},
		{
			name: "Formatted example",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `
						<Log>
						  <User>
						    <ID>00001</ID>
							<Name>Joe</Name>
							<Email>joe.smith@example.com</Email>
						  </User>
						  <Text>User did a thing</Text>
						</Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag": "Log",
				"children": []any{
					map[string]any{
						"tag": "User",
						"children": []any{
							map[string]any{
								"tag":     "ID",
								"content": "00001",
							},
							map[string]any{
								"tag":     "Name",
								"content": "Joe",
							},
							map[string]any{
								"tag":     "Email",
								"content": "joe.smith@example.com",
							},
						},
					},
					map[string]any{
						"tag":     "Text",
						"content": "User did a thing",
					},
				},
			},
		},
		{
			name: "Multiple tags with the same name",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log>This record has a collision<User id="0001"/><User id="0002"/></Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag":     "Log",
				"content": "This record has a collision",
				"children": []any{
					map[string]any{
						"tag": "User",
						"attributes": map[string]any{
							"id": "0001",
						},
					},
					map[string]any{
						"tag": "User",
						"attributes": map[string]any{
							"id": "0002",
						},
					},
				},
			},
		},
		{
			name: "Multiple lines of content",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log>
						This record has multiple lines of
						<User id="0001"/>
						text content
						</Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag":     "Log",
				"content": "This record has multiple lines oftext content",
				"children": []any{
					map[string]any{
						"tag": "User",
						"attributes": map[string]any{
							"id": "0001",
						},
					},
				},
			},
		},
		{
			name: "Attribute only element",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<HostInfo hostname="example.com" zone="east-1" cloudprovider="aws" />`, nil
					},
				},
			},
			want: map[string]any{
				"tag": "HostInfo",
				"attributes": map[string]any{
					"hostname":      "example.com",
					"zone":          "east-1",
					"cloudprovider": "aws",
				},
			},
		},
		{
			name: "Ignores XML declaration",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<?xml version="1.0" encoding="UTF-8" ?><Log>Log content</Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag":     "Log",
				"content": "Log content",
			},
		},
		{
			name: "Ignores comments",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log>This has a comment <!-- This is comment text --></Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag":     "Log",
				"content": "This has a comment",
			},
		},
		{
			name: "Ignores processing instructions",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log><?xml-stylesheet type="text/xsl" href="style.xsl"?>Log content</Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag":     "Log",
				"content": "Log content",
			},
		},
		{
			name: "Ignores directives",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log><!ELEMENT xi:fallback ANY>Log content</Log>`, nil
					},
				},
			},
			want: map[string]any{
				"tag":     "Log",
				"content": "Log content",
			},
		},
		{
			name: "Missing closing element",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log id="1">`, nil
					},
				},
			},
			parseError: "unmarshal xml: decode next token: XML syntax error on line 1: unexpected EOF",
		},
		{
			name: "Missing nested closing element",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log><Text></Log>`, nil
					},
				},
			},
			parseError: "unmarshal xml: decode next token: XML syntax error on line 1: element <Text> closed by </Log>",
		},
		{
			name: "Multiple XML elements in payload (trailing bytes)",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return `<Log></Log><Log></Log>`, nil
					},
				},
			},
			parseError: "trailing bytes after parsing xml",
		},
		{
			name: "Error getting target",
			oArgs: &ParseXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return "", errors.New("failed to get string")
					},
				},
			},
			parseError: "error getting value in ottl.StandardStringGetter[interface {}]: failed to get string",
		},
		{
			name:        "Invalid arguments",
			oArgs:       nil,
			createError: "ParseXMLFactory args must be of type *ParseXMLArguments[K]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := createParseXMLFunction[any](ottl.FunctionContext{}, tt.oArgs)
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
