// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// Retry configuration constants
const (
	defaultRetryInitialInterval = 1 * time.Second
	defaultRetryMaxInterval     = 30 * time.Second
	defaultRetryMaxElapsedTime  = 5 * time.Minute
)

// rateLimitState holds GitHub API rate limit information for a specific API type.
// GitHub has separate rate limits for GraphQL and REST APIs.
// Thread-safe via mutex for concurrent access from multiple goroutines.
type rateLimitState struct {
	mu sync.RWMutex

	// Current rate limit values from most recent API response
	cost      int       // Points consumed by last query (GraphQL only)
	limit     int       // Maximum points/requests per hour
	remaining int       // Points/requests remaining in current window
	resetAt   time.Time // When rate limit window resets
	used      int       // Total points/requests used in current window

	// Timestamp of last update
	lastUpdated time.Time
}

// update updates the rate limit state from GraphQL response.
func (r *rateLimitState) update(cost, limit, remaining, used int, resetAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cost = cost
	r.limit = limit
	r.remaining = remaining
	r.resetAt = resetAt
	r.used = used
	r.lastUpdated = time.Now()
}

// updateFromRest updates the rate limit state from REST API response.
func (r *rateLimitState) updateFromRest(limit, remaining int, reset time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limit = limit
	r.remaining = remaining
	r.resetAt = reset
	r.used = limit - remaining
	r.lastUpdated = time.Now()
}

// get returns current rate limit values.
func (r *rateLimitState) get() (limit, remaining, used int, resetAt time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.limit, r.remaining, r.used, r.resetAt
}

// needsProactiveWait determines if we should wait before making more requests.
// Returns true if remaining points are below threshold and we have rate limit data.
func (r *rateLimitState) needsProactiveWait(threshold int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.lastUpdated.IsZero() {
		return false
	}

	return r.remaining < threshold
}

// waitDuration calculates how long to wait until rate limit resets.
// Returns 0 if rate limit has already reset.
func (r *rateLimitState) waitDuration() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.resetAt.IsZero() {
		return 0
	}

	wait := time.Until(r.resetAt)
	if wait < 0 {
		return 0
	}

	return wait
}

// checkAndWaitGraphQL performs proactive rate limit checking for the GraphQL API.
func (ghs *githubScraper) checkAndWaitGraphQL(ctx context.Context) error {
	return ghs.checkAndWait(ctx, ghs.graphQLRateLimit, "GraphQL")
}

// checkAndWaitREST performs proactive rate limit checking for the REST API.
func (ghs *githubScraper) checkAndWaitREST(ctx context.Context) error {
	return ghs.checkAndWait(ctx, ghs.restRateLimit, "REST")
}

// checkAndWait performs proactive rate limit checking for the given rate limit state.
// If remaining points are below threshold and proactive retry is enabled,
// waits until rate limit resets. Returns error if context cancelled.
func (ghs *githubScraper) checkAndWait(ctx context.Context, rateLimit *rateLimitState, apiType string) error {
	if !ghs.cfg.ProactiveRetry.Enabled {
		return nil
	}

	if !rateLimit.needsProactiveWait(ghs.cfg.ProactiveRetry.Threshold) {
		return nil
	}

	limit, remaining, used, resetAt := rateLimit.get()
	waitDuration := rateLimit.waitDuration()

	ghs.logger.Warn(
		"Proactive rate limit wait triggered",
		zap.String("api", apiType),
		zap.Int("remaining", remaining),
		zap.Int("limit", limit),
		zap.Int("used", used),
		zap.Int("threshold", ghs.cfg.ProactiveRetry.Threshold),
		zap.Time("reset_at", resetAt),
		zap.Duration("wait_duration", waitDuration),
	)

	select {
	case <-time.After(waitDuration):
		ghs.logger.Info(
			"Proactive rate limit wait completed",
			zap.String("api", apiType),
			zap.Duration("waited", waitDuration),
		)
		return nil
	case <-ctx.Done():
		ghs.logger.Debug(
			"Proactive wait interrupted by context cancellation",
			zap.String("api", apiType),
		)
		return ctx.Err()
	}
}

