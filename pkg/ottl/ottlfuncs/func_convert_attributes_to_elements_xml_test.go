// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ConvertAttributesToElementsXML(t *testing.T) {
	tests := []struct {
		name     string
		document string
		xPath    string
		want     string
	}{
		{
			name:     "nop",
			document: `<a><b/></a>`,
			want:     `<a><b></b></a>`,
		},
		{
			name:     "nop declaration",
			document: `<?xml version="1.0" encoding="UTF-8"?><a><b/></a>`,
			want:     `<?xml version="1.0" encoding="UTF-8"?><a><b></b></a>`,
		},
		{
			name:     "single attribute",
			document: `<a foo="bar"/>`,
			want:     `<a><foo>bar</foo></a>`,
		},
		{
			name:     "multiple attributes - order 1",
			document: `<a foo="bar" hello="world"/>`,
			want:     `<a><foo>bar</foo><hello>world</hello></a>`,
		},
		{
			name:     "multiple attributes - order 2",
			document: `<a hello="world" foo="bar"/>`,
			want:     `<a><hello>world</hello><foo>bar</foo></a>`,
		},
		{
			name:     "with child elements",
			document: `<a hello="world" foo="bar"><b/><c/><b/></a>`,
			want:     `<a><b></b><c></c><b></b><hello>world</hello><foo>bar</foo></a>`,
		},
		{
			name:     "with child value",
			document: `<a hello="world" foo="bar">free value</a>`,
			want:     `<a>free value<hello>world</hello><foo>bar</foo></a>`,
		},
		{
			name:     "with child elements and values",
			document: `<a hello="world" foo="bar">free value<b/>2<c/></a>`,
			want:     `<a>free value<b></b>2<c></c><hello>world</hello><foo>bar</foo></a>`,
		},
		{
			name:     "multiple levels",
			document: `<a hello="world" foo="bar"><b href="www.example.com"></b></a>`,
			want:     `<a><b><href>www.example.com</href></b><hello>world</hello><foo>bar</foo></a>`,
		},
		{
			name:     "xpath filtered",
			document: `<a hello="world" foo="bar"><b href="www.example.com"></b></a>`,
			xPath:    "/a/b/@*", // only convert attributes of b
			want:     `<a hello="world" foo="bar"><b><href>www.example.com</href></b></a>`,
		},
		{
			name:     "attributes found with non-attributes xpath",
			document: `<a hello="world" foo="bar"><b href="www.example.com"></b></a>`,
			xPath:    "/a/b", // convert b (the attributes of b, even though the element b was selected)
			want:     `<a hello="world" foo="bar"><b href="www.example.com"></b></a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &ConvertAttributesToElementsXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.document, nil
					},
				},
				XPath: ottl.NewTestingOptional(tt.xPath),
			}
			exprFunc, err := createConvertAttributesToElementsXMLFunction[any](ottl.FunctionContext{}, args)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCreateConvertAttributesToElementsXMLFunc(t *testing.T) {
	factory := NewConvertAttributesToElementsXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XPath should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &ConvertAttributesToElementsXMLArguments[any]{
			XPath: ottl.NewTestingOptional("!"),
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &ConvertAttributesToElementsXMLArguments[any]{
			Target: invalidXMLGetter(),
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
