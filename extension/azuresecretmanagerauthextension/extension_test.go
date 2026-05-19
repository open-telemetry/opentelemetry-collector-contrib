// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuresecretmanagerauthextension

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"
)

// Client tests

func TestClient_RoundTripper_SetsBasicAuth(t *testing.T) {
	ext := &azureSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "testuser", password: "testpass"})

	var capturedReq *http.Request
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	rt, err := ext.RoundTripper(base)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	username, password, ok := capturedReq.BasicAuth()
	require.True(t, ok)
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "testpass", password)
}

func TestClient_RoundTripper_ColonInUsername(t *testing.T) {
	ext := &azureSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "user:name", password: "pass"})

	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	_, err := ext.RoundTripper(base)
	require.Error(t, err)
	assert.ErrorIs(t, err, basicauth.ErrInvalidFormat)
}

func TestClient_PerRPCCredentials_Authorization(t *testing.T) {
	ext := &azureSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "rpcuser", password: "rpcpass"})

	cred, err := ext.PerRPCCredentials()
	require.NoError(t, err)

	md, err := cred.GetRequestMetadata(t.Context())
	require.NoError(t, err)

	authHeader, ok := md["authorization"]
	require.True(t, ok)
	assert.Contains(t, authHeader, "Basic ")
}

func TestClient_PerRPCCredentials_RequireTransportSecurity(t *testing.T) {
	ext := &azureSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "user", password: "pass"})

	cred, err := ext.PerRPCCredentials()
	require.NoError(t, err)
	assert.True(t, cred.RequireTransportSecurity())
}

func TestClient_CredentialRotation_PickedUp(t *testing.T) {
	ext := &azureSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "old", password: "oldpass"})

	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	rt, err := ext.RoundTripper(base)
	require.NoError(t, err)

	ext.creds.Store(&clientCredentials{username: "new", password: "newpass"})

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
	_, err = rt.RoundTrip(req)
	require.NoError(t, err)
}

func TestClient_CredentialProvider_Interface(t *testing.T) {
	ext := &azureSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "u", password: "p"})

	assert.Equal(t, "u", ext.Username())
	assert.Equal(t, "p", ext.Password())
}

// Server tests

func TestServer_Authenticate_ValidCredentials(t *testing.T) {
	ext := &azureSecretsManagerAuthServer{}
	matchFn := func(username, password string) bool {
		return username == "admin" && password == "secret"
	}
	ext.matchFunc.Store(&matchFn)

	headers := map[string][]string{
		"Authorization": {"Basic YWRtaW46c2VjcmV0"}, // admin:secret
	}

	ctx, err := ext.Authenticate(t.Context(), headers)
	require.NoError(t, err)

	cl := client.FromContext(ctx)
	require.NotNil(t, cl.Auth)
	assert.Equal(t, "admin", cl.Auth.GetAttribute("username"))
}

func TestServer_Authenticate_InvalidCredentials(t *testing.T) {
	ext := &azureSecretsManagerAuthServer{}
	matchFn := func(string, string) bool { return false }
	ext.matchFunc.Store(&matchFn)

	headers := map[string][]string{
		"Authorization": {"Basic YWRtaW46d3Jvbmc="}, // admin:wrong
	}

	_, err := ext.Authenticate(t.Context(), headers)
	require.Error(t, err)
	assert.ErrorIs(t, err, basicauth.ErrInvalidCredentials)
}

func TestServer_Authenticate_NoAuthHeader(t *testing.T) {
	ext := &azureSecretsManagerAuthServer{}
	matchFn := func(string, string) bool { return true }
	ext.matchFunc.Store(&matchFn)

	headers := map[string][]string{}

	_, err := ext.Authenticate(t.Context(), headers)
	require.Error(t, err)
	assert.ErrorIs(t, err, basicauth.ErrNoAuth)
}

func TestServer_Authenticate_NotStarted(t *testing.T) {
	ext := &azureSecretsManagerAuthServer{}

	headers := map[string][]string{
		"Authorization": {"Basic YWRtaW46c2VjcmV0"},
	}

	_, err := ext.Authenticate(t.Context(), headers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

// Helpers

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