// isRetryableError determines if an error should trigger exponential backoff retry.
// Returns true for:
// - HTTP 403 with rate limit indication
// - HTTP 429 (secondary rate limit)
// - HTTP 5xx (server errors)
// - Network/timeout errors
// Returns false for:
// - HTTP 4xx (except 403 rate limit and 429)
// - Context cancellation
// - nil errors
// - Unknown errors (logged for investigation)
func isRetryableError(err error, logger *zap.Logger) bool {
	if err == nil {
		return false
	}

	// Context cancellation is not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for GraphQL HTTP error (genqlient wraps HTTP errors)
	var gqlErr *graphql.HTTPError
	if errors.As(err, &gqlErr) {
		return isRetryableStatusCode(gqlErr.StatusCode, err)
	}

	// Check for go-github ErrorResponse which implements Response() *http.Response
	var httpErr interface{ Response() *http.Response }
	if errors.As(err, &httpErr) {
		if resp := httpErr.Response(); resp != nil {
			statusCode := resp.StatusCode

			// Special case: 403 with X-RateLimit-Remaining header
			if statusCode == http.StatusForbidden {
				if resp.Header.Get("X-RateLimit-Remaining") == "0" {
					return true
				}
			}

			return isRetryableStatusCode(statusCode, err)
		}
	}

	// Network errors, timeouts, DNS failures are retryable
	var urlErr interface{ Timeout() bool }
	if errors.As(err, &urlErr) {
		return true
	}

	// Unknown error type - log for investigation and don't retry
	logger.Debug(
		"Unknown error type encountered, not retrying",
		zap.Error(err),
		zap.String("error_type", fmt.Sprintf("%T", err)),
	)
	return false
}

// isRetryableStatusCode determines if an HTTP status code is retryable.
func isRetryableStatusCode(statusCode int, err error) bool {
	// 429 Too Many Requests (secondary rate limit)
	if statusCode == http.StatusTooManyRequests {
		return true
	}

	// 403 Forbidden - check if it's rate limit related
	if statusCode == http.StatusForbidden {
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "api rate limit exceeded") {
			return true
		}
		return false
	}

	// 5xx server errors are retryable
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	// 4xx client errors are NOT retryable
	if statusCode >= 400 && statusCode < 500 {
		return false
	}

	// Non-error status codes or unknown - don't retry
	return false
}

// withRetry wraps a function with exponential backoff retry logic.
// Retries on retryable errors (5xx, 429, 403 rate limit, network errors).
// Stops retrying on permanent errors (4xx except 429, context cancellation).
func (ghs *githubScraper) withRetry(ctx context.Context, operation func() error) error {
	if !ghs.cfg.RetryOnFailure.Enabled {
		return operation()
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = defaultRetryInitialInterval
	expBackoff.MaxInterval = defaultRetryMaxInterval

	attempt := 0
	retryFunc := func() (struct{}, error) {
		attempt++
		err := operation()

		if err == nil {
			return struct{}{}, nil
		}

		// Context cancellation takes priority - stop immediately
		if ctx.Err() != nil {
			return struct{}{}, backoff.Permanent(ctx.Err())
		}

		if !isRetryableError(err, ghs.logger) {
			ghs.logger.Debug(
				"Non-retryable error encountered, stopping retry",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			return struct{}{}, backoff.Permanent(err)
		}

		ghs.logger.Info(
			"Retryable error encountered, will retry",
			zap.Error(err),
			zap.Int("attempt", attempt),
		)

		return struct{}{}, err
	}

	_, err := backoff.Retry(
		ctx,
		retryFunc,
		backoff.WithBackOff(expBackoff),
		backoff.WithMaxElapsedTime(defaultRetryMaxElapsedTime),
	)
	if err != nil {
		ghs.logger.Warn(
			"Operation failed after retries",
			zap.Error(err),
			zap.Int("total_attempts", attempt),
		)
	}

	return err
}
