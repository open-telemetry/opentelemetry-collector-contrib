// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	internalrl "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter/internal/ratelimit"
)

var errRateLimited = errors.New("error sending data, rate-limited")

type dataCategory string

const (
	dataCategoryTrace dataCategory = "trace"
	dataCategoryLog   dataCategory = "log"
)

// rateLimiter keeps track of rate limit maps per DSN.
type rateLimiter struct {
	mu        sync.Mutex
	dsnLimits map[string]internalrl.Map
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		dsnLimits: make(map[string]internalrl.Map),
	}
}

// isRateLimited returns the remaining backoff for the given category.
func (r *rateLimiter) isRateLimited(dsn string, category dataCategory, now time.Time) (time.Duration, bool) {
	internalCategory := mapCategory(category)
	if internalCategory == internalrl.CategoryUnknown {
		return 0, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.dsnLimits[dsn]
	if !ok {
		return 0, false
	}

	deadline := m.Deadline(internalCategory)
	if !deadline.After(internalrl.Deadline(now)) {
		if deadline.Equal(internalrl.Deadline{}) {
			return 0, false
		}
		// cleanup expired map entries
		delete(m, internalCategory)
		delete(m, internalrl.CategoryAll)
		if len(m) == 0 {
			delete(r.dsnLimits, dsn)
		}
		return 0, false
	}

	return time.Until(time.Time(deadline)), true
}

// updateFromResponse updates rate limits using Sentry response headers.
func (r *rateLimiter) updateFromResponse(dsn string, resp *http.Response, now time.Time) {
	limits := internalrl.FromResponse(resp)
	if len(limits) == 0 && resp.StatusCode == http.StatusTooManyRequests {
		limits = internalrl.Map{internalrl.CategoryAll: internalrl.Deadline(now.Add(internalrl.DefaultRetryAfter))}
	}
	if len(limits) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.dsnLimits[dsn]; !ok {
		r.dsnLimits[dsn] = make(internalrl.Map)
	}
	r.dsnLimits[dsn].Merge(limits)
}

func mapCategory(category dataCategory) internalrl.Category {
	switch category {
	case dataCategoryTrace:
		return internalrl.CategoryTransaction
	case dataCategoryLog:
		return internalrl.CategoryLog
	default:
		return internalrl.CategoryUnknown
	}
}

// parseXSentryRateLimitReset parses the X-Sentry-Rate-Limit-Reset header from the Sentry API.
func parseXSentryRateLimitReset(reset string) time.Duration {
	retryAfter := internalrl.DefaultRetryAfter

	if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
		resetAt := time.Unix(resetTime, 0)
		retryAfter = time.Until(resetAt)
		retryAfter = max(retryAfter, 0)
	}
	return retryAfter
}
