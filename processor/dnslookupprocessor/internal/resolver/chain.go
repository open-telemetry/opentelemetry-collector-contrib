// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// ChainResolver represents a chain of resolvers that will be run in sequence
type ChainResolver struct {
	name      string
	resolvers []Resolver
	logger    *zap.Logger
}

func NewChainResolver(resolvers []Resolver, logger *zap.Logger) *ChainResolver {
	return &ChainResolver{
		name:      "chain",
		resolvers: resolvers,
		logger:    logger,
	}
}

// Resolve runs resolvers in sequence.
// returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return c.resolveInSequence(LogKeyHostname, hostname, func(r Resolver) (string, error) {
		return r.Resolve(ctx, hostname)
	})
}

// Reverse runs resolvers in sequence.
// returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return c.resolveInSequence(LogKeyIP, ip, func(r Resolver) (string, error) {
		return r.Reverse(ctx, ip)
	})
}

func (c *ChainResolver) Name() string {
	return c.name
}

// resolveInSequence attempts to resolveInSequence the given hostname/IP using the chain of resolvers.
// It returns the first successful IP/hostname. No resolution is considered a success.
// If one of the resolvers fails, the resolver follows up with retries with exponential backoff.
// It returns the last error of the last resolver if all retries failed.
func (c *ChainResolver) resolveInSequence(logKey string, target string, resolverFn func(resolver Resolver) (string, error)) (string, error) {
	var lastErr error

	for _, r := range c.resolvers {
		result, err := resolverFn(r)

		// Successful resolution
		if err == nil || errors.Is(err, ErrNoResolution) {
			c.logger.Debug(fmt.Sprintf("DNS lookup from %s", r.Name()),
				zap.String(logKey, target),
				zap.String(Flip(logKey), result))
			return result, nil
		}

		lastErr = err
	}

	// When the host file resolver is the only one in the chain, and it returns ErrNotInHostFiles,
	if errors.Is(lastErr, ErrNotInHostFiles) {
		c.logger.Debug("No matching entry in hostfiles", zap.String(logKey, target))
		return "", nil
	}

	return "", lastErr
}
