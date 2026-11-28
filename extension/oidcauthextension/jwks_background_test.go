// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type jwksRoundTripFunc func(*http.Request) (*http.Response, error)

func (f jwksRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jwksTestResponse(body []byte, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body))}
}

func TestJWKSBackgroundRefreshStartsAfterFirstLoad(t *testing.T) {
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	synctest.Test(t, func(t *testing.T) {
		var requests atomic.Int32
		transport := jwksRoundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			return jwksTestResponse(body, http.StatusOK), nil
		})
		keys := newExpiringRemoteKeySet(t.Context(), "https://issuer.example.com/keys", &http.Client{Transport: transport, Timeout: 5 * time.Second}, zap.NewNop())
		defer keys.stop()
		time.Sleep(2 * jwksCacheTTL)
		synctest.Wait()
		require.Zero(t, requests.Load(), "unused providers must retain lazy key loading")
		require.NoError(t, keys.refresh(t.Context()))
		time.Sleep(jwksRefreshInterval)
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load())
	})
}

func TestJWKSBackgroundRefreshKeepsCachedRequestsFast(t *testing.T) {
	keyA, err := createPrivateKey()
	require.NoError(t, err)
	keyB, err := createPrivateKey()
	require.NoError(t, err)
	bodyA, bodyB := jwksResponse(t, keyA, "a"), jwksResponse(t, keyB, "b")
	synctest.Test(t, func(t *testing.T) {
		var requests atomic.Int32
		release := make(chan struct{})
		transport := jwksRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if requests.Add(1) == 1 {
				return jwksTestResponse(bodyA, http.StatusOK), nil
			}
			select {
			case <-release:
				return jwksTestResponse(bodyB, http.StatusOK), nil
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		})
		keys := newExpiringRemoteKeySet(t.Context(), "https://issuer.example.com/keys", &http.Client{Transport: transport, Timeout: 5 * time.Second}, zap.NewNop())
		defer keys.stop()
		verifier := oidc.NewVerifier("https://issuer.example.com", keys, &oidc.Config{ClientID: "collector"})
		claims := jwksClaims("https://issuer.example.com")
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		tokenA, tokenB := jwksToken(t, keyA, "a", claims), jwksToken(t, keyB, "b", claims)
		_, err := verifier.Verify(t.Context(), tokenA)
		require.NoError(t, err)
		time.Sleep(jwksRefreshInterval)
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load())
		_, err = verifier.Verify(t.Context(), tokenA)
		require.NoError(t, err, "a blocked background fetch must not delay valid cached requests")
		rotated := make(chan error, 1)
		go func() { _, verifyErr := verifier.Verify(t.Context(), tokenB); rotated <- verifyErr }()
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load(), "the key miss must share the background fetch")
		close(release)
		require.NoError(t, <-rotated)
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load())
		_, err = verifier.Verify(t.Context(), tokenB)
		require.NoError(t, err)
		require.EqualValues(t, 2, requests.Load(), "the rotated key must already be cached")
	})
}

func TestJWKSBackgroundFailurePreservesHardExpiry(t *testing.T) {
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	synctest.Test(t, func(t *testing.T) {
		var failed atomic.Bool
		var requests atomic.Int32
		transport := jwksRoundTripFunc(func(*http.Request) (*http.Response, error) {
			requests.Add(1)
			if failed.Load() {
				return jwksTestResponse(nil, http.StatusServiceUnavailable), nil
			}
			return jwksTestResponse(body, http.StatusOK), nil
		})
		keys := newExpiringRemoteKeySet(t.Context(), "https://issuer.example.com/keys", &http.Client{Transport: transport, Timeout: 5 * time.Second}, zap.NewNop())
		defer keys.stop()
		verifier := oidc.NewVerifier("https://issuer.example.com", keys, &oidc.Config{ClientID: "collector"})
		claims := jwksClaims("https://issuer.example.com")
		claims["exp"] = time.Now().Add(time.Hour).Unix()
		token := jwksToken(t, key, "key", claims)
		_, err := verifier.Verify(t.Context(), token)
		require.NoError(t, err)
		expiresAt := keys.snapshot.Load().expiresAt
		failed.Store(true)
		time.Sleep(jwksRefreshInterval)
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load())
		_, err = verifier.Verify(t.Context(), token)
		require.NoError(t, err, "fresh cached keys remain usable during a failed refresh")
		time.Sleep(jwksCacheTTL - jwksRefreshInterval)
		synctest.Wait()
		require.Equal(t, expiresAt, keys.snapshot.Load().expiresAt, "failed retries must not extend trust")
		_, err = verifier.Verify(t.Context(), token)
		require.Error(t, err, "expired cached keys must not authenticate during an outage")
		failed.Store(false)
		time.Sleep(jwksRetryInterval)
		synctest.Wait()
		before := requests.Load()
		_, err = verifier.Verify(t.Context(), token)
		require.NoError(t, err, "background retry must recover without an authentication request")
		require.Equal(t, before, requests.Load())
	})
}

