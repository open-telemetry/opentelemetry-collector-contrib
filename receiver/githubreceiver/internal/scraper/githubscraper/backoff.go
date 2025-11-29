// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/cenkalti/backoff/v5"
	"github.com/google/go-github/v79/github"
	"go.opentelemetry.io/collector/component"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.uber.org/zap"
)

// rateLimitInfo is an interface for accessing GitHub rate limit metadata
type rateLimitInfo interface {
	GetLimit() int
	GetCost() int
	GetRemaining() int
	GetResetAt() time.Time
}

// graphqlCallWithRetry wraps a GraphQL API call with exponential backoff,
// proactively detecting rate limits from response metadata (if available) and
// retrying on rate limit errors and transient failures
func graphqlCallWithRetry[T any](
	ctx context.Context,
	logger *zap.Logger,
	telemetry component.TelemetrySettings,
	scrapeInterval time.Duration,
	apiCall func() (*T, error),
) (*T, error) {
	expBackoff := &backoff.ExponentialBackOff{
		InitialInterval:     1 * time.Second,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         30 * time.Second,
	}

	var attempts int64
	operation := func() (*T, error) {
		attempts++
		resp, err := apiCall()

		// Check rate limit metadata from successful responses if available
		if resp != nil {
			// Try to extract rate limit info using type assertion
			type rateLimitGetter interface {
				GetRateLimit() rateLimitInfo
			}

			if rlGetter, ok := any(resp).(rateLimitGetter); ok {
				rateLimit := rlGetter.GetRateLimit()

				// Log rate limit status for observability
				logger.Debug("GitHub API rate limit status",
					zap.Int("limit", rateLimit.GetLimit()),
					zap.Int("remaining", rateLimit.GetRemaining()),
					zap.Int("cost", rateLimit.GetCost()),
					zap.Time("reset_at", rateLimit.GetResetAt()),
				)

				// Proactive rate limit detection: check if next query would exceed quota
				if rateLimit.GetCost() >= rateLimit.GetRemaining() {
					waitDuration := time.Until(rateLimit.GetResetAt())

					logger.Warn("Approaching GitHub API rate limit, waiting for reset",
						zap.Int("remaining", rateLimit.GetRemaining()),
						zap.Int("cost", rateLimit.GetCost()),
						zap.Duration("wait_duration", waitDuration),
						zap.Time("reset_at", rateLimit.GetResetAt()),
					)

					// Record proactive rate limit wait metric
					counter, meterErr := telemetry.MeterProvider.Meter("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver").
						Int64Counter("github.rate_limit.proactive_wait")
					if meterErr == nil {
						counter.Add(ctx, 1)
					}

					// Wait exactly until reset time
					return nil, &backoff.RetryAfterError{
						Duration: waitDuration,
					}
				}
			}
		}

		if err == nil {
			return resp, nil
		}

		// Type-safe error handling for HTTP errors
		if !isRetriableError(err) {
			return nil, backoff.Permanent(err)
		}

		// Log retry attempt with warning level
		logger.Warn("GitHub API rate limited or temporary failure, retrying",
			zap.Int64("attempt", attempts),
			zap.Error(err),
		)

		// Record retry metric
		counter, meterErr := telemetry.MeterProvider.Meter("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver").
			Int64Counter("http.request.resend_count")
		if meterErr == nil {
			counter.Add(ctx, 1)
		}

		return nil, err
	}

	// Execute with backoff
	result, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(expBackoff),
		backoff.WithMaxElapsedTime(scrapeInterval))
	if err != nil {
		// Max elapsed time exceeded - log error
		logger.Error("GitHub API call failed after retries",
			zap.Int64("total_attempts", attempts),
			zap.Error(err),
		)

		// Record failure metric
		counter, meterErr := telemetry.MeterProvider.Meter("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver").
			Int64Counter(string(semconv.HTTPRequestResendCountKey))
		if meterErr == nil {
			counter.Add(ctx, 1)
		}
	}

	return result, err
}

// isRetriableError determines if an error should trigger a retry by checking
// structured error types for HTTP status codes, with fallback to string matching
// for GraphQL-level errors
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for genqlient HTTPError (from GraphQL API calls)
	var httpErr *graphql.HTTPError
	if errors.As(err, &httpErr) {
		return isRetriableStatusCode(httpErr.StatusCode)
	}

	// Check for go-github ErrorResponse (from REST API calls)
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil {
		return isRetriableStatusCode(ghErr.Response.StatusCode)
	}

	// Fallback to string matching for GraphQL-level errors
	// These are errors in the GraphQL response body (HTTP 200 with errors field)
	// rather than HTTP-level errors
	errStr := err.Error()

	// GraphQL rate limit errors
	if strings.Contains(errStr, "API rate limit exceeded") {
		return true
	}

	// Secondary rate limit
	if strings.Contains(errStr, "secondary rate limit") {
		return true
	}

	return false
}

// isRetriableStatusCode checks if an HTTP status code should trigger a retry.
// Retriable status codes include rate limits and transient server errors.
func isRetriableStatusCode(statusCode int) bool {
	switch statusCode {
	case 403: // Forbidden (often used for rate limiting by GitHub)
		return true
	case 429: // Too Many Requests (explicit rate limit)
		return true
	case 502: // Bad Gateway (transient server error)
		return true
	case 503: // Service Unavailable (transient server error)
		return true
	case 504: // Gateway Timeout (transient server error)
		return true
	default:
		return false
	}
}
