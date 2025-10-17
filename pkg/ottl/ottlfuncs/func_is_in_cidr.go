// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"errors"
	"net"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsInCIDRArguments[K any] struct {
	Target    ottl.StringGetter[K]
	addresses []string
}

func NewIsInCIDRFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsInCIDR", &IsInCIDRArguments[K]{}, createIsInCIDRFunction[K])
}

func createIsInCIDRFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsInCIDRArguments[K])
	if !ok {
		return nil, errors.New("InInCIDRFactory args must be of type *IsInCIDRArguments[K]")
	}

	return isInCIDR(args.Target, args.addresses), nil
}

func isInCIDR[K any](target ottl.StringGetter[K], networks []string) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		ip := net.ParseIP(val)
		if ip == nil {
			return false, nil
		}

		for _, network := range networks {
			_, subnet, err := net.ParseCIDR(network)
			if err != nil {
				return nil, err
			}

			if subnet.Contains(ip) {
				return true, nil
			}
		}

		return false, nil
	}
}
