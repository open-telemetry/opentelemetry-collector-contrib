// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package customottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// Get is a temporary OTTL editor to allow statements to return values. This
// will be removed after OTTL can parse data retrival expressions:
// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35621

type GetArguments[K any] struct {
	Value ottl.Getter[K]
}

func NewGetFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("get", &GetArguments[K]{}, createGetFunction[K])
}

func createGetFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*GetArguments[K])

	if !ok {
		return nil, fmt.Errorf("GetFactory args must be of type *GetArguments[K]")
	}

	return get(args.Value), nil
}

func get[K any](value ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		return value.Get(ctx, tCtx)
	}
}

// ConvertToStatement converts ottl.Value to an OTTL statement. To do
// this, it uses a custom `get` editor. This is expected to be a
// temporary measure until value parsing is allowed by the OTTL pkg.
func ConvertToStatement(s string) string {
	return fmt.Sprintf("get(%s)", s)
}
