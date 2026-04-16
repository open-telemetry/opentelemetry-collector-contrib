// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
	"go.uber.org/zap"
)

// fastRetryConfig returns a RetryConfig with sub-millisecond backoff and no
// randomization, suitable for deterministic, fast tests.
func fastRetryConfig() RetryConfig {
	return RetryConfig{
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     1 * time.Millisecond,
			RandomizationFactor: 0, // deterministic
			Multiplier:          1.5,
			MaxInterval:         5 * time.Millisecond,
		},
		MaxRetries: 10,
	}
}

// newTestRetryRT creates a retryRoundTripper with fast, deterministic backoff.
func newTestRetryRT(base http.RoundTripper) *retryRoundTripper {
	return &retryRoundTripper{
		base:   base,
		cfg:    fastRetryConfig(),
		logger: zap.NewNop(),
	}
}

// sequenceHandler returns a handler that responds with the given status codes
// in order, returning 200 once the sequence is exhausted.
func sequenceHandler(statuses ...int) (http.Handler, *atomic.Int32) {
	var call atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		idx := int(call.Add(1)) - 1
		if idx < len(statuses) {
			w.WriteHeader(statuses[idx])
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return handler, &call
}

func TestRetryOn502(t *testing.T) {
	handler, calls := sequenceHandler(
		http.StatusBadGateway,
		http.StatusBadGateway,
	)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), calls.Load()) // 2 retries + 1 success
}

func TestRetryOn503(t *testing.T) {
	handler, calls := sequenceHandler(http.StatusServiceUnavailable)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load()) // 1 retry + 1 success
}

func TestRetryOn504(t *testing.T) {
	handler, calls := sequenceHandler(http.StatusGatewayTimeout)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load()) // 1 retry + 1 success
}

func TestRetryOn429(t *testing.T) {
	handler, calls := sequenceHandler(http.StatusTooManyRequests)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), calls.Load())
}

func TestRetryOn403WithRetryAfter(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if call.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Retry-After of 1s is respected as-is (not capped).
	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(2), call.Load())
}

func TestNoRetryOn403WithoutRetryAfter(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, int32(1), call.Load()) // No retry
}

func TestNoRetryOn404(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, int32(1), call.Load()) // No retry
}

func TestNoRetryOnSuccess(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt := newTestRetryRT(http.DefaultTransport)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), call.Load()) // No retry
}

func TestNoRetryWhenDisabled(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	cfg := fastRetryConfig()
	cfg.Enabled = false
	rt := &retryRoundTripper{
		base:   http.DefaultTransport,
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, int32(1), call.Load()) // No retry
}

func TestMaxRetriesExceeded(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call.Add(1)
		w.WriteHeader(http.StatusBadGateway) // Always fail
	}))
	defer srv.Close()

	cfg := fastRetryConfig()
	cfg.MaxRetries = 3
	rt := &retryRoundTripper{
		base:   http.DefaultTransport,
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	client := &http.Client{Transport: rt}

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, int32(4), call.Load()) // 1 initial + 3 retries
}

func TestRetryContextCancellation(t *testing.T) {
	// Server always returns 502 so retries never succeed.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	cfg := fastRetryConfig()
	cfg.MaxRetries = 0 // unlimited — context cancellation is the only bound
	rt := &retryRoundTripper{
		base:   http.DefaultTransport,
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	client := &http.Client{Transport: rt}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
