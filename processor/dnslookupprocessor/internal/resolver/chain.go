// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"

import (
	"context"
	"errors"
)

// ChainResolver represents a chain of resolvers that will be run in sequence
type ChainResolver struct {
	resolvers []Resolver
}

func NewChainResolver(resolvers []Resolver) *ChainResolver {
	return &ChainResolver{
		resolvers: resolvers,
	}
}

// Resolve runs resolvers in sequence.
// Returns successful resolutions or an error if all resolvers fail
func (c *ChainResolver) Resolve(ctx context.Context, hostname string) ([]string, error) {
	return c.resolveInSequence(func(r Resolver) ([]string, error) {
		return r.Resolve(ctx, hostname)
	})
}

// Reverse runs resolvers in sequence.
// Returns successful resolutions or an error if all resolvers fail
func (c *ChainResolver) Reverse(ctx context.Context, ip string) ([]string, error) {
	return c.resolveInSequence(func(r Resolver) ([]string, error) {
		return r.Reverse(ctx, ip)
	})
}

// resolveInSequence attempts to resolve the given hostname/IP using the chain of resolvers.
// It returns successful IP/hostname. No resolution is considered a valid resolution that no need to continue the chain.
// It returns the last error of the last resolver if all retries failed.
func (c *ChainResolver) resolveInSequence(resolverFn func(resolver Resolver) ([]string, error)) ([]string, error) {
	var lastErr error

	for _, r := range c.resolvers {
		result, err := resolverFn(r)

		// Successful resolution
		if result != nil {
			return result, nil
		}

		// No Resolution is a valid resolution to return
		if errors.Is(err, ErrNoResolution) {
			return nil, nil
		}

		lastErr = err
	}

	return nil, lastErr
}
