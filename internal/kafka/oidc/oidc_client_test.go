// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidc // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const (
	testClientID = "mock-client-id"
	testScope    = "mock-scope"
)

var tokenExpireSecs = 10

type MockOAuthProvider struct {
	MockSignin func(clientID, scope string, requestedAuthority ...string) (*sarama.AccessToken, error)
}

func (m *MockOAuthProvider) Signin(clientID, scope string, requestedAuthority ...string) (*sarama.AccessToken, error) {
	if m.MockSignin != nil {
		return m.MockSignin(clientID, scope, requestedAuthority...)
	}
	return nil, errors.New("MockSignin function not defined")
}

func (*MockOAuthProvider) Name() string {
	return "mock"
}

func TestOIDCProvider_GetToken_Success(t *testing.T) {
	secretFile, err := k8sSecretFile()
	assert.NoError(t, err)

	testClientSecret, err = os.ReadFile(secretFile)
	assert.NoError(t, err)

	oidcServerQuit := make(chan bool, 1)
	portCh := make(chan int, 1)
	go func() {
		oidcServer(oidcServerQuit, portCh)
	}()
	defer func() {
		oidcServerQuit <- true
	}()

	// Wait for server to start and get the port
	port := <-portCh
	tokenURL := fmt.Sprintf("http://127.0.0.1:%d/token", port)

	oidcProvider := NewOIDCfileTokenProvider(context.Background(), testClientID, secretFile, tokenURL, []string{testScope}, 0)

	saramaToken, err := oidcProvider.Token()
	require.NoError(t, err)
	assert.NotNil(t, saramaToken)
	assert.NotEmpty(t, saramaToken.Token)

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	tokenObj, err := parser.Parse(saramaToken.Token, func(_ *jwt.Token) (any, error) {
		return publicKey, nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, tokenObj)
	claims := tokenObj.Claims.(jwt.MapClaims)
	assert.Equal(t, testClientID, claims["client_id"])
	assert.Equal(t, testScope, claims["scope"])

	assert.WithinDuration(t, time.Now(), time.Unix(int64(claims["iat"].(float64)), 0), 2*time.Second)
	expectedTimeout := time.Now().Add(time.Duration(tokenExpireSecs) * time.Second)
	actualTimeout := time.Unix(int64(claims["exp"].(float64)), 0)
	assert.WithinDuration(t, expectedTimeout, actualTimeout, 2*time.Second)
}

func TestOIDCProvider_GetToken_Error(t *testing.T) {
	secretFile, err := k8sSecretFile()
	assert.NoError(t, err)

	testClientSecret, err = os.ReadFile(secretFile)
	assert.NoError(t, err)

	oidcServerQuit := make(chan bool)
	portCh := make(chan int, 1)
	go func() {
		oidcServer(oidcServerQuit, portCh)
	}()
	defer func() {
		oidcServerQuit <- true
	}()

	// Wait for server to start and get the port
	port := <-portCh
	tokenURL := fmt.Sprintf("http://127.0.0.1:%d/token", port)

	oidcProvider := NewOIDCfileTokenProvider(context.Background(), "wrong-client-id", secretFile,
		tokenURL, []string{testScope}, 0)

	saramaToken, err := oidcProvider.Token()
	require.Error(t, err)
	assert.Nil(t, saramaToken)
}

func TestOIDCProvider_TokenCaching(t *testing.T) {
	secretFile, err := k8sSecretFile()
	assert.NoError(t, err)

	testClientSecret, err = os.ReadFile(secretFile)
	assert.NoError(t, err)

	oidcServerQuit := make(chan bool, 1)
	portCh := make(chan int, 1)
	go func() {
		oidcServer(oidcServerQuit, portCh)
	}()
	defer func() {
		oidcServerQuit <- true
	}()

	// Wait for server to start and get the port
	port := <-portCh
	tokenURL := fmt.Sprintf("http://127.0.0.1:%d/token", port)

	oidcProvider := NewOIDCfileTokenProvider(context.Background(), testClientID, secretFile, tokenURL, []string{testScope}, 0)

	token1, err1 := oidcProvider.Token()
	assert.NoError(t, err1)
	assert.NotNil(t, token1)

	token2, err2 := oidcProvider.Token()
	assert.NoError(t, err2)
	assert.NotNil(t, token2)
	assert.Equal(t, token1, token2)
}

func TestOIDCProvider_TokenExpired(t *testing.T) {
	secretFile, err := k8sSecretFile()
	assert.NoError(t, err)

	testClientSecret, err = os.ReadFile(secretFile)
	assert.NoError(t, err)

	oidcServerQuit := make(chan bool, 1)
	portCh := make(chan int, 1)
	go func() {
		oidcServer(oidcServerQuit, portCh)
	}()
	defer func() {
		oidcServerQuit <- true
	}()

	// Wait for server to start and get the port
	port := <-portCh
	tokenURL := fmt.Sprintf("http://127.0.0.1:%d/token", port)

	originalTokenExpireSecs := tokenExpireSecs
	defer func() {
		tokenExpireSecs = originalTokenExpireSecs
	}()
	tokenExpireSecs = 3
	oidcProvider := NewOIDCfileTokenProvider(context.Background(), testClientID, secretFile, tokenURL, []string{testScope}, 0)

	token1, err1 := oidcProvider.Token()
	assert.NoError(t, err1)
	assert.NotNil(t, token1)

	time.Sleep(3500 * time.Millisecond)

	token2, err2 := oidcProvider.Token()
	assert.NoError(t, err2)
	assert.NotNil(t, token2)
	assert.NotEqual(t, token1, token2)
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

// An implementation of a very basic OIDC server that supports only
// the "client_credentials" grant type.
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scope        string `json:"scope"`
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

var (
	privateKey       *rsa.PrivateKey
	publicKey        *rsa.PublicKey
	testClientSecret []byte
)

func init() {
	var err error
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal("Failed to generate RSA key:", err)
	}
	publicKey = &privateKey.PublicKey
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		err := json.NewEncoder(w).Encode(ErrorResponse{
			Error:            "invalid_request",
			ErrorDescription: "Method not allowed",
		})
		if err != nil {
			log.Printf("could not encode error response: %v", err)
		}
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(ErrorResponse{
			Error:            "invalid_request",
			ErrorDescription: "Failed to parse form data",
		})
		if err != nil {
			log.Printf("could not encode error response: %v", err)
		}
		return
	}

	grantType := r.FormValue("grant_type")
	submittedClientID := r.FormValue("client_id")
	submittedClientSecret := r.FormValue("client_secret")
	scope := r.FormValue("scope")

	if grantType != "client_credentials" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(ErrorResponse{
			Error:            "unsupported_grant_type",
			ErrorDescription: "Only client_credentials grant type is supported",
		})
		if err != nil {
			log.Printf("could not encode error response: %v", err)
		}
		return
	}

	if submittedClientID == "" || submittedClientSecret == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(ErrorResponse{
			Error:            "invalid_client",
			ErrorDescription: "Client ID and secret are required",
		})
		if err != nil {
			log.Printf("could not encode error response: %v", err)
		}
		return
	}

	if !validateClient(submittedClientID, submittedClientSecret) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		err := json.NewEncoder(w).Encode(ErrorResponse{
			Error:            "invalid_client",
			ErrorDescription: "Invalid client credentials",
		})
		if err != nil {
			log.Printf("could not encode error response: %v", err)
		}
		return
	}

	token, err := generateJWTToken(submittedClientID, scope)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		encodeErr := json.NewEncoder(w).Encode(ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Failed to generate token",
		})
		if encodeErr != nil {
			log.Printf("could not encode error response: %v", encodeErr)
		}
		return
	}

	// log.Printf("tokenHandler(): tokenExpireSecs = %v", tokenExpireSecs)

	response := oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
		// Note that `expiry` is not an official field in the OIDC or OAuth2 specs
		Expiry:    time.Now().Add(time.Duration(tokenExpireSecs) * time.Second),
		ExpiresIn: int64(tokenExpireSecs),
	}

	// log.Printf("SERVER tokenHandler(): response token = %s\n", DumpOauth2Token(&response))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("could not encode response: %v", err)
	}
}

