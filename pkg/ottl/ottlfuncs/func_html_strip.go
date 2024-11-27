// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/microcosm-cc/bluemonday"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type HTMLStripArguments[K any] struct {
	HTMLSource ottl.StringGetter[K]
}

func NewHTMLStripFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("HTMLStrip", &HTMLStripArguments[K]{}, createHTMLStripFunction[K])
}

func createHTMLStripFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*HTMLStripArguments[K])
	if !ok {
		return nil, fmt.Errorf("NewHTMLStripFactory args must be of type *HTMLStripArguments[K]")
	}

	return htmlStrip(args.HTMLSource), nil
}

func htmlStrip[K any](unescapedSource ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		htmlString, err := unescapedSource.Get(ctx, tCtx)
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
