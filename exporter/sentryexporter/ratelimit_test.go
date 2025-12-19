// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_XSentryRateLimits(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Sentry-Rate-Limits": []string{"1:transaction:key, 2:log_item:project"}},
	}

	rl.updateFromResponse("dsn", resp, now)

	delayTrace, limitedTrace := rl.isRateLimited("dsn", dataCategoryTrace, now)
	assert.True(t, limitedTrace)
	assert.Equal(t, time.Second, delayTrace.Round(time.Second))

	delayLog, limitedLog := rl.isRateLimited("dsn", dataCategoryLog, now)
	assert.True(t, limitedLog)
	assert.Equal(t, 2*time.Second, delayLog.Round(time.Second))
}

func TestRateLimiter_GlobalLimit(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Sentry-Rate-Limits": []string{"5::organization"}},
	}

	rl.updateFromResponse("dsn", resp, now)

	delayTrace, limitedTrace := rl.isRateLimited("dsn", dataCategoryTrace, now)
	delayLog, limitedLog := rl.isRateLimited("dsn", dataCategoryLog, now)

	assert.True(t, limitedTrace)
	assert.True(t, limitedLog)
	assert.Equal(t, 5*time.Second, delayTrace.Round(time.Second))
	assert.Equal(t, 5*time.Second, delayLog.Round(time.Second))
}

func TestRateLimiter_UnknownCategoryUsesRetryAfterDefault(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter()

	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"X-Sentry-Rate-Limits": []string{"60:error:key"}},
	}

	rl.updateFromResponse("dsn", resp, now)

	delayTrace, limitedTrace := rl.isRateLimited("dsn", dataCategoryTrace, now)
	delayLog, limitedLog := rl.isRateLimited("dsn", dataCategoryLog, now)

	assert.True(t, limitedTrace)
	assert.True(t, limitedLog)
	assert.Equal(t, 60*time.Second, delayTrace.Round(time.Second))
	assert.Equal(t, 60*time.Second, delayLog.Round(time.Second))
}

func TestRateLimiter_RetryAfterFallback(t *testing.T) {
	now := time.Now()
	rl := newRateLimiter()

	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{},
	}

	rl.updateFromResponse("dsn", resp, now)

	delayTrace, limitedTrace := rl.isRateLimited("dsn", dataCategoryTrace, now)
	delayLog, limitedLog := rl.isRateLimited("dsn", dataCategoryLog, now)

	assert.True(t, limitedTrace)
	assert.True(t, limitedLog)
	assert.Equal(t, 60*time.Second, delayTrace.Round(time.Second))
	assert.Equal(t, 60*time.Second, delayLog.Round(time.Second))
}
