// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/parseutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type URIArguments[K any] struct {
	URI ottl.StringGetter[K]
}

func NewURIFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("URI", &URIArguments[K]{}, createURIFunction[K])
}

func createURIFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*URIArguments[K])
	if !ok {
		return nil, fmt.Errorf("URIFactory args must be of type *URIArguments[K]")
	}

	return uri(args.URI), nil //revive:disable-line:var-naming
}

func uri[K any](uriSource ottl.StringGetter[K]) ottl.ExprFunc[K] { //revive:disable-line:var-naming
	return func(ctx context.Context, tCtx K) (any, error) {
		uriString, err := uriSource.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if uriString == "" {
			return nil, fmt.Errorf("uri cannot be empty")
		}

		return parseutils.ParseURI(uriString, true)
	}
}