func TestJWKSBackgroundShutdownCancelsFetch(t *testing.T) {
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	synctest.Test(t, func(t *testing.T) {
		var requests atomic.Int32
		var canceled atomic.Bool
		transport := jwksRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if requests.Add(1) == 1 {
				return jwksTestResponse(body, http.StatusOK), nil
			}
			<-r.Context().Done()
			canceled.Store(true)
			return nil, r.Context().Err()
		})
		keys := newExpiringRemoteKeySet(t.Context(), "https://issuer.example.com/keys", &http.Client{Transport: transport, Timeout: 5 * time.Second}, zap.NewNop())
		defer keys.stop()
		require.NoError(t, keys.refresh(t.Context()))
		time.Sleep(jwksRefreshInterval)
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load())
		keys.stop()
		require.True(t, canceled.Load(), "shutdown must cancel and join an in-flight refresh")
		time.Sleep(2 * jwksCacheTTL)
		synctest.Wait()
		require.EqualValues(t, 2, requests.Load(), "shutdown must stop the refresh timer")
		require.ErrorIs(t, keys.refresh(t.Context()), context.Canceled)
	})
}

func TestJWKSRefreshCoalescesAndHonorsRequestCancellation(t *testing.T) {
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	synctest.Test(t, func(t *testing.T) {
		var requests atomic.Int32
		release := make(chan struct{})
		transport := jwksRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests.Add(1)
			select {
			case <-release:
				return jwksTestResponse(body, http.StatusOK), nil
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
		})
		keys := newExpiringRemoteKeySet(t.Context(), "https://issuer.example.com/keys", &http.Client{Transport: transport, Timeout: 5 * time.Second}, zap.NewNop())
		defer keys.stop()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		canceled := make(chan error, 1)
		go func() { canceled <- keys.refresh(ctx) }()
		pending := make(chan error, 16)
		for range cap(pending) {
			go func() { pending <- keys.refresh(t.Context()) }()
		}
		synctest.Wait()
		require.EqualValues(t, 1, requests.Load())
		cancel()
		require.ErrorIs(t, <-canceled, context.Canceled)
		close(release)
		for range cap(pending) {
			require.NoError(t, <-pending)
		}
		require.EqualValues(t, 1, requests.Load(), "canceling one waiter must not cancel a shared fetch")
	})
}

func TestJWKSStartFailureStopsBackgroundRefreshers(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	cfg := &Config{Providers: []ProviderCfg{
		{IssuerURL: "https://issuer.example.com", Audience: "collector", JWKSURL: server.URL + "/keys"},
		{IssuerURL: server.URL, Audience: "collector"},
	}}
	ext := newExtension(cfg, zap.NewNop()).(*oidcExtension)
	require.Error(t, ext.Start(t.Context(), nil))
	keys := ext.providerContainers["https://issuer.example.com"].remoteKeys
	require.ErrorIs(t, keys.ctx.Err(), context.Canceled)
	require.ErrorIs(t, keys.refresh(t.Context()), context.Canceled)
	require.NoError(t, ext.Shutdown(t.Context()))
}

func TestJWKSRefreshIgnoresUnsupportedKeys(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	good := jwksResponse(t, key, "key")
	var set struct {
		Keys []json.RawMessage `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(good, &set))
	set.Keys = append(set.Keys, json.RawMessage(`{"kty":"unknown"}`), json.RawMessage(`{"alg":"ES256K","kty":"EC"}`))
	body, err := json.Marshal(set)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	verifier := cacheTestVerifier(t, server.URL, time.Now)
	_, err = verifier.Verify(t.Context(), jwksToken(t, key, "key", jwksClaims(server.URL)))
	assert.NoError(t, err)
}
