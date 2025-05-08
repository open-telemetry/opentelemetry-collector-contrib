// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func createRouteFunction[K any](_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		return true, nil
	}, nil
}

func Functions[K any]() map[string]ottl.Factory[K] {
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