func validateClient(clientID, clientSecret string) bool {
	validClients := map[string]string{
		testClientID: string(testClientSecret),
		// "test_client": "test_secret",
	}

	expectedSecret, exists := validClients[clientID]
	return exists && expectedSecret == clientSecret
}

func generateJWTToken(clientID, scope string) (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"iss":       "oidc-mock-server",
		"sub":       clientID,
		"aud":       "api",
		"exp":       now.Add(time.Duration(tokenExpireSecs) * time.Second).Unix(),
		"iat":       now.Unix(),
		"client_id": clientID,
	}

	if scope != "" {
		claims["scope"] = scope
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	return token.SignedString(privateKey)
}

func oidcServer(shutdownCh <-chan bool, portCh chan<- int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", tokenHandler)
	s := &http.Server{
		Addr:              ":0", // Use port 0 for dynamic allocation
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
	}

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		log.Fatalf("could not create listener: %v", err)
	}

	// Get the actual port number and send it back
	port := listener.Addr().(*net.TCPAddr).Port
	portCh <- port

	go func() {
		err = s.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			log.Printf("OIDC server error: %v", err)
		}
	}()

	<-shutdownCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = s.Shutdown(shutdownCtx)
	if err != nil {
		log.Printf("error shutting down OIDC server: %v", err)
	}
}
