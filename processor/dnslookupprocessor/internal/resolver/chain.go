// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/resolver"

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
// Returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return c.resolveInSequence(LogKeyHostname, hostname, func(r Resolver) (string, error) {
		return r.Resolve(ctx, hostname)
	})
}

// Reverse runs resolvers in sequence.
// Returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return c.resolveInSequence(LogKeyIP, ip, func(r Resolver) (string, error) {
		return r.Reverse(ctx, ip)
	})
}

func (c *ChainResolver) Name() string {
	return c.name
}

// Close closes all resolvers in the chain
func (c *ChainResolver) Close() error {
	var errs []error
	for _, r := range c.resolvers {
		if err := r.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// resolveInSequence attempts to resolve the given hostname/IP using the chain of resolvers.
// It returns the first successful IP/hostname. No resolution is considered a success.
// It returns the last error of the last resolver if all retries failed.
func (c *ChainResolver) resolveInSequence(logKey string, target string, resolverFn func(resolver Resolver) (string, error)) (string, error) {
	var lastErr error

	for _, r := range c.resolvers {
		result, err := resolverFn(r)

		// Successful resolution
		if err == nil {
			c.logger.Debug(fmt.Sprintf("DNS lookup from %s", r.Name()),
				zap.String(logKey, target),
				zap.String(Flip(logKey), result))

			return result, nil
		}

		// NS returns No Resolution
		if errors.Is(err, ErrNoResolution) {
			return "", err
		}

		lastErr = err
	}

	return "", lastErr
}
