// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_GetXML(t *testing.T) {
	tests := []struct {
		name     string
		document string
		xPath    string
		want     string
	}{
		{
			name:     "get single element",
			document: `<a><b/></a>`,
			xPath:    "/a/b",
			want:     `<b></b>`,
		},
		{
			name:     "get single complex element",
			document: `<a foo="bar"><b>hello</b></a>`,
			xPath:    "/a",
			want:     `<a foo="bar"><b>hello</b></a>`,
		},
		{
			name:     "get uniform elements from same parent",
			document: `<a><b>hello</b><b>world</b></a>`,
			xPath:    "/a/b",
			want:     `<b>hello</b><b>world</b>`,
		},
		{
			name:     "get nonuniform elements from same parent",
			document: `<a><b>hello</b><b><c>world</c></b><d/></a>`,
			xPath:    "/a/*",
			want:     `<b>hello</b><b><c>world</c></b><d></d>`,
		},
		{
			name:     "get elements from various places",
			document: `<a><x>1</x><b><x>2</x></b><d><e><f><x>3</x></f></e></d></a>`,
			xPath:    "/a//x",
			want:     `<x>1</x><x>2</x><x>3</x>`,
		},
		{
			name:     "get filtered elements from various places",
			document: `<a><x env="prod">1</x><b><x env="dev">2</x></b><d><e><f><x env="prod">3</x></f></e></d></a>`,
			xPath:    "/a//x[@env='prod']",
			want:     `<x env="prod">1</x><x env="prod">3</x>`,
		},
		{
			name:     "ignore empty",
			document: ``,
			xPath:    "/",
			want:     ``,
		},
		{
			name:     "ignore declaration",
			document: `<?xml version="1.0" encoding="UTF-8"?><a></a>`,
			xPath:    "/*",
			want:     `<a></a>`,
		},
		{
			name:     "ignore comments",
			document: `<!-- comment --><a></a><!-- comment -->`,
			xPath:    "/*",
			want:     `<a></a>`,
		},
		{
			name:     "get attribute selection",
			document: `<a foo="bar"></a>`,
			xPath:    "/a/@foo",
			want:     `bar`,
		},
		{
			name:     "get text selection",
			document: `<a>hello</a>`,
			xPath:    "/a/text()",
			want:     `hello`,
		},
		{
			name:     "get chardata selection",
			document: `<a><![CDATA[hello]]></a>`,
			xPath:    "/a/text()",
			want:     `hello`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewGetXMLFactory[any]()
			exprFunc, err := factory.CreateFunction(
				ottl.FunctionContext{},
				&GetXMLArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(_ context.Context, _ any) (any, error) {
							return tt.document, nil
						},
					},
					XPath: tt.xPath,
				})
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCreateGetXMLFunc(t *testing.T) {
	factory := NewGetXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XPath should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &GetXMLArguments[any]{
			XPath: "!",
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &GetXMLArguments[any]{
			Target: invalidXMLGetter(),
			XPath:  "/",
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
