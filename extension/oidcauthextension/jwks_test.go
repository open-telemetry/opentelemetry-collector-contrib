// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jose-util/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension/extensionauth"
)

func TestJWKSURLConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		url  string
		file string
		err  string
	}{
		{name: "discovery remains the default"},
		{name: "https", url: "https://auth.example.com/keys"},
		{name: "http", url: "http://localhost:8080/keys"},
		{name: "query", url: "https://auth.example.com/keys?tenant=example"},
		{name: "relative URL", url: "/keys", err: "jwks_url"},
		{name: "unsupported scheme", url: "file:///keys", err: "jwks_url"},
		{name: "missing host", url: "https:///keys", err: "jwks_url"},
		{name: "malformed URL", url: "https://%zz/keys", err: "jwks_url"},
		{name: "credentials", url: "https://user:password@auth.example.com/keys", err: "jwks_url"},
		{name: "fragment", url: "https://auth.example.com/keys#fragment", err: "jwks_url"},
		{name: "conflicting file", url: "https://auth.example.com/keys", file: "/not-read.json", err: "mutually exclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Providers: []ProviderCfg{{
				IssuerURL: "https://issuer.example.com", Audience: "collector", JWKSURL: tc.url, PublicKeysFile: tc.file,
			}}}
			if tc.err != "" {
				require.ErrorContains(t, cfg.Validate(), tc.err)
			} else {
				require.NoError(t, cfg.Validate())
			}
		})
	}
	var cfg Config
	cm := confmap.NewFromStringMap(map[string]any{"providers": []any{map[string]any{
		"issuer_url": "https://issuer.example.com", "audience": "collector", "jwks_url": "https://auth.example.com/keys",
	}}})
	require.NoError(t, cm.Unmarshal(&cfg))
	require.NoError(t, cfg.Validate())
	require.Len(t, cfg.Providers, 1)
	assert.Equal(t, "https://auth.example.com/keys", cfg.Providers[0].JWKSURL)
}

func jwksResponse(t *testing.T, key *rsa.PrivateKey, id string) []byte {
	t.Helper()
	body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: id, Algorithm: "RS256", Use: "sig"}}})
	require.NoError(t, err)
	return body
}

func jwksToken(t *testing.T, key *rsa.PrivateKey, id string, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", id))
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	signed, err := signer.Sign(payload)
	require.NoError(t, err)
	token, err := signed.CompactSerialize()
	require.NoError(t, err)
	return token
}

func jwksClaims(issuer string) map[string]any {
	return map[string]any{
		"iss": issuer, "aud": "collector", "exp": time.Now().Add(time.Minute).Unix(),
		"sub": "user", "email": "user@example.com", "groups": []string{"collectors"}, "tenant_id": "example",
	}
}

func startJWKSAuthenticator(t *testing.T, cfg ProviderCfg) extensionauth.Server {
	t.Helper()
	config := &Config{Providers: []ProviderCfg{cfg}}
	require.NoError(t, config.Validate())
	ext := newTestExtension(t, config)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.WithoutCancel(t.Context()))) })
	auth, ok := ext.(extensionauth.Server)
	require.True(t, ok)
	return auth
}

func TestJWKSURLSupportedAlgorithms(t *testing.T) {
	t.Parallel()
	for _, alg := range allSupportedAlgorithms {
		t.Run(string(alg), func(t *testing.T) {
			t.Parallel()
			publicKey, privateKey, err := generator.NewSigningKey(alg, 0)
			require.NoError(t, err)
			issuer := "https://issuer.example.com"
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: privateKey}, (&jose.SignerOptions{}).WithHeader("kid", "key"))
			require.NoError(t, err)
			payload, err := json.Marshal(jwksClaims(issuer))
			require.NoError(t, err)
			signed, err := signer.Sign(payload)
			require.NoError(t, err)
			token, err := signed.CompactSerialize()
			require.NoError(t, err)

			for _, keyAlg := range []string{string(alg), ""} {
				name := "with_alg"
				if keyAlg == "" {
					name = "without_alg"
				}
				t.Run(name, func(t *testing.T) {
					body, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: publicKey, KeyID: "key", Algorithm: keyAlg, Use: "sig"}}})
					require.NoError(t, err)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
					t.Cleanup(server.Close)
					auth := startJWKSAuthenticator(t, ProviderCfg{IssuerURL: issuer, Audience: "collector", JWKSURL: server.URL})
					_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
					require.NoError(t, err)
				})
			}
		})
	}
}

