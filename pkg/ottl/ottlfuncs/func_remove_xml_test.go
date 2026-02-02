// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_RemoveXML(t *testing.T) {
	tests := []struct {
		name     string
		document string
		xPath    string
		want     string
	}{
		{
			name:     "remove single element",
			document: `<a><b/></a>`,
			xPath:    "/a/b",
			want:     `<a></a>`,
		},
		{
			name:     "remove multiple element",
			document: `<a><b/><b/></a>`,
			xPath:    "/a/b",
			want:     `<a></a>`,
		},
		{
			name:     "remove multiple element with children",
			document: `<a><b/><b><c/></b></a>`,
			xPath:    "/a/b",
			want:     `<a></a>`,
		},
		{
			name:     "remove multiple element various depths",
			document: `<a><b/><b/><c><b><d/></b></c></a>`,
			xPath:    "/a//b",
			want:     `<a><c></c></a>`,
		},
		{
			name:     "remove attribute",
			document: `<a foo="bar"/>`,
			xPath:    "/a/@foo",
			want:     `<a></a>`,
		},
		{
			name:     "remove element with attribute",
			document: `<a><b foo="bar"/><b foo="notbar"/></a>`,
			xPath:    "/a/b[@foo='bar']",
			want:     `<a><b foo="notbar"></b></a>`,
		},
		{
			name:     "remove attributes from multiple nodes",
			document: `<a><b foo="bar"/><c foo="bar"/></a>`,
			xPath:    "//@foo",
			want:     `<a><b></b><c></c></a>`,
		},
		{
			name:     "remove multiple attributes from single node",
			document: `<a><b foo="bar" hello="world" keep="this"/></a>`,
			xPath:    "//@*[local-name() != 'keep']",
			want:     `<a><b keep="this"></b></a>`,
		},
		{
			name:     "remove text",
			document: `<a>delete this</a>`,
			xPath:    "//text()['*delete*']",
			want:     `<a></a>`,
		},
		{
			name:     "remove comments",
			document: `<a><b><c><!-- delete this --></c><!-- this too--></b></a>`,
			xPath:    "//comment()",
			want:     `<a><b><c></c></b></a>`,
		},
		{
			name:     "remove CDATA",
			document: `<a><b><![CDATA[text content possibly containing literal <or &characters ]]></b></a>`,
			xPath:    "//text()['<![CDATA[*']",
			want:     `<a><b></b></a>`,
		},
		{
			name:     "preserve declaration",
			document: `<?xml version="1.0" encoding="UTF-8"?><a>delete this</a>`,
			xPath:    "//text()['*delete*']",
			want:     `<?xml version="1.0" encoding="UTF-8"?><a></a>`,
		},
		{
			name:     "ignore empty",
			document: ``,
			xPath:    "/",
			want:     ``,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewRemoveXMLFactory[any]()
			exprFunc, err := factory.CreateFunction(
				ottl.FunctionContext{},
				&RemoveXMLArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(context.Context, any) (any, error) {
							return tt.document, nil
						},
					},
					XPath: tt.xPath,
				})
			require.NoError(t, err)

			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCreateRemoveXMLFunc(t *testing.T) {
	factory := NewRemoveXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XPath should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &RemoveXMLArguments[any]{
			XPath: "!",
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &RemoveXMLArguments[any]{
			Target: invalidXMLGetter(),
			XPath:  "/",
		})
	require.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func invalidXMLGetter() ottl.StandardStringGetter[any] {
	return ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return `<a>>>>>>>`, nil
		},
	}
}
