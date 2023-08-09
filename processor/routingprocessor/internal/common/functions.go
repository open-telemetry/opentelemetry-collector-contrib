// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func createRouteFunction[K any](_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		return true, nil
	}, nil
}

func Functions[K any]() map[string]ottl.Factory[K] {
	return ottl.CreateFactoryMap(
		ottlfuncs.NewIsMatchFactory[K](),
		ottlfuncs.NewDeleteKeyFactory[K](),
		ottlfuncs.NewDeleteMatchingKeysFactory[K](),
		// noop function, it is required since the parsing of conditions is not implemented yet,
		////github.com/open-telemetry/opentelemetry-collector-contrib/issues/13545
		ottl.NewFactory("route", nil, createRouteFunction[K]),
	)
}
