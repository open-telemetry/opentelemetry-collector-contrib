// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidc // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/oauth2-proxy/mockoidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockOAuthProvider struct {
	MockSignin func(clientID, scope string, requestedAuthority ...string) (*sarama.AccessToken, error)
}

func (m *MockOAuthProvider) Signin(clientID, scope string, requestedAuthority ...string) (*sarama.AccessToken, error) {
	if m.MockSignin != nil {
		return m.MockSignin(clientID, scope, requestedAuthority...)
	}
	return nil, errors.New("MockSignin function not defined")
}

func (m *MockOAuthProvider) Name() string {
	return "mock"
}

func TestOIDCProvider_GetToken_Success(t *testing.T) {
	clientID := "mock-client-id"
	secretFile, err := k8sSecretFile()
	assert.NoError(t, err)

	clientSecret, err := os.ReadFile(secretFile)
	assert.NoError(t, err)

	oidcServerQuit := make(chan any)
	go func() {
		oidcServer(oidcServerQuit, clientID, string(clientSecret), 10, 10)
	}()
	defer func() {
		oidcServerQuit <- true
	}()

	time.Sleep(3 * time.Second) // wait for OIDC to fully start
	tokenURL := "http://127.0.0.1:3000/oidc"

	oidcProvider := NewOIDCfileTokenProvider(context.Background(), clientID, secretFile, tokenURL, []string{"mock-scope"}, 0)

	token, err := oidcProvider.Token()
	require.NoError(t, err)
	assert.NotNil(t, token)
	assert.NotEmpty(t, token.Token)
}

// func TestOIDCProvider_GetToken_Error(t *testing.T) {
// 	mockProvider := &MockOAuthProvider{
// 		// TODO can we remove all requestAuthority ... ?
// 		MockSignin: func(clientID, scope string, requestedAuthority ...string) (*sarama.AccessToken, error) {
// 			return nil, errors.New("failed to sign in")
// 		},
// 	}

// 	oidcProvider := NewOIDCFileTokenProvider("mock-client-id", "mock-scopes", mockProvider, 0)

// 	token, err := oidcProvider.Token()
// 	assert.NotNil(t, err)
// 	assert.Nil(t, token)
// 	assert.Equal(t, "failed to refresh token: failed to sign in", err.Error())
// }

// func TestOIDCProvider_TokenCaching(t *testing.T) {
// 	mockProvider := &MockOAuthProvider{
// 		MockSignin: func(clientID, scope string, requestedAuthority ...string) (*AccessToken, error) {
// 			return &AccessToken{
// 				AccessToken: "mock-access-token",
// 				ExpiresIn:   3600,
// 				TokenType:   "Bearer",
// 				Scope:       "mock-scopes",
// 			}, nil
// 		},
// 	}

// 	oidcProvider := NewGRSigningAccessTokenProvider("mock-client-id", "mock-scopes", mockProvider, 0)

// 	token1, err1 := oidcProvider.GetToken()
// 	assert.Nil(t, err1)
// 	assert.NotNil(t, token1)

// 	token2, err2 := oidcProvider.GetToken()
// 	assert.Nil(t, err2)
// 	assert.NotNil(t, token2)
// 	assert.Equal(t, token1, token2)
// }

// func TestOIDCProvider_TokenExpired(t *testing.T) {
// 	mockProvider := &MockOAuthProvider{
// 		MockSignin: func(clientID, scope string, requestedAuthority ...string) (*AccessToken, error) {
// 			return &AccessToken{
// 				AccessToken: "mock-access-token",
// 				ExpiresIn:   1,
// 				TokenType:   "Bearer",
// 				Scope:       "mock-scopes",
// 			}, nil
// 		},
// 	}

// 	oidcProvider := NewGRSigningAccessTokenProvider("mock-client-id", "mock-scopes", mockProvider, 0)
// 	oidcProvider.refreshCooldown = 500 * time.Millisecond

// 	token1, err1 := oidcProvider.GetToken()
// 	assert.Nil(t, err1)
// 	assert.NotNil(t, token1)

// 	time.Sleep(2 * time.Second)

