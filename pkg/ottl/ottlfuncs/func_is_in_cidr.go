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
	Target   ottl.StringGetter[K]
	Networks ottl.StringLikeSliceGetter[K]
}

func NewIsInCIDRFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsInCIDR", &IsInCIDRArguments[K]{}, createIsInCIDRFunction[K])
}

func createIsInCIDRFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsInCIDRArguments[K])
	if !ok {
		return nil, errors.New("IsInCIDRFactory args must be of type *IsInCIDRArguments[K]")
	}

	return isInCIDR(args.Target, args.Networks)
}

func isInCIDR[K any](target ottl.StringGetter[K], networks ottl.StringLikeSliceGetter[K]) (ottl.ExprFunc[K], error) {
	// Check if the networks are literals and pre-parse them if so.
	var literalNetworks []*net.IPNet
	if networkValues, isLiteral := ottl.GetLiteralValue[K, []string](networks); isLiteral {
		parsed, err := parseNetworks(networkValues)
		if err != nil {
			return nil, err
		}
		literalNetworks = parsed
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

		subnets := literalNetworks
		if subnets == nil {
			// Parse networks at runtime for dynamic values.
			networkValues, err := networks.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			subnets, err = parseNetworks(networkValues)
			if err != nil {
				return nil, err
			}
		}

		for _, subnet := range subnets {
			if subnet.Contains(ip) {
				return true, nil
			}
		}

		return false, nil
	}, nil
}

func parseNetworks(networks []string) ([]*net.IPNet, error) {
	subnets := make([]*net.IPNet, 0, len(networks))
	for _, network := range networks {
		_, subnet, err := net.ParseCIDR(network)
		if err != nil {
			return nil, err
		}
		subnets = append(subnets, subnet)
	}
	return subnets, nil
}
