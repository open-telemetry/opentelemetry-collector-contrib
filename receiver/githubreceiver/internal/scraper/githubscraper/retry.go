// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// retryRoundTripper wraps an http.RoundTripper and retries on transient GitHub
// API errors (429, 502, 503, 504) and secondary rate limits (403 + Retry-After).
// Retries use exponential backoff with jitter and are bounded by MaxRetries,
// MaxElapsedTime, and the request context (cancelled when the scrape cycle ends).
type retryRoundTripper struct {
	base   http.RoundTripper
	cfg    RetryConfig
	logger *zap.Logger
}

func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.base.RoundTrip(req)
	if err != nil || !rt.cfg.Enabled {
		return resp, err
	}

	if !isRetryable(resp) {
		return resp, nil
	}

	b := &backoff.ExponentialBackOff{
		InitialInterval:     rt.cfg.InitialInterval,
		RandomizationFactor: rt.cfg.RandomizationFactor,
		Multiplier:          rt.cfg.Multiplier,
		MaxInterval:         rt.cfg.MaxInterval,
	}
	b.Reset()

	start := time.Now()
	for attempt := 0; isRetryable(resp); attempt++ {
		if rt.cfg.MaxRetries > 0 && attempt >= rt.cfg.MaxRetries {
			break
		}
		if rt.cfg.MaxElapsedTime > 0 && time.Since(start) >= rt.cfg.MaxElapsedTime {
			break
		}

		delay := b.NextBackOff()

		// Honor Retry-After header from GitHub rate limits. The value is
		// used as-is (not capped) because retrying before the server's
		// requested delay just wastes an attempt. Context cancellation
		// and MaxRetries already bound total retry behavior.
		if ra := parseRetryAfter(resp.Header); ra > 0 {
			delay = time.Duration(ra) * time.Second
			b.Reset()
		}

		rt.logger.Debug("retrying GitHub API request",
			zap.String("url", req.URL.String()),
			zap.Int("status", resp.StatusCode),
			zap.Int("attempt", attempt+1),
			zap.Duration("backoff", delay),
		)

		// Drain and close the response body to reuse the TCP connection.
		if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
			rt.logger.Debug("failed to drain response body", zap.Error(drainErr))
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			rt.logger.Debug("failed to close response body", zap.Error(closeErr))
		}

		// Wait for backoff or context cancellation.
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}

		// Reset request body for retry (genqlient uses bytes.NewReader which
		// auto-sets GetBody, making POST bodies replayable).
		if req.GetBody != nil {
			req.Body, err = req.GetBody()
			if err != nil {
				return nil, err
			}
		}

		resp, err = rt.base.RoundTrip(req)
		if err != nil {
			return resp, err
		}
	}

	return resp, nil
}

// isRetryable returns true for HTTP status codes that indicate a transient
// GitHub API error worth retrying.
func isRetryable(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	case http.StatusForbidden: // 403 -- only with Retry-After (secondary rate limit)
		return resp.Header.Get("Retry-After") != ""
	}
	return false
}

// parseRetryAfter extracts the delay in seconds from a Retry-After header.
// Returns 0 if the header is absent or not a valid integer.
func parseRetryAfter(h http.Header) int {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	seconds, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return seconds
}
