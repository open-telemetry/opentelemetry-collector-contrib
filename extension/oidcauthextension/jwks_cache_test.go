// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cacheTestVerifier(t *testing.T, url string, now func() time.Time) *oidc.IDTokenVerifier {
	t.Helper()
	keys := newExpiringRemoteKeySet(t.Context(), url)
	keys.now = now
	return oidc.NewVerifier(url, keys, &oidc.Config{ClientID: "collector", Now: now})
}

func TestJWKSCacheExpiresDespiteContinuedUse(t *testing.T) {
	t.Parallel()
	keyA, err := createPrivateKey()
	require.NoError(t, err)
	keyB, err := createPrivateKey()
	require.NoError(t, err)
	var body atomic.Value
	body.Store(jwksResponse(t, keyA, "a"))
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(body.Load().([]byte))
	}))
	t.Cleanup(server.Close)
	now := time.Now()
	verifier := cacheTestVerifier(t, server.URL, func() time.Time { return now })
	claims := jwksClaims(server.URL)
	claims["exp"] = now.Add(time.Hour).Unix()
	tokenA := jwksToken(t, keyA, "a", claims)
	_, err = verifier.Verify(t.Context(), tokenA)
	require.NoError(t, err)
	body.Store(jwksResponse(t, keyB, "b"))
	// Keep presenting the same unexpired token. No new kid arrives to trigger
	// go-oidc's normal rotation handling, and cache hits must not extend the TTL.
	for range 4 {
		now = now.Add(time.Minute)
		_, err = verifier.Verify(t.Context(), tokenA)
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, requests.Load())
	now = now.Add(time.Minute)
	_, err = verifier.Verify(t.Context(), tokenA)
	require.Error(t, err, "a removed key must stop authenticating after five minutes")
	assert.EqualValues(t, 2, requests.Load())
	tokenB := jwksToken(t, keyB, "b", claims)
	_, err = verifier.Verify(t.Context(), tokenB)
	require.NoError(t, err, "the refreshed key must authenticate without a restart")
}

func TestJWKSCacheExpiredRefreshFailsClosed(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	good := jwksResponse(t, key, "key")
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"unavailable", http.StatusServiceUnavailable, "unavailable"},
		{"malformed", http.StatusOK, "not JSON"},
		{"empty keys", http.StatusOK, `{"keys":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var failed atomic.Bool
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				if failed.Load() {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				_, _ = w.Write(good)
			}))
			t.Cleanup(server.Close)
			now := time.Now()
			verifier := cacheTestVerifier(t, server.URL, func() time.Time { return now })
			claims := jwksClaims(server.URL)
			claims["exp"] = now.Add(time.Hour).Unix()
			token := jwksToken(t, key, "key", claims)
			_, err := verifier.Verify(t.Context(), token)
			require.NoError(t, err)
			failed.Store(true)
			now = now.Add(jwksCacheTTL)
			_, err = verifier.Verify(t.Context(), token)
			require.Error(t, err, "expired cached keys must not be a fallback when refresh fails")
			assert.EqualValues(t, 2, requests.Load())
			failed.Store(false)
			_, err = verifier.Verify(t.Context(), token)
			require.NoError(t, err, "authentication must recover when the endpoint recovers")
		})
	}
}

func TestJWKSCacheConcurrentExpiry(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	now := time.Now()
	verifier := cacheTestVerifier(t, server.URL, func() time.Time { return now })
	claims := jwksClaims(server.URL)
	claims["exp"] = now.Add(time.Hour).Unix()
	token := jwksToken(t, key, "key", claims)
	_, err = verifier.Verify(t.Context(), token)
	require.NoError(t, err)
	now = now.Add(jwksCacheTTL)
	errors := make(chan error, 16)
	for range cap(errors) {
		go func() {
			_, err := verifier.Verify(t.Context(), token)
			errors <- err
		}()
	}
	for range cap(errors) {
		require.NoError(t, <-errors)
	}
}