// 	token2, err2 := oidcProvider.GetToken()
// 	assert.Nil(t, err2)
// 	assert.NotNil(t, token2)
// 	assert.NotEqual(t, token1, token2)
// }

func oidcServer(ch <-chan any, clientID, clientSecret string, accessTTLsecs, refreshTTLsecs int) {
	// Create RSA Private Key for token signing
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	m, _ := mockoidc.NewServer(rsaKey)
	m.ClientID = clientID
	m.ClientSecret = clientSecret
	m.AccessTTL = time.Duration(accessTTLsecs) * time.Second
	m.RefreshTTL = time.Duration(refreshTTLsecs) * time.Second

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			fmt.Fprintf(os.Stderr, "\n-------- oidcServer: before next.ServerHTTP(): request URL is %v\n", req.URL)
			// custom middleware logic here ...
			next.ServeHTTP(rw, req)
			fmt.Fprintf(os.Stderr, "\n-------- oidcServer: after next.ServeHTTP(): request URL is %v\n", req.URL)
			// custom middleware logic here...
		})
	}
	m.AddMiddleware(middleware)

	ln, _ := net.Listen("tcp", "127.0.0.1:3000")

	err := m.Start(ln, nil) // specify nil for tlsConfig to use HTTP
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not start server: %v\n", err)
		os.Exit(1)
	}
	defer m.Shutdown()

	// cfg := m.Config()
	// fmt.Printf("%s\n", litter.Sdump(cfg))

	<-ch
	fmt.Fprintf(os.Stderr, "oidcServer shutting down\n")
}

func k8sSecretFile() (string, error) {
	k8sSAtoken := `eyJhbGciOiJSUzI1NiIsImtpZCI6IjdjdDhhT0pTSXh0Zm0yUVprUVRXaFpTVFpHUlQ0MlFkbDMzQXQ1XzRURkkifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiXSwiZXhwIjoxNzg0Mzk4Mzk5LCJpYXQiOjE3NTI4NjIzOTksImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiNTE3Zjg4ZDUtOTNjZC00YWIzLWFkZWItY2NiZjExMDdmZGYxIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJ0ZXN0Iiwibm9kZSI6eyJuYW1lIjoia2luZC1jb250cm9sLXBsYW5lIiwidWlkIjoiZThjZTczMWMtMmZjNC00NWZjLWJjNzItMzdhYTgyNDQzN2EwIn0sInBvZCI6eyJuYW1lIjoib2lkYy1zZXJ2ZXIteDU4bm4iLCJ1aWQiOiJmM2Q3YTBkZC04Yzk3LTRkNTgtOGYyYy01ZDRiNThjMmY5NDIifSwic2VydmljZWFjY291bnQiOnsibmFtZSI6ImRlZmF1bHQiLCJ1aWQiOiIwZjc1MTJhNi00ZjhkLTQxYjAtOTM4NC1mYWE4YzlmZWUxMWYifSwid2FybmFmdGVyIjoxNzUyODY2MDA2fSwibmJmIjoxNzUyODYyMzk5LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6dGVzdDpkZWZhdWx0In0.A8eDX9Wz6aoAwO-Vrg2ddbxJ5d7r3pdg8J6D4gyHPNQLRmBcZHaWagRKJTZ3gDYvT_u_hCG5RJrHARt9MncftPJ5_gdRyXckbd9a9dcSSRVxFEPzdaUR6GSmTmI2sUwhU33AnWmRqlOlZW_WtslPGXl8tNsfDLfpvabjAuBFJrb7KB8MvzXVNvVcJ8BmM4oglX3e3xIxLBzSSQFkW9OGdmeWFsMh-lNaHpzXQGaZx3W2Wit2SUigbDDSJPCTs_tFMdPv-LW0AH9eRd5yU_j87gEsapu_u5j6qcNku-3g79LcGoIvTqe8QdSI7OeoWVnD05SjfAoyHhR-aoMJtSCOQg`

	tokenPath := filepath.Join(os.TempDir(), "k8sToken")
	err := os.WriteFile(tokenPath, []byte(k8sSAtoken), 0o600)
	if err != nil {
		return "", fmt.Errorf("error writing %s: %w", tokenPath, err)
	}

	return tokenPath, nil
}
