// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_RemoveXML(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.StringGetter[any]
		xPath  string
		want   string
	}{
		{
			name: "remove single element",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b/></a>`, nil
				},
			},
			xPath: "/a/b",
			want:  `<a></a>`,
		},
		{
			name: "remove multiple element",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b/><b/></a>`, nil
				},
			},
			xPath: "/a/b",
			want:  `<a></a>`,
		},
		{
			name: "remove multiple element with children",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b/><b><c/></b></a>`, nil
				},
			},
			xPath: "/a/b",
			want:  `<a></a>`,
		},
		{
			name: "remove multiple element various depths",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b/><b/><c><b><d/></b></c></a>`, nil
				},
			},
			xPath: "/a//b",
			want:  `<a><c></c></a>`,
		},
		{
			name: "remove attribute",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a foo="bar"/>`, nil
				},
			},
			xPath: "/a/@foo",
			want:  `<a></a>`,
		},
		{
			name: "remove element with attribute",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b foo="bar"/><b foo="notbar"/></a>`, nil
				},
			},
			xPath: "/a/b[@foo='bar']",
			want:  `<a><b foo="notbar"></b></a>`,
		},
		{
			name: "remove attributes from multiple nodes",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b foo="bar"/><c foo="bar"/></a>`, nil
				},
			},
			xPath: "//@foo",
			want:  `<a><b></b><c></c></a>`,
		},
		{
			name: "remove multiple attributes from single node",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b foo="bar" hello="world" keep="this"/></a>`, nil
				},
			},
			xPath: "//@*[local-name() != 'keep']",
			want:  `<a><b keep="this"></b></a>`,
		},
		{
			name: "remove text",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a>delete this</a>`, nil
				},
			},
			xPath: "//text()['*delete*']",
			want:  `<a></a>`,
		},
		{
			name: "remove comments",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b><c><!-- delete this --></c><!-- this too--></b></a>`, nil
				},
			},
			xPath: "//comment()",
			want:  `<a><b><c></c></b></a>`,
		},
		{
			name: "remove CDATA",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<a><b><![CDATA[text content possibly containing literal <or &characters ]]></b></a>`, nil
				},
			},
			xPath: "//text()['<![CDATA[*']",
			want:  `<a><b></b></a>`,
		},
		{
			name: "preserve declaration",
			target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `<?xml version="1.0" encoding="UTF-8"?><a>delete this</a>`, nil
				},
			},
			xPath: "//text()['*delete*']",
			want:  `<?xml version="1.0" encoding="UTF-8"?><a></a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := removeXML(tt.target, tt.xPath)
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func Test_RemoveXML_InvalidXML(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return `<a>>>>>>>`, nil
		},
	}
	exprFunc := removeXML(target, "/foo")
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}

func Test_RemoveXML_InvalidXPath(t *testing.T) {
	target := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return `<a></a>`, nil
		},
	}
	exprFunc := removeXML(target, "!")
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}

func TestCreateRemoveXMLFunc(t *testing.T) {
	exprFunc, err := createRemoveXMLFunction[any](ottl.FunctionContext{}, &RemoveXMLArguments[any]{})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)

	_, err = createRemoveXMLFunction[any](ottl.FunctionContext{}, nil)
	assert.Error(t, err)
}
