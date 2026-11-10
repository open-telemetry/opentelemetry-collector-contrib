// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// tokenRequest is what the fake token endpoint records for each request.
type tokenRequest struct {
	form       url.Values
	authHeader string
}

// newFakeTokenServer returns a token endpoint that records every request and
// issues tokens that expire after one second.
func newFakeTokenServer(t *testing.T) (*httptest.Server, func() []tokenRequest) {
	t.Helper()
	var (
		mu   sync.Mutex
		reqs []tokenRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.NoError(t, r.ParseForm())
		mu.Lock()
		reqs = append(reqs, tokenRequest{form: r.PostForm, authHeader: r.Header.Get("Authorization")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   1,
		})
	}))
	t.Cleanup(srv.Close)
	return srv, func() []tokenRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]tokenRequest(nil), reqs...)
	}
}

func publicKeyFromPEMFile(t *testing.T, path string) *rsa.PublicKey {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	pk, err := jwt.ParseRSAPrivateKeyFromPEM(b)
	require.NoError(t, err)
	return &pk.PublicKey
}

func parseAssertion(t *testing.T, assertion string, pub *rsa.PublicKey, aud string) (*jwt.Token, jwt.MapClaims) {
	t.Helper()
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(assertion, claims, func(*jwt.Token) (any, error) {
		return pub, nil
	}, jwt.WithValidMethods([]string{"RS384"}), jwt.WithAudience(aud), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	require.NoError(t, err)
	return tok, claims
}

func TestPrivateKeyJWTClientAuth(t *testing.T) {
	srv, requests := newFakeTokenServer(t)
	const keyFile = "testdata/client.key"

	auth, err := newClientAuthenticator(&Config{
		ClientID:                 "my-client",
		TokenEndpointAuthMethod:  authMethodPrivateKeyJWT,
		ClientCertificateKeyFile: keyFile,
		ClientCertificateKeyID:   "key-1",
		SignatureAlgorithm:       "RS384",
		Audience:                 "https://issuer.example.com/oauth2/token",
		Claims:                   map[string]any{"extra": "value"},
		TokenURL:                 srv.URL,
		Scopes:                   []string{"api.metrics", "api.traces"},
		EndpointParams:           url.Values{"audience": []string{"backend"}},
		// Larger than the 1s token lifetime, so every call fetches a new token.
		ExpiryBuffer: 10 * time.Second,
	}, zap.NewNop())
	require.NoError(t, err)

	for range 2 {
		tok, err := auth.Token(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "access-token", tok.AccessToken)
	}

	reqs := requests()
	require.Len(t, reqs, 2)
	pub := publicKeyFromPEMFile(t, keyFile)

	jtis := map[any]struct{}{}
	for _, req := range reqs {
		assert.Empty(t, req.authHeader, "no Authorization header must be sent")
		assert.Equal(t, []string{"client_credentials"}, req.form["grant_type"])
		assert.Equal(t, []string{"my-client"}, req.form["client_id"])
		assert.Equal(t, []string{clientAssertionTypeJWTBearer}, req.form["client_assertion_type"])
		assert.Equal(t, []string{"api.metrics api.traces"}, req.form["scope"])
		assert.Equal(t, []string{"backend"}, req.form["audience"])
		assert.NotContains(t, req.form, "client_secret")
		assert.NotContains(t, req.form, "assertion")
		require.Len(t, req.form["client_assertion"], 1)

		tok, claims := parseAssertion(t, req.form.Get("client_assertion"), pub, "https://issuer.example.com/oauth2/token")
		assert.Equal(t, "key-1", tok.Header["kid"])
		assert.Equal(t, "my-client", claims["iss"])
		assert.Equal(t, "my-client", claims["sub"])
		assert.Equal(t, "value", claims["extra"])
		assert.NotContains(t, claims, "scope")
		exp, err := claims.GetExpirationTime()
		require.NoError(t, err)
		iat, err := claims.GetIssuedAt()
		require.NoError(t, err)
		assert.Equal(t, clientAssertionLifetime, exp.Sub(iat.Time))
		jtis[claims["jti"]] = struct{}{}
	}
	assert.Len(t, jtis, 2, "each token request must use a fresh assertion")
}

func TestPrivateKeyJWTAudienceDefaultsToTokenURL(t *testing.T) {
	srv, requests := newFakeTokenServer(t)

	auth, err := newClientAuthenticator(&Config{
		ClientIDFile:             "testdata/test-cred.txt",
		TokenEndpointAuthMethod:  authMethodPrivateKeyJWT,
		ClientCertificateKeyFile: "testdata/client.key",
		SignatureAlgorithm:       "RS384",
		TokenURL:                 srv.URL,
	}, zap.NewNop())
	require.NoError(t, err)

	_, err = auth.Token(t.Context())
	require.NoError(t, err)

	reqs := requests()
	require.Len(t, reqs, 1)
	assert.NotContains(t, reqs[0].form, "scope")
	tok, claims := parseAssertion(t, reqs[0].form.Get("client_assertion"), publicKeyFromPEMFile(t, "testdata/client.key"), srv.URL)
	assert.NotContains(t, tok.Header, "kid")

	clientID, err := readCredentialsFile("testdata/test-cred.txt")
	require.NoError(t, err)
	assert.Equal(t, clientID, claims["iss"])
	assert.Equal(t, clientID, claims["sub"])
	assert.Equal(t, []string{clientID}, reqs[0].form["client_id"])
}

func writeRSAKey(t *testing.T, path string) *rsa.PublicKey {
	t.Helper()
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(pk)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
	return &pk.PublicKey
}

func TestPrivateKeyJWTRereadsRotatedKeyFile(t *testing.T) {
	srv, requests := newFakeTokenServer(t)
	keyFile := filepath.Join(t.TempDir(), "client.key")
	oldPub := writeRSAKey(t, keyFile)

	auth, err := newClientAuthenticator(&Config{
		ClientID:                 "my-client",
		TokenEndpointAuthMethod:  authMethodPrivateKeyJWT,
		ClientCertificateKeyFile: keyFile,
		SignatureAlgorithm:       "RS384",
		TokenURL:                 srv.URL,
		ExpiryBuffer:             10 * time.Second,
	}, zap.NewNop())
	require.NoError(t, err)

	_, err = auth.Token(t.Context())
	require.NoError(t, err)
	newPub := writeRSAKey(t, keyFile)
	_, err = auth.Token(t.Context())
	require.NoError(t, err)

	reqs := requests()
	require.Len(t, reqs, 2)
	parseAssertion(t, reqs[0].form.Get("client_assertion"), oldPub, srv.URL)
	parseAssertion(t, reqs[1].form.Get("client_assertion"), newPub, srv.URL)
}

func TestPrivateKeyJWTMissingKeyFile(t *testing.T) {
	_, err := newClientAuthenticator(&Config{
		ClientID:                 "my-client",
		TokenEndpointAuthMethod:  authMethodPrivateKeyJWT,
		ClientCertificateKeyFile: "testdata/does-not-exist.key",
		TokenURL:                 "https://example.com/token",
	}, zap.NewNop())
	assert.ErrorIs(t, err, errNoClientCertificateProvided)
}

func TestClientSecretAuthMethods(t *testing.T) {
	tests := []struct {
		method     string
		wantHeader bool
	}{
		{method: authMethodClientSecretBasic, wantHeader: true},
		{method: authMethodClientSecretPost, wantHeader: false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			srv, requests := newFakeTokenServer(t)
			auth, err := newClientAuthenticator(&Config{
				ClientID:                "my-client",
				ClientSecret:            "my-secret",
				TokenEndpointAuthMethod: tt.method,
				TokenURL:                srv.URL,
			}, zap.NewNop())
			require.NoError(t, err)

			_, err = auth.Token(t.Context())
			require.NoError(t, err)

			reqs := requests()
			require.Len(t, reqs, 1)
			if tt.wantHeader {
				assert.NotEmpty(t, reqs[0].authHeader)
				assert.NotContains(t, reqs[0].form, "client_secret")
			} else {
				assert.Empty(t, reqs[0].authHeader)
				assert.Equal(t, []string{"my-client"}, reqs[0].form["client_id"])
				assert.Equal(t, []string{"my-secret"}, reqs[0].form["client_secret"])
			}
		})
	}
}
