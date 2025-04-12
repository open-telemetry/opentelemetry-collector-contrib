// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// ChainResolver represents a chain of resolvers that will be run in sequence
type ChainResolver struct {
	maxRetries int
	resolvers  []Resolver
	logger     *zap.Logger
}

func NewChainResolver(maxRetries int, resolvers []Resolver, logger *zap.Logger) *ChainResolver {
	return &ChainResolver{
		maxRetries: maxRetries,
		resolvers:  resolvers,
		logger:     logger,
	}
}

// Resolve runs resolvers in sequence.
// returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Resolve(ctx context.Context, hostname string) (string, error) {
	return c.resolveWithRetry(ctx, LogKeyHostname, func(r Resolver) (string, error) {
		return r.Resolve(ctx, hostname)
	})
}

// Reverse runs resolvers in sequence.
// returns the first successful resolution or an error if all resolvers fail
func (c *ChainResolver) Reverse(ctx context.Context, ip string) (string, error) {
	return c.resolveWithRetry(ctx, LogKeyIP, func(r Resolver) (string, error) {
		return r.Reverse(ctx, ip)
	})
}

// resolveWithRetry attempts to resolve the given hostname/IP using the chain of resolvers.
// It returns the first successful IP/hostname.
// If all resolvers have no resolution, it considers as a success and returns an empty string.
// If one of the resolvers fails, it retries with exponential backoff.
// It returns the last error of the last resolver if all retries failed.
func (c *ChainResolver) resolveWithRetry(ctx context.Context, logKey string, resolverFn func(resolver Resolver) (string, error)) (string, error) {
	lastErr := ErrNoResolution
	allNoResolution := true
	expBackOff := NewExponentialBackOff()

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		for _, r := range c.resolvers {
			result, err := resolverFn(r)
			if err == nil {
				// Successful resolution
				return result, nil
			} else {
				lastErr = err
				if !errors.Is(err, ErrNoResolution) {
					allNoResolution = false
				}
			}
		}

		// If all resolvers have NoResolution error, we can stop retrying
		if allNoResolution {
			return "", nil
		}

		// Sleep for retry
		if attempt < c.maxRetries {
			sleepDuration := expBackOff.NextBackOff()

			select {
			case <-time.After(sleepDuration):
				c.logger.Debug(logKey+" lookup retry after backoff.", zap.String("interval", sleepDuration.String()))
			case <-ctx.Done():
				c.logger.Warn("Context cancelled during "+logKey+" lookup.", zap.Error(ctx.Err()))
				return "", ctx.Err()
			}
		}
	}

	return "", lastErr
}

func IsNoResolutionError(errs []error) bool {
	for _, e := range errs {
		if !errors.Is(e, ErrNoResolution) {
			return false
		}
	}
	return true
}

func NewExponentialBackOff() *backoff.ExponentialBackOff {
	expBackOff := backoff.ExponentialBackOff{
		InitialInterval:     100 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          2,
		MaxInterval:         10 * time.Second,
	}
	expBackOff.Reset()
	return &expBackOff
}
