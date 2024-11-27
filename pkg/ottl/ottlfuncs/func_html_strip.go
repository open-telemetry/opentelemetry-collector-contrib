// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/microcosm-cc/bluemonday"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type HtmlStripArguments[K any] struct {
	HtmlSource ottl.StringGetter[K]
}

func NewHtmlStripFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("HTMLStrip", &HtmlStripArguments[K]{}, createHtmlStripFunction[K])
}

func createHtmlStripFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*HtmlStripArguments[K])
	if !ok {
		return nil, fmt.Errorf("NewHtmlStripFactory args must be of type *HtmlStripArguments[K]")
	}

	return htmlStrip(args.HtmlSource), nil
}

func htmlStrip[K any](htmlSource ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		htmlString, err := htmlSource.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if htmlString == "" {
			return "", nil
		}

		b := bluemonday.StripTagsPolicy()
		return b.Sanitize(htmlString), nil
	}
}
