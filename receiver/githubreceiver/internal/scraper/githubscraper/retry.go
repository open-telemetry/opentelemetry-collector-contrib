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
// API errors (502, 503, 504, 429) and secondary rate limits (403 + Retry-After).
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

		// Honor Retry-After header from GitHub secondary rate limits,
		// capped to MaxInterval to prevent excessive delays.
		if ra := parseRetryAfter(resp.Header); ra > 0 {
			delay = min(time.Duration(ra)*time.Second, rt.cfg.MaxInterval)
			b.Reset()
		}

		rt.logger.Warn("retrying GitHub API request",
			zap.Int("status", resp.StatusCode),
			zap.Int("attempt", attempt+1),
			zap.Duration("backoff", delay),
		)

		// Drain and close the response body to reuse the TCP connection.
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()

		// Wait for backoff or context cancellation.
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
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
