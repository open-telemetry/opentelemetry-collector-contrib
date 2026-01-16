// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

func TestRateLimitState_Update(t *testing.T) {
	state := &rateLimitState{}
	resetAt := time.Now().Add(time.Hour)

	state.update(10, 5000, 4990, 10, resetAt)

	limit, remaining, used, gotResetAt := state.get()
	assert.Equal(t, 5000, limit)
	assert.Equal(t, 4990, remaining)
	assert.Equal(t, 10, used)
	assert.Equal(t, resetAt.Unix(), gotResetAt.Unix())
}

func TestRateLimitState_UpdateFromRest(t *testing.T) {
	state := &rateLimitState{}
	reset := time.Now().Add(time.Hour)

	state.updateFromRest(5000, 4500, reset)

	limit, remaining, used, resetAt := state.get()
	assert.Equal(t, 5000, limit)
	assert.Equal(t, 4500, remaining)
	assert.Equal(t, 500, used) // calculated as limit - remaining
	assert.Equal(t, reset.Unix(), resetAt.Unix())
}

func TestRateLimitState_NeedsProactiveWait(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		threshold int
		updated   bool
		want      bool
	}{
		{
			name:      "no data yet",
			remaining: 50,
			threshold: 100,
			updated:   false,
			want:      false,
		},
		{
			name:      "remaining below threshold",
			remaining: 50,
			threshold: 100,
			updated:   true,
			want:      true,
		},
		{
			name:      "remaining above threshold",
			remaining: 150,
			threshold: 100,
			updated:   true,
			want:      false,
		},
		{
			name:      "remaining equals threshold",
			remaining: 100,
			threshold: 100,
			updated:   true,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &rateLimitState{}
			if tt.updated {
				state.update(1, 5000, tt.remaining, 5000-tt.remaining, time.Now().Add(time.Hour))
			}
			got := state.needsProactiveWait(tt.threshold)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRateLimitState_WaitDuration(t *testing.T) {
	tests := []struct {
		name    string
		resetAt time.Time
		wantGt  time.Duration
		wantLt  time.Duration
	}{
		{
			name:    "future reset",
			resetAt: time.Now().Add(30 * time.Second),
			wantGt:  25 * time.Second,
			wantLt:  35 * time.Second,
		},
		{
			name:    "past reset",
			resetAt: time.Now().Add(-10 * time.Second),
			wantGt:  -1,
			wantLt:  1 * time.Millisecond,
		},
		{
			name:    "zero reset",
			resetAt: time.Time{},
			wantGt:  -1,
			wantLt:  1 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &rateLimitState{}
			state.update(1, 5000, 100, 4900, tt.resetAt)
			got := state.waitDuration()
			assert.Greater(t, got, tt.wantGt)
			assert.Less(t, got, tt.wantLt)
		})
	}
}

func TestRateLimitState_Concurrency(_ *testing.T) {
	state := &rateLimitState{}
	done := make(chan struct{})

	// Writer goroutine
	go func() {
		for i := range 1000 {
			state.update(i, 5000, 5000-i, i, time.Now().Add(time.Hour))
		}
		close(done)
	}()

	// Reader goroutine - should not race or panic
	for range 1000 {
		_, _, _, _ = state.get()
		_ = state.needsProactiveWait(100)
		_ = state.waitDuration()
	}

	<-done
}

func TestIsRetryableError(t *testing.T) {
	logger := receivertest.NewNopSettings(metadata.Type).Logger

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "graphql 429 error",
			err:  &graphql.HTTPError{StatusCode: http.StatusTooManyRequests},
			want: true,
		},
		{
			name: "graphql 500 error",
			err:  &graphql.HTTPError{StatusCode: http.StatusInternalServerError},
			want: true,
		},
		{
			name: "graphql 502 error",
			err:  &graphql.HTTPError{StatusCode: http.StatusBadGateway},
			want: true,
		},
		{
			name: "graphql 503 error",
			err:  &graphql.HTTPError{StatusCode: http.StatusServiceUnavailable},
			want: true,
		},
		{
			name: "graphql 404 error",
			err:  &graphql.HTTPError{StatusCode: http.StatusNotFound},
			want: false,
		},
		{
			name: "graphql 401 error",
			err:  &graphql.HTTPError{StatusCode: http.StatusUnauthorized},
			want: false,
		},
		{
			name: "graphql 403 without rate limit message",
			err:  &graphql.HTTPError{StatusCode: http.StatusForbidden},
			want: false,
		},
		{
			name: "unknown error",
			err:  errors.New("some random error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err, logger)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errMsg     string
		want       bool
	}{
		{
			name:       "429 Too Many Requests",
			statusCode: http.StatusTooManyRequests,
			errMsg:     "rate limited",
			want:       true,
		},
		{
			name:       "403 with rate limit message",
			statusCode: http.StatusForbidden,
			errMsg:     "API rate limit exceeded",
			want:       true,
		},
		{
			name:       "403 without rate limit",
			statusCode: http.StatusForbidden,
			errMsg:     "access denied",
			want:       false,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			errMsg:     "server error",
			want:       true,
		},
		{
			name:       "502 Bad Gateway",
			statusCode: http.StatusBadGateway,
			errMsg:     "bad gateway",
			want:       true,
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			errMsg:     "service unavailable",
			want:       true,
		},
		{
			name:       "404 Not Found",
			statusCode: http.StatusNotFound,
			errMsg:     "not found",
			want:       false,
		},
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			errMsg:     "unauthorized",
			want:       false,
		},
		{
			name:       "200 OK",
			statusCode: http.StatusOK,
			errMsg:     "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			got := isRetryableStatusCode(tt.statusCode, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWithRetry_Disabled(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.RetryOnFailure.Enabled = false

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	var callCount int
	err := ghs.withRetry(t.Context(), func() error {
		callCount++
		return errors.New("test error")
	})

	assert.Error(t, err)
	assert.Equal(t, 1, callCount, "should only call once when retry is disabled")
}

func TestWithRetry_Success(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	var callCount int
	err := ghs.withRetry(t.Context(), func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestWithRetry_RetryableError(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	var callCount atomic.Int32
	err := ghs.withRetry(t.Context(), func() error {
		count := callCount.Add(1)
		if count < 3 {
			return &graphql.HTTPError{StatusCode: http.StatusInternalServerError}
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(3), callCount.Load(), "should retry until success")
}

func TestWithRetry_PermanentError(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	var callCount int
	err := ghs.withRetry(t.Context(), func() error {
		callCount++
		return &graphql.HTTPError{StatusCode: http.StatusNotFound}
	})

	assert.Error(t, err)
	assert.Equal(t, 1, callCount, "should not retry permanent errors")
}

func TestWithRetry_ContextCancellation(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	ctx, cancel := context.WithCancel(t.Context())

	var callCount atomic.Int32
	err := ghs.withRetry(ctx, func() error {
		count := callCount.Add(1)
		if count == 2 {
			cancel()
		}
		return &graphql.HTTPError{StatusCode: http.StatusInternalServerError}
	})

	assert.Error(t, err)
	// Should stop retrying after context is cancelled
	assert.LessOrEqual(t, callCount.Load(), int32(3))
}

func TestCheckAndWait_Disabled(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ProactiveRetry.Enabled = false

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	// Set rate limit to below threshold
	ghs.graphQLRateLimit.update(1, 5000, 10, 4990, time.Now().Add(time.Hour))

	start := time.Now()
	err := ghs.checkAndWaitGraphQL(t.Context())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 100*time.Millisecond, "should not wait when disabled")
}

func TestCheckAndWait_NotNeeded(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	// Set rate limit above threshold
	ghs.graphQLRateLimit.update(1, 5000, 500, 4500, time.Now().Add(time.Hour))

	start := time.Now()
	err := ghs.checkAndWaitGraphQL(t.Context())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, 100*time.Millisecond, "should not wait when above threshold")
}

func TestCheckAndWait_ContextCancellation(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ProactiveRetry.Threshold = 1000

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	// Set rate limit below threshold with reset far in future
	ghs.graphQLRateLimit.update(1, 5000, 10, 4990, time.Now().Add(time.Hour))

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := ghs.checkAndWaitGraphQL(ctx)
	elapsed := time.Since(start)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 200*time.Millisecond, "should return quickly on context cancellation")
}

func TestCheckAndWait_WaitsUntilReset(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ProactiveRetry.Threshold = 1000

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	// Set rate limit below threshold with reset very soon
	resetTime := time.Now().Add(100 * time.Millisecond)
	ghs.graphQLRateLimit.update(1, 5000, 10, 4990, resetTime)

	start := time.Now()
	err := ghs.checkAndWaitGraphQL(t.Context())
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond, "should wait for reset")
	assert.Less(t, elapsed, 300*time.Millisecond, "should not wait too long")
}

func TestSeparateRateLimits(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	// Update GraphQL rate limit
	ghs.graphQLRateLimit.update(1, 5000, 4000, 1000, time.Now().Add(time.Hour))

	// Update REST rate limit with different values
	ghs.restRateLimit.updateFromRest(5000, 3000, time.Now().Add(30*time.Minute))

	// Verify they are separate
	gqlLimit, gqlRemaining, _, _ := ghs.graphQLRateLimit.get()
	restLimit, restRemaining, _, _ := ghs.restRateLimit.get()

	assert.Equal(t, 5000, gqlLimit)
	assert.Equal(t, 4000, gqlRemaining)
	assert.Equal(t, 5000, restLimit)
	assert.Equal(t, 3000, restRemaining)
}

func TestCheckAndWaitGraphQL_UsesCorrectState(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ProactiveRetry.Threshold = 1000

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	// GraphQL: below threshold (should trigger wait if used)
	ghs.graphQLRateLimit.update(1, 5000, 10, 4990, time.Now().Add(50*time.Millisecond))
	// REST: above threshold (should not trigger wait)
	ghs.restRateLimit.updateFromRest(5000, 4500, time.Now().Add(time.Hour))

	start := time.Now()
	err := ghs.checkAndWaitGraphQL(t.Context())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 30*time.Millisecond, "should wait for GraphQL reset")
}

func TestCheckAndWaitREST_UsesCorrectState(t *testing.T) {
	factory := Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ProactiveRetry.Threshold = 1000

	ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	// GraphQL: above threshold (should not trigger wait)
	ghs.graphQLRateLimit.update(1, 5000, 4500, 500, time.Now().Add(time.Hour))
	// REST: below threshold (should trigger wait if used)
	ghs.restRateLimit.updateFromRest(5000, 10, time.Now().Add(50*time.Millisecond))

	start := time.Now()
	err := ghs.checkAndWaitREST(t.Context())
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 30*time.Millisecond, "should wait for REST reset")
}
