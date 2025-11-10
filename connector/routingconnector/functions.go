// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func createRouteFunction[K any](ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		return true, nil
	}, nil
}

func standardFunctions[K any]() map[string]ottl.Factory[K] {
	// standard converters do not transform data, so we can safely use them
	funcs := ottlfuncs.StandardConverters[K]()

	deleteKey := ottlfuncs.NewDeleteKeyFactory[K]()
	funcs[deleteKey.Name()] = deleteKey

	deleteMatchingKeys := ottlfuncs.NewDeleteMatchingKeysFactory[K]()
	funcs[deleteMatchingKeys.Name()] = deleteMatchingKeys

	route := ottl.NewFactory("route", nil, createRouteFunction[K])
	funcs[route.Name()] = route

	return funcs
}

func spanFunctions() map[string]ottl.Factory[ottlspan.TransformContext] {
	funcs := standardFunctions[ottlspan.TransformContext]()

	isRootSpan := ottlfuncs.NewIsRootSpanFactory()
	funcs[isRootSpan.Name()] = isRootSpan

	return funcs
}
