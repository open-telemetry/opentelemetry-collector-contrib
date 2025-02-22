// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ConvertTextToElementsXML(t *testing.T) {
	tests := []struct {
		name        string
		document    string
		xPath       string
		elementName string
		want        string
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
			name:     "nop attributes",
			document: `<a foo="bar" hello="world"/>`,
			want:     `<a foo="bar" hello="world"></a>`,
		},
		{
			name:     "nop wrapped text",
			document: `<a>hello world</a>`,
			want:     `<a>hello world</a>`,
		},
		{
			name:     "simple hanging",
			document: `<a><b/>foo</a>`,
			want:     `<a><b></b><value>foo</value></a>`,
		},
		{
			name:        "simple hanging with tag name",
			elementName: "bar",
			document:    `<a><b/>foo</a>`,
			want:        `<a><b></b><bar>foo</bar></a>`,
		},
		{
			name:     "multiple hanging same level",
			document: `<a>foo<b/>bar</a>`,
			want:     `<a><value>foo</value><b></b><value>bar</value></a>`,
		},
		{
			name:        "multiple hanging multiple levels",
			document:    `<a>foo<b/>bar<c/>1<d>not</d>2<e><f/><f/></e></a>`,
			elementName: "v",
			want:        `<a><v>foo</v><b></b><v>bar</v><c></c><v>1</v><d>not</d><v>2</v><e><f></f><f></f></e></a>`,
		},
		{
			name:     "xpath select some",
			document: `<a><b><c/>foo</b><d><c/>bar</d><b><c/>baz</b></a>`,
			xPath:    "/a/b",
			want:     `<a><b><c></c><value>foo</value></b><d><c></c>bar</d><b><c></c><value>baz</value></b></a>`,
		},
		{
			name:        "xpath with element name",
			document:    `<a><b><c/>foo</b><d><c/>bar</d><b><c/>baz</b></a>`,
			xPath:       "/a/b",
			elementName: "V",
			want:        `<a><b><c></c><V>foo</V></b><d><c></c>bar</d><b><c></c><V>baz</V></b></a>`,
		},
	}
	factory := NewConvertTextToElementsXMLFactory[any]()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &ConvertTextToElementsXMLArguments[any]{
				Target: ottl.StandardStringGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						return tt.document, nil
					},
				},
				XPath:       ottl.NewTestingOptional(tt.xPath),
				ElementName: ottl.NewTestingOptional(tt.elementName),
			}
			exprFunc, err := factory.CreateFunction(ottl.FunctionContext{}, args)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestCreateConvertTextToElementsXMLFunc(t *testing.T) {
	factory := NewConvertTextToElementsXMLFactory[any]()
	fCtx := ottl.FunctionContext{}

	// Invalid arg type
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XPath should error on function creation
	exprFunc, err = factory.CreateFunction(
		fCtx, &ConvertTextToElementsXMLArguments[any]{
			XPath: ottl.NewTestingOptional("!"),
		})
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// Invalid XML should error on function execution
	exprFunc, err = factory.CreateFunction(
		fCtx, &ConvertTextToElementsXMLArguments[any]{
			Target: invalidXMLGetter(),
		})
	assert.NoError(t, err)
	assert.NotNil(t, exprFunc)
	_, err = exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
