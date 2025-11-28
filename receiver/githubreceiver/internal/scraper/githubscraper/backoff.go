// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"context"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// graphqlCallWithRetry wraps a GraphQL API call with exponential backoff
// retrying on rate limit errors (403, 429) and transient failures
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

		if err == nil {
			return resp, nil
		}

		if !isRetriableError(err) {
			return nil, backoff.Permanent(err)
		}

		// Log retry attempt with warning level
		logger.Warn("GitHub API rate limited or temporary failure, retrying",
			zap.Int64("attempt", attempts),
			zap.Error(err),
		)

		// Record retry metric via internal telemetry
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

// isRetriableError determines if an error should trigger a retry
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// GraphQL rate limit errors
	if strings.Contains(errStr, "API rate limit exceeded") {
		return true
	}

	// HTTP status code errors
	if strings.Contains(errStr, "returned error 403") ||
		strings.Contains(errStr, "returned error 429") ||
		strings.Contains(errStr, "returned error 502") ||
		strings.Contains(errStr, "returned error 503") ||
		strings.Contains(errStr, "returned error 504") {
		return true
	}

	// Secondary rate limit
	if strings.Contains(errStr, "secondary rate limit") {
		return true
	}

	return false
}
