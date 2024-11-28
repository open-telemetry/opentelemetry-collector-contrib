// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/microcosm-cc/bluemonday"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type StripHTMLArguments[K any] struct {
	HTMLSource ottl.StringGetter[K]
}

func NewStripHTMLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("StripHTML", &StripHTMLArguments[K]{}, createStripHTMLFunction[K])
}

func createStripHTMLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*StripHTMLArguments[K])
	if !ok {
		return nil, fmt.Errorf("NewStripHTMLFactory args must be of type *StripHTMLArguments[K]")
	}

	return stripHTML(args.HTMLSource), nil
}

func stripHTML[K any](rawString ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		htmlString, err := rawString.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if htmlString == "" {
			return "", nil
		}

		b := bluemonday.StrictPolicy()
		return b.Sanitize(htmlString), nil
	}
}
