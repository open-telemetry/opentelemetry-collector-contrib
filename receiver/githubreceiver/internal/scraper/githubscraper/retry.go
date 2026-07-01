// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// retryRoundTripper wraps an http.RoundTripper and retries on transient GitHub
// API errors (429, 502, 503, 504), secondary rate limits (403 + Retry-After),
// and primary rate limits (403/429 + X-RateLimit-Remaining: 0).
// Retries use exponential backoff with jitter and are bounded by MaxRetries,
// MaxElapsedTime, and the request context (cancelled when the scrape cycle ends).
//
// RoundTrip is safe for concurrent use. The optional now and sleep test seams
// must be set before the round-tripper is first used and never reassigned
// after; they are read without synchronization on the hot path.
type retryRoundTripper struct {
	base   http.RoundTripper
	cfg    RetryConfig
	logger *zap.Logger

	// Test seams. Both nil in production. Set once at construction in tests;
	// must not be mutated after the first call to RoundTrip.
	now   func() time.Time
	sleep func(time.Duration) <-chan time.Time
}

func (rt *retryRoundTripper) nowFn() time.Time {
	if rt.now != nil {
		return rt.now()
	}
	return time.Now()
}

// wait blocks until d elapses or ctx is cancelled, returning ctx.Err() in the
// latter case. In production the wait uses time.NewTimer with a deferred Stop
// so a cancelled long wait (e.g. an hour-long primary-rate-limit reset) does
// not pin the underlying runtime timer until it fires. Tests inject rt.sleep
// to skip the wait entirely.
func (rt *retryRoundTripper) wait(ctx context.Context, d time.Duration) error {
	if rt.sleep != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-rt.sleep(d):
			return nil
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.base.RoundTrip(req)
	if err != nil || !rt.cfg.Enabled {
		return resp, err
	}

	primaryRL := isPrimaryRateLimit(resp)
	if !isRetryable(resp, primaryRL) {
		return resp, nil
	}

	b := &backoff.ExponentialBackOff{
		InitialInterval:     rt.cfg.InitialInterval,
		RandomizationFactor: rt.cfg.RandomizationFactor,
		Multiplier:          rt.cfg.Multiplier,
		MaxInterval:         rt.cfg.MaxInterval,
	}
	b.Reset()

	start := rt.nowFn()
	wasPrimaryRL := false
	for attempt := 0; ; attempt++ {
		if !isRetryable(resp, primaryRL) {
			break
		}
		if rt.cfg.MaxRetries > 0 && attempt >= rt.cfg.MaxRetries {
			break
		}
		if rt.cfg.MaxElapsedTime > 0 && rt.nowFn().Sub(start) >= rt.cfg.MaxElapsedTime {
			break
		}

		delay := b.NextBackOff()

		// Honor server-supplied wait hints. The values are used as-is
		// (not capped) because retrying before the server's requested
		// delay just wastes an attempt. Context cancellation and
		// MaxRetries already bound total retry behavior. The
		// primary-rate-limit branch (X-RateLimit-Remaining: 0) takes
		// precedence over the secondary-rate-limit Retry-After path.
		if primaryRL {
			d := parseRateLimitReset(resp.Header, rt.nowFn())
			if d <= 0 {
				// No usable reset hint — the budget is exhausted but
				// we have no idea when it refills. Burning the
				// remaining MaxRetries against the same closed window
				// just wastes API quota and ends in failure anyway.
				break
			}
			delay = d
			// Only reset backoff on transition into primary-RL handling
			// so an alternating primary-RL / transient-error sequence
			// does not wipe the exponential schedule each pass.
			if !wasPrimaryRL {
				b.Reset()
			}
		} else if ra := parseRetryAfter(resp.Header); ra > 0 {
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

		if err = rt.wait(req.Context(), delay); err != nil {
			return nil, err
		}

		// Reset request body for retry (genqlient uses bytes.NewReader which
		// auto-sets GetBody, making POST bodies replayable).
		if req.GetBody != nil {
			req.Body, err = req.GetBody()
			if err != nil {
				return nil, err
			}
		}

		wasPrimaryRL = primaryRL
		resp, err = rt.base.RoundTrip(req)
		if err != nil {
			return resp, err
		}
		primaryRL = isPrimaryRateLimit(resp)
	}

	return resp, nil
}

// isRetryable returns true for HTTP status codes that indicate a transient
// GitHub API error worth retrying. primaryRL is the precomputed result of
// isPrimaryRateLimit for the same response — passed in so callers can avoid
// recomputing the header check.
func isRetryable(resp *http.Response, primaryRL bool) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	case http.StatusForbidden: // 403 -- secondary rate limit (Retry-After) or primary rate limit (X-RateLimit-Remaining: 0)
		return resp.Header.Get("Retry-After") != "" || primaryRL
	}
	return false
}

// isPrimaryRateLimit reports whether the response indicates GitHub's primary
// rate-limit budget is exhausted. GitHub signals this with status 403 (or 429)
// plus X-RateLimit-Remaining: 0 and an X-RateLimit-Reset epoch.
func isPrimaryRateLimit(resp *http.Response) bool {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return false
	}
	return resp.Header.Get("X-RateLimit-Remaining") == "0"
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

// parseRateLimitReset computes how long to wait before retrying a primary
// rate-limited request, derived from GitHub's X-RateLimit-Reset header (a
// Unix epoch in seconds). Returns 0 if the header is absent, unparseable, or
// already in the past relative to now.
func parseRateLimitReset(h http.Header, now time.Time) time.Duration {
	v := h.Get("X-RateLimit-Reset")
	if v == "" {
		return 0
	}
	epoch, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	d := time.Unix(epoch, 0).Sub(now)
	if d < 0 {
		return 0
	}
	return d
}
