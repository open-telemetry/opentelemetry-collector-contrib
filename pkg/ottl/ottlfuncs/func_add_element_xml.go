// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/antchfx/xmlquery"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type AddElementXMLArguments[K any] struct {
	Target ottl.StringGetter[K]
	XPath  string
	Name   string
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

	if args.Name == "" {
		return nil, fmt.Errorf("AddElementXML name must be non-empty")
	}

	return addElementXML(args.Target, args.XPath, args.Name), nil
}

// addElementXML returns a XML formatted string that is a result of adding a child element to all matching elements
// in the target XML.
func addElementXML[K any](target ottl.StringGetter[K], xPath string, name string) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		var doc *xmlquery.Node
		if targetVal, err := target.Get(ctx, tCtx); err != nil {
			return nil, err
		} else if doc, err = parseNodesXML(targetVal); err != nil {
			return nil, err
		}
		var errs []error
		xmlquery.FindEach(doc, xPath, func(_ int, n *xmlquery.Node) {
			switch n.Type {
			case xmlquery.ElementNode, xmlquery.DocumentNode:
				xmlquery.AddChild(n, &xmlquery.Node{
					Type: xmlquery.ElementNode,
					Data: name,
				})
			default:
				errs = append(errs, fmt.Errorf("AddElementXML XPath selected non-element: %q", n.Data))
			}
		})
		return doc.OutputXML(false), errors.Join(errs...)
	}
}
