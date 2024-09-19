// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type RemoveXMLArguments[K any] struct {
	Target ottl.StringGetter[K]
	XPath  string
}

func NewRemoveXMLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("RemoveXML", &RemoveXMLArguments[K]{}, createRemoveXMLFunction[K])
}

func createRemoveXMLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*RemoveXMLArguments[K])

	if !ok {
		return nil, fmt.Errorf("RemoveXML args must be of type *RemoveXMLAguments[K]")
	}

	return removeXML(args.Target, args.XPath), nil
}

// removeXML returns a `pcommon.String` that is a result of renaming the target XML  or s
func removeXML[K any](target ottl.StringGetter[K], xPath string) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		// Validate the xpath
		_, err = xpath.Compile(xPath)
		if err != nil {
			return nil, fmt.Errorf("invalid xpath: %w", err)
		}

		// Check if the xml starts with a declaration node
		preserveDeclearation := strings.HasPrefix(targetVal, "<?xml")

		top, err := xmlquery.Parse(strings.NewReader(targetVal))
		if err != nil {
			return nil, fmt.Errorf("parse xml: %w", err)
		}

		if !preserveDeclearation {
			xmlquery.RemoveFromTree(top.FirstChild)
		}

		xmlquery.FindEach(top, xPath, func(_ int, n *xmlquery.Node) {
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

		return top.OutputXML(false), nil
	}
}
