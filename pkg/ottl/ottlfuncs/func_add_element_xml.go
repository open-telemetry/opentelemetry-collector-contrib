// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/antchfx/xmlquery"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type AddElementXMLArguments[K any] struct {
	Target ottl.StringGetter[K]
	XPath  string
}

func NewAddElementXMLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("AddElementXML", &AddElementXMLArguments[K]{}, createAddElementXMLFunction[K])
}

func createAddElementXMLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*AddElementXMLArguments[K])

	if !ok {
		return nil, fmt.Errorf("AddElementXML args must be of type *AddElementXMLAguments[K]")
	}

	if err := validateXPath(args.XPath); err != nil {
		return nil, err
	}

	return addElementXML(args.Target, args.XPath), nil
}

// addElementXML returns a XML formatted string that is a result of removing all matching nodes from the target XML.
// This currently supports removal of elements, attributes, text values, comments, and CharData.
func addElementXML[K any](target ottl.StringGetter[K], xPath string) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		var doc *xmlquery.Node
		if targetVal, err := target.Get(ctx, tCtx); err != nil {
			return nil, err
		} else if doc, err = parseNodesXML(targetVal); err != nil {
			return nil, err
		}
		xmlquery.FindEach(doc, xPath, func(_ int, n *xmlquery.Node) {
			switch n.Type {
			case xmlquery.ElementNode:
				xmlquery.RemoveFromTree(n)
			case xmlquery.AttributeNode:
				n.Parent.RemoveAttr(n.Data)
			case xmlquery.TextNode:
				n.Data = ""
			case xmlquery.CommentNode:
				xmlquery.RemoveFromTree(n)
			case xmlquery.CharDataNode:
				xmlquery.RemoveFromTree(n)
			}
		})
		return doc.OutputXML(false), nil
	}
}
