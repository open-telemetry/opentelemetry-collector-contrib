// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"net/url"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type UriArguments[K any] struct {
	URI ottl.StringGetter[K]
}

func NewUriFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Uri", &UriArguments[K]{}, createTimeFunction[K])
}

func createUriFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*UriArguments[K])
	if !ok {
		return nil, fmt.Errorf("TimeFactory args must be of type *TimeArguments[K]")
	}

	return Uri(args.URI)
}

func Uri[K any](uriSource ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		uriString, err := uriSource.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if uriString == "" {
			return nil, fmt.Errorf("uri cannot be nil")
		}

		uriParts := make(map[string]string)

		_, err = url.Parse(uriString)
		if err != nil {
			return nil, err
		}

		return uriParts, nil
	}, nil
}
