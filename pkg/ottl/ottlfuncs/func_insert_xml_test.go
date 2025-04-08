// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_InsertXML(t *testing.T) {
	tests := []struct {
		name      string
		document  string
		xPath     string
		subdoc    string
		want      string
		expectErr string
	}{
		{
			name:     "add single element",
			document: `<a></a>`,
			xPath:    "/a",
			subdoc:   `<b/>`,
			want:     `<a><b></b></a>`,
		},
		{
			name:     "add single element to multiple matches",
			document: `<a></a><a></a>`,
			xPath:    "/a",
			subdoc:   `<b/>`,
			want:     `<a><b></b></a><a><b></b></a>`,
		},
		{
			name:     "add single element at multiple levels",
			document: `<a></a><z><a></a></z>`,
			xPath:    "//a",
			subdoc:   `<b/>`,
			want:     `<a><b></b></a><z><a><b></b></a></z>`,
		},
		{
			name:     "add multiple elements at root",
			document: `<a></a>`,
			xPath:    "/",
			subdoc:   `<b/><c/>`,
			want:     `<a></a><b></b><c></c>`,
		},
		{
			name:     "add multiple elements to other element",
			document: `<a></a>`,
			xPath:    "/a",
			subdoc:   `<b/><c/>`,
			want:     `<a><b></b><c></c></a>`,
		},
		{
			name:     "add multiple elements to multiple elements",
			document: `<a></a><a></a>`,
			xPath:    "/a",
			subdoc:   `<b/><c/>`,
			want:     `<a><b></b><c></c></a><a><b></b><c></c></a>`,
		},
		{
			name:     "add multiple elements at multiple levels",
			document: `<a></a><z><a></a></z>`,
			xPath:    "//a",
			subdoc:   `<b/><c/>`,
			want:     `<a><b></b><c></c></a><z><a><b></b><c></c></a></z>`,
		},
		{
			name:     "add rich doc",
			document: `<a></a>`,
			xPath:    "/a",
			subdoc:   `<x foo="bar"><b>text</b><c><d><e>1</e><e><![CDATA[two]]></e></d></c></x>`,
			want:     `<a><x foo="bar"><b>text</b><c><d><e>1</e><e><![CDATA[two]]></e></d></c></x></a>`,
		},
		{
			name:     "add root element to empty document",
			document: ``,
			xPath:    "/",
			subdoc:   `<a/>`,
			want:     `<a></a>`,
		},
		{
			name:     "add root element to non-empty document",
			document: `<a></a>`,
			xPath:    "/",
			subdoc:   `<a/>`,
			want:     `<a></a><a></a>`,
		},
		{
			name:      "err on attribute",
			document:  `<a foo="bar"></a>`,
			xPath:     "/a/@foo",
			subdoc:    "<b/>",
			want:      `<a foo="bar"></a>`,
			expectErr: `InsertXML XPath selected non-element: "foo"`,
		},
		{
			name:      "err on text content",
			document:  `<a>foo</a>`,
			xPath:     "/a/text()",
			subdoc:    "<b/>",
			want:      `<a>foo</a>`,
			expectErr: `InsertXML XPath selected non-element: "foo"`,
		},
		{
			name:     "insert missing elements",
			document: `<a><b>has b</b></a><a></a><a><b>also has b</b></a>`,
			xPath:    "//a[not(b)]", // elements of a which do not have a b
			subdoc:   `<b></b>`,
			want:     `<a><b>has b</b></a><a><b></b></a><a><b>also has b</b></a>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewInsertXMLFactory[any]()
			exprFunc, err := f.CreateFunction(
				ottl.FunctionContext{},
				&InsertXMLArguments[any]{
					Target: ottl.StandardStringGetter[any]{
						Getter: func(_ context.Context, _ any) (any, error) {
							return tt.document, nil
						},
					},
					XPath: tt.xPath,
					SubDocument: ottl.StandardStringGetter[any]{
						Getter: func(_ context.Context, _ any) (any, error) {
							return tt.subdoc, nil
						},
					},
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

func TestCreateInsertXMLFunc(t *testing.T) {
	factory := NewInsertXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XPath should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &InsertXMLArguments[any]{
			XPath: "!",
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML target should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &InsertXMLArguments[any]{
			Target: invalidXMLGetter(),
			XPath:  "/",
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)

	// Invalid XML subdoc should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &InsertXMLArguments[any]{
			Target: ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "<a/>", nil
				},
			},
			XPath:       "/",
			SubDocument: invalidXMLGetter(),
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
