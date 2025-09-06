// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/sawmillsfuncs"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type FromContextArguments[K any] struct {
	Key string
}

func NewFromContextFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"FromContext",
		&FromContextArguments[K]{},
		createFromContextFunction[K],
	)
}

func createFromContextFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*FromContextArguments[K])

	if !ok {
		return nil, errors.New(
			"FromContextFactory args must be of type *FromContextArguments[K]",
		)
	}

	return getFromContext[K](args.Key), nil
}

func getFromContext[K any](key string) ottl.ExprFunc[K] {
	return func(ctx context.Context, _ K) (any, error) {
		cl := client.FromContext(ctx)
		ss := cl.Metadata.Get(key)

		if len(ss) != 1 {
			return nil, nil
		}

		return ss[0], nil
	}
}
