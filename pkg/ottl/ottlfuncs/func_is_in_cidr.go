// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
import (
	"context"
	"errors"
	"net"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type isInCIDRArguments[K any] struct {
	Target   ottl.StringGetter[K]
	Networks ottl.SliceGetter[K, ottl.StringGetter[K]]
}

// NewIsInCIDRFactory returns a factory for the IsInCIDR OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#isincidr
func NewIsInCIDRFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsInCIDR", &isInCIDRArguments[K]{}, createIsInCIDRFunction[K])
}

func createIsInCIDRFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*isInCIDRArguments[K])
	if !ok {
		return nil, errors.New("IsInCIDRFactory args must be of type *isInCIDRArguments[K]")
	}

	return isInCIDR(args.Target, &args.Networks)
}

func isInCIDR[K any](target ottl.StringGetter[K], networks *ottl.SliceGetter[K, ottl.StringGetter[K]]) (ottl.ExprFunc[K], error) {
	var literalNetworks []*net.IPNet
	// Len reports size only when the parser supplies a static list of network getters
	if length, ok := networks.Len(); ok {
		networkGetters, err := networks.Get(context.Background(), *new(K))
		if err != nil {
			return nil, err
		}

		literalNetworks = make([]*net.IPNet, 0, length)
		// Parse literal networks once, use the runtime path when the list contains a dynamic getter
		for _, network := range networkGetters {
			literal, isLiteral := ottl.GetLiteralValue[K, string](network)
			if !isLiteral {
				literalNetworks = nil
				break
			}
			_, subnet, err := net.ParseCIDR(literal)
			if err != nil {
				return nil, err
			}
			literalNetworks = append(literalNetworks, subnet)
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		ip := net.ParseIP(val)
		if ip == nil {
			return false, nil
		}

		if literalNetworks != nil {
			for _, subnet := range literalNetworks {
				if subnet.Contains(ip) {
					return true, nil
				}
			}
			return false, nil
		}

		// Resolve a dynamic network list only after the target is a valid IP address.
		networkGetters, err := networks.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if networkGetters == nil {
			return nil, errors.New("networks cannot be nil")
		}

		for _, network := range networkGetters {
			networkValue, err := network.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}

			_, subnet, err := net.ParseCIDR(networkValue)
			if err != nil {
				return nil, err
			}
			if subnet.Contains(ip) {
				return true, nil
			}
		}
		return false, nil
	}, nil
}
