// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_AddElementXML(t *testing.T) {
	tests := []struct {
		name        string
		document    string
		xPath       string
		elementName string
		want        string
		expectErr   string
	}{
		{
			name:        "add single element",
			document:    `<a></a>`,
			xPath:       "/a",
			elementName: "b",
			want:        `<a><b></b></a>`,
		},
		{
			name:        "add to multiple elements",
			document:    `<a></a><a></a>`,
			xPath:       "/a",
			elementName: "b",
			want:        `<a><b></b></a><a><b></b></a>`,
		},
		{
			name:        "add at multiple levels",
			document:    `<a></a><b><a></a></b>`,
			xPath:       "//a",
			elementName: "c",
			want:        `<a><c></c></a><b><a><c></c></a></b>`,
		},
		{
			name:        "add root element to empty document",
			document:    ``,
			xPath:       "/",
			elementName: "a",
			want:        `<a></a>`,
		},
		{
			name:        "add root element to non-empty document",
			document:    `<a></a>`,
			xPath:       "/",
			elementName: "a",
			want:        `<a></a><a></a>`,
		},
		{
			name:        "err on attribute",
			document:    `<a foo="bar"></a>`,
			xPath:       "/a/@foo",
			elementName: "b",
			want:        `<a foo="bar"></a>`,
			expectErr:   `AddElementXML XPath selected non-element: "foo"`,
		},
		{
			name:        "err on text content",
			document:    `<a>foo</a>`,
			xPath:       "/a/text()",
			elementName: "b",
			want:        `<a>foo</a>`,
			expectErr:   `AddElementXML XPath selected non-element: "foo"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewAddElementXMLFactory[any]()
			exprFunc, err := f.CreateFunction(
				ottl.FunctionContext{},
				&AddElementXMLArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(_ context.Context, _ any) (any, error) {
							return tt.document, nil
						},
					},
					XPath: tt.xPath,
					Name:  tt.elementName,
				})
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			if tt.expectErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectErr)
			}
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCreateAddElementXMLFunc(t *testing.T) {
	factory := NewAddElementXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XPath should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &AddElementXMLArguments[any]{
			XPath: "!",
			Name:  "foo",
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Empty Name should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &AddElementXMLArguments[any]{
			XPath: "/",
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &AddElementXMLArguments[any]{
			Target: invalidXMLGetter(),
			XPath:  "/",
			Name:   "foo",
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
