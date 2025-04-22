// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"

	"go.uber.org/zap"
)

// ChainResolver represents a chain of resolvers that will be run in sequence
type ChainResolver struct {
	resolvers []Resolver
	logger    *zap.Logger
}

func NewChainResolver(resolvers []Resolver, logger *zap.Logger) *ChainResolver {
	return &ChainResolver{
		resolvers: resolvers,
		logger:    logger,
	}
}

// Resolve runs resolvers in sequence.
// returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return c.resolveInSequence(func(r Resolver) (string, error) {
		return r.Resolve(ctx, hostname)
	})
}

// Reverse runs resolvers in sequence.
// returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return c.resolveInSequence(func(r Resolver) (string, error) {
		return r.Reverse(ctx, ip)
	})
}

// resolveInSequence attempts to resolveInSequence the given hostname/IP using the chain of resolvers.
// It returns the first successful IP/hostname. No resolution is considered a success.
// If one of the resolvers fails, the resolver follows up with retries with exponential backoff.
// It returns the last error of the last resolver if all retries failed.
func (c *ChainResolver) resolveInSequence(resolverFn func(resolver Resolver) (string, error)) (string, error) {
	var lastErr error

	for _, r := range c.resolvers {
		result, err := resolverFn(r)

		// Successful resolution
		if err == nil || errors.Is(err, ErrNoResolution) {
			return result, nil
		}

		lastErr = err
	}

	if errors.Is(lastErr, ErrNotInHostFiles) {
		return "", nil
	}

	return "", lastErr
}