func TestJWKSURLRejectsHMACAlgorithms(t *testing.T) {
	t.Parallel()
	for _, alg := range []jose.SignatureAlgorithm{jose.HS256, jose.HS384, jose.HS512} {
		t.Run(string(alg), func(t *testing.T) {
			t.Parallel()
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(server.Close)
			auth := startJWKSAuthenticator(t, ProviderCfg{IssuerURL: server.URL, Audience: "collector", JWKSURL: server.URL})
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: make([]byte, 64)}, nil)
			require.NoError(t, err)
			payload, err := json.Marshal(jwksClaims(server.URL))
			require.NoError(t, err)
			signed, err := signer.Sign(payload)
			require.NoError(t, err)
			token, err := signed.CompactSerialize()
			require.NoError(t, err)
			_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
			require.ErrorContains(t, err, "unexpected signature algorithm")
			assert.Zero(t, requests.Load(), "unsupported algorithms must be rejected before fetching keys")
		})
	}
}

func TestJWKSURLAuthenticationAndRotation(t *testing.T) {
	t.Parallel()
	keyA, err := createPrivateKey()
	require.NoError(t, err)
	keyB, err := createPrivateKey()
	require.NoError(t, err)
	var current atomic.Value
	current.Store(jwksResponse(t, keyA, "a"))
	var requests, discovery atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keys" {
			discovery.Add(1)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		requests.Add(1)
		_, _ = w.Write(current.Load().([]byte))
	}))
	t.Cleanup(server.Close)
	issuer := server.URL + "/issuer"
	auth := startJWKSAuthenticator(t, ProviderCfg{
		IssuerURL: issuer, Audience: "collector", JWKSURL: server.URL + "/keys", UsernameClaim: "email", GroupsClaim: "groups",
	})
	assert.Zero(t, requests.Load(), "keys are fetched on the first verification")
	tokenA := jwksToken(t, keyA, "a", jwksClaims(issuer))
	ctx, err := auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + tokenA}})
	require.NoError(t, err)
	data := client.FromContext(ctx).Auth
	require.NotNil(t, data)
	assert.Equal(t, tokenA, data.GetAttribute("raw"))
	assert.Equal(t, "example", data.GetAttribute("claims.tenant_id"))
	assert.Equal(t, "user@example.com", data.GetAttribute("subject"))
	assert.Equal(t, []string{"collectors"}, data.GetAttribute("membership"))
	_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + tokenA}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, requests.Load(), "verified keys must be cached")
	current.Store(jwksResponse(t, keyB, "b"))
	tokenB := jwksToken(t, keyB, "b", jwksClaims(issuer))
	_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + tokenB}})
	require.NoError(t, err)
	assert.EqualValues(t, 2, requests.Load(), "an unknown key triggers refresh without restarting")
	_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + tokenA}})
	require.Error(t, err, "the old key is absent after the cache is refreshed")
	assert.Zero(t, discovery.Load(), "direct JWKS must never request discovery")
}

func TestJWKSURLRejectsInvalidTokens(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	otherKey, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(server.Close)
	issuer := "https://issuer.example.com"
	auth := startJWKSAuthenticator(t, ProviderCfg{IssuerURL: issuer, Audience: "collector", JWKSURL: server.URL})
	for _, name := range []string{"issuer", "audience", "expiry", "signature", "malformed", "algorithm"} {
		t.Run(name, func(t *testing.T) {
			claims := jwksClaims(issuer)
			signingKey := key
			switch name {
			case "issuer":
				claims["iss"] = "https://other.example.com"
			case "audience":
				claims["aud"] = "other"
			case "expiry":
				claims["exp"] = time.Now().Add(-time.Minute).Unix()
			case "signature":
				signingKey = otherKey
			}
			token := jwksToken(t, signingKey, "key", claims)
			if name == "malformed" {
				token = "invalid"
			}
			if name == "algorithm" {
				signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: make([]byte, 32)}, nil)
				require.NoError(t, err)
				payload, err := json.Marshal(claims)
				require.NoError(t, err)
				signed, err := signer.Sign(payload)
				require.NoError(t, err)
				token, err = signed.CompactSerialize()
				require.NoError(t, err)
			}
			ctx, err := auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
			require.Error(t, err)
			assert.Nil(t, client.FromContext(ctx).Auth)
		})
	}
}

