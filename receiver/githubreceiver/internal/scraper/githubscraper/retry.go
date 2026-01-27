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

// GraphQLRateLimitProvider is implemented by GraphQL response types that contain
// rate limit information. All generated GraphQL response types include a RateLimit
// field that satisfies this interface.
type GraphQLRateLimitProvider interface {
	GetCost() int
	GetLimit() int
	GetRemaining() int
	GetUsed() int
	GetResetAt() time.Time
}

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

// waitInfo holds atomic snapshot of rate limit state for proactive waiting decisions.
type waitInfo struct {
	needsWait bool
	duration  time.Duration
	limit     int
	remaining int
	used      int
	resetAt   time.Time
}

// checkWaitRequired atomically determines if proactive waiting is needed and returns
// all relevant info in a single lock acquisition. This prevents race conditions
// between checking if wait is needed and getting wait parameters.
func (r *rateLimitState) checkWaitRequired(threshold int) waitInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := waitInfo{
		limit:     r.limit,
		remaining: r.remaining,
		used:      r.used,
		resetAt:   r.resetAt,
	}

	// No data yet - don't wait
	if r.lastUpdated.IsZero() {
		return info
	}

	// Above threshold - don't wait
	if r.remaining >= threshold {
		return info
	}

	// Calculate wait duration
	duration := time.Until(r.resetAt)
	if duration <= 0 {
		// Reset time has passed - don't wait
		return info
	}

	info.needsWait = true
	info.duration = duration
	return info
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

	// Atomically check if wait is required and get all wait parameters
	info := rateLimit.checkWaitRequired(ghs.cfg.ProactiveRetry.Threshold)
	if !info.needsWait {
		return nil
	}

	ghs.logger.Warn(
		"Proactive rate limit wait triggered",
		zap.String("api", apiType),
		zap.Int("remaining", info.remaining),
		zap.Int("limit", info.limit),
		zap.Int("used", info.used),
		zap.Int("threshold", ghs.cfg.ProactiveRetry.Threshold),
		zap.Time("reset_at", info.resetAt),
		zap.Duration("wait_duration", info.duration),
	)

	startWait := time.Now()
	select {
	case <-time.After(info.duration):
		ghs.logger.Info(
			"Proactive rate limit wait completed",
			zap.String("api", apiType),
			zap.Duration("waited", time.Since(startWait)),
		)
		return nil
	case <-ctx.Done():
		ghs.logger.Debug(
			"Proactive wait interrupted by context cancellation",
			zap.String("api", apiType),
			zap.Duration("waited", time.Since(startWait)),
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
	if errors.As(err, &urlErr) && urlErr.Timeout() {
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

// withRetryResult wraps a function that returns a value with exponential backoff retry logic.
// This generic version returns (*T, error) and handles rate limit metadata extraction.
// Similar to the original graphqlCallWithRetry pattern but integrated with the scraper's
// configuration and rate limit state management.
func withRetryResult[T any](
	ctx context.Context,
	ghs *githubScraper,
	rateLimit *rateLimitState,
	apiType string,
	apiCall func() (*T, error),
	getRateLimit func(*T) GraphQLRateLimitProvider,
) (*T, error) {
	// Proactive rate limit check before making the call
	if err := ghs.checkAndWait(ctx, rateLimit, apiType); err != nil {
		return nil, err
	}

	// If retry is disabled, make direct call
	if !ghs.cfg.RetryOnFailure.Enabled {
		resp, err := apiCall()
		if err == nil && resp != nil && getRateLimit != nil {
			rl := getRateLimit(resp)
			rateLimit.update(rl.GetCost(), rl.GetLimit(), rl.GetRemaining(), rl.GetUsed(), rl.GetResetAt())
		}
		return resp, err
	}

	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = defaultRetryInitialInterval
	expBackoff.MaxInterval = defaultRetryMaxInterval

	attempt := 0
	operation := func() (*T, error) {
		attempt++
		resp, err := apiCall()

		// Always update rate limit state if we have a response (even with errors).
		// This handles GraphQL's partial success pattern where we get both data and errors.
		if resp != nil && getRateLimit != nil {
			rl := getRateLimit(resp)
			rateLimit.update(rl.GetCost(), rl.GetLimit(), rl.GetRemaining(), rl.GetUsed(), rl.GetResetAt())

			// Check if we're approaching rate limit and should wait
			// This mirrors the original backoff.go proactive detection
			if ghs.cfg.ProactiveRetry.Enabled && rl.GetRemaining() < ghs.cfg.ProactiveRetry.Threshold {
				waitDuration := time.Until(rl.GetResetAt())
				if waitDuration > 0 {
					ghs.logger.Warn(
						"Approaching rate limit, signaling backoff to wait",
						zap.String("api", apiType),
						zap.Int("remaining", rl.GetRemaining()),
						zap.Int("threshold", ghs.cfg.ProactiveRetry.Threshold),
						zap.Duration("wait_duration", waitDuration),
					)
					return nil, &backoff.RetryAfterError{Duration: waitDuration}
				}
			}

			// If we have a valid response, return it along with any error.
			// The caller is responsible for handling partial success cases
			// (e.g., GraphQL queries that return both data and errors).
			// We wrap the error as Permanent to stop retrying since we have usable data.
			if err != nil {
				return resp, backoff.Permanent(err)
			}
			return resp, nil
		}

		// No response - handle as pure error case
		if err == nil {
			return resp, nil
		}

		// Context cancellation takes priority - stop immediately
		if ctx.Err() != nil {
			return nil, backoff.Permanent(ctx.Err())
		}

		if !isRetryableError(err, ghs.logger) {
			ghs.logger.Debug(
				"Non-retryable error encountered, stopping retry",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)
			return nil, backoff.Permanent(err)
		}

		ghs.logger.Info(
			"Retryable error encountered, will retry",
			zap.Error(err),
			zap.Int("attempt", attempt),
		)

		return nil, err
	}

	result, err := backoff.Retry(
		ctx,
		operation,
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

	return result, err
}

// withGraphQLRetry is a convenience wrapper for GraphQL API calls that handles
// rate limit checking, retry logic, and rate limit state updates.
func withGraphQLRetry[T any](
	ctx context.Context,
	ghs *githubScraper,
	apiCall func() (*T, error),
	getRateLimit func(*T) GraphQLRateLimitProvider,
) (*T, error) {
	return withRetryResult(ctx, ghs, ghs.graphQLRateLimit, "GraphQL", apiCall, getRateLimit)
}