func TestJWKSURLExplicitIgnoreSettings(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(server.Close)
	auth := startJWKSAuthenticator(t, ProviderCfg{
		IssuerURL: "https://configured.example.com", IgnoreIssuer: true, IgnoreAudience: true, JWKSURL: server.URL,
	})
	token := jwksToken(t, key, "key", jwksClaims("https://other.example.com"))
	_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
	require.NoError(t, err)
}

func TestJWKSURLWithDiscoveryProvider(t *testing.T) {
	t.Parallel()
	discovery, err := newOIDCServer()
	require.NoError(t, err)
	discovery.Start()
	t.Cleanup(discovery.Close)
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "direct")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(server.Close)
	cfg := &Config{
		IssuerURL: discovery.URL, Audience: "collector",
		Providers: []ProviderCfg{{IssuerURL: server.URL, Audience: "collector", JWKSURL: server.URL}},
	}
	require.NoError(t, cfg.Validate())
	ext := newTestExtension(t, cfg)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(context.WithoutCancel(t.Context()))) })
	auth, ok := ext.(extensionauth.Server)
	require.True(t, ok)
	payload, err := json.Marshal(jwksClaims(discovery.URL))
	require.NoError(t, err)
	discoveredToken, err := discovery.token(payload)
	require.NoError(t, err)
	directToken := jwksToken(t, key, "direct", jwksClaims(server.URL))
	for _, token := range []string{discoveredToken, directToken} {
		_, err := auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
		require.NoError(t, err)
	}
}

func TestJWKSURLRejectsEmptyKeySet(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	t.Cleanup(server.Close)
	key, err := createPrivateKey()
	require.NoError(t, err)
	auth := startJWKSAuthenticator(t, ProviderCfg{IssuerURL: server.URL, Audience: "collector", JWKSURL: server.URL})
	token := jwksToken(t, key, "key", jwksClaims(server.URL))
	_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
	require.Error(t, err)
}

func TestJWKSURLFailuresPreserveCachedKeys(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	good := jwksResponse(t, key, "key")
	type response struct {
		status int
		body   []byte
	}
	for _, tc := range []struct {
		name string
		response
	}{
		{"unavailable", response{http.StatusServiceUnavailable, nil}},
		{"malformed", response{http.StatusOK, []byte("not JSON")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var current atomic.Value
			current.Store(tc.response)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				r := current.Load().(response)
				w.WriteHeader(r.status)
				_, _ = w.Write(r.body)
			}))
			t.Cleanup(server.Close)
			auth := startJWKSAuthenticator(t, ProviderCfg{IssuerURL: server.URL, Audience: "collector", JWKSURL: server.URL})
			token := jwksToken(t, key, "key", jwksClaims(server.URL))
			_, err := auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
			require.Error(t, err, "first fetch failure must reject the request")
			current.Store(response{http.StatusOK, good})
			_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
			require.NoError(t, err, "a later request can recover")
			current.Store(tc.response)
			other, err := createPrivateKey()
			require.NoError(t, err)
			unknown := jwksToken(t, other, "other", jwksClaims(server.URL))
			_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + unknown}})
			require.Error(t, err, "unavailable new keys must reject the request")
			_, err = auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
			require.NoError(t, err, "cached keys remain usable")
		})
	}
}

func TestJWKSURLUsesIssuerCA(t *testing.T) {
	t.Parallel()
	key, err := createPrivateKey()
	require.NoError(t, err)
	body := jwksResponse(t, key, "key")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(server.Close)
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o600))
	for _, trusted := range []bool{false, true} {
		name := "untrusted"
		cfg := ProviderCfg{IssuerURL: "https://issuer.example.com", Audience: "collector", JWKSURL: server.URL}
		if trusted {
			name = "trusted"
			cfg.IssuerCAPath = caPath
		}
		t.Run(name, func(t *testing.T) {
			auth := startJWKSAuthenticator(t, cfg)
			token := jwksToken(t, key, "key", jwksClaims(cfg.IssuerURL))
			_, err := auth.Authenticate(t.Context(), map[string][]string{"authorization": {"Bearer " + token}})
			if trusted {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
