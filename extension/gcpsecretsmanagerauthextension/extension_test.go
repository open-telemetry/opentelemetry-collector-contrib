// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpsecretsmanagerauthextension

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth"
)

// Client tests

func TestRoundTripper_SetsBasicAuth(t *testing.T) {
	ext := &gcpSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "alice", password: "secret123"})

	rt, err := ext.RoundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "alice", user)
		assert.Equal(t, "secret123", pass)
		return &http.Response{StatusCode: http.StatusOK}, nil
	}))
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRoundTripper_ColonInUsername(t *testing.T) {
	ext := &gcpSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "user:name", password: "pass"})

	_, err := ext.RoundTripper(http.DefaultTransport)
	assert.Error(t, err)
}

func TestPerRPCCredentials_Authorization(t *testing.T) {
	ext := &gcpSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "bob", password: "hunter2"})

	rpc, err := ext.PerRPCCredentials()
	require.NoError(t, err)

	md, err := rpc.GetRequestMetadata(context.Background())
	require.NoError(t, err)

	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("bob:hunter2"))
	assert.Equal(t, expected, md["authorization"])
}

func TestPerRPCCredentials_RequireTransportSecurity(t *testing.T) {
	ext := &gcpSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "bob", password: "pass"})

	rpc, err := ext.PerRPCCredentials()
	require.NoError(t, err)
	assert.True(t, rpc.RequireTransportSecurity())
}

func TestCredentialRotation_PickedUp(t *testing.T) {
	ext := &gcpSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "old", password: "oldpass"})

	rt, err := ext.RoundTripper(roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "new", user)
		assert.Equal(t, "newpass", pass)
		return &http.Response{StatusCode: http.StatusOK}, nil
	}))
	require.NoError(t, err)

	ext.creds.Store(&clientCredentials{username: "new", password: "newpass"})

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err = rt.RoundTrip(req)
	require.NoError(t, err)
}

func TestCredentialProvider_Interface(t *testing.T) {
	ext := &gcpSecretsManagerAuthClient{}
	ext.creds.Store(&clientCredentials{username: "u", password: "p"})

	var _ basicauth.CredentialProvider = ext
	assert.Equal(t, "u", ext.Username())
	assert.Equal(t, "p", ext.Password())
}

// Server tests

func TestAuthenticate_ValidCredentials(t *testing.T) {
	ext := &gcpSecretsManagerAuthServer{}
	matchFn := func(user, pass string) bool {
		return user == "admin" && pass == "correct"
	}
	ext.matchFunc.Store(&matchFn)

	encoded := base64.StdEncoding.EncodeToString([]byte("admin:correct"))
	headers := map[string][]string{
		"authorization": {fmt.Sprintf("Basic %s", encoded)},
	}

	ctx, err := ext.Authenticate(context.Background(), headers)
	require.NoError(t, err)
	assert.NotNil(t, ctx)
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	ext := &gcpSecretsManagerAuthServer{}
	matchFn := func(user, pass string) bool {
		return false
	}
	ext.matchFunc.Store(&matchFn)

	encoded := base64.StdEncoding.EncodeToString([]byte("admin:wrong"))
	headers := map[string][]string{
		"authorization": {fmt.Sprintf("Basic %s", encoded)},
	}

	_, err := ext.Authenticate(context.Background(), headers)
	assert.Error(t, err)
}

func TestAuthenticate_NoAuthHeader(t *testing.T) {
	ext := &gcpSecretsManagerAuthServer{}
	matchFn := func(_, _ string) bool { return true }
	ext.matchFunc.Store(&matchFn)

	headers := map[string][]string{}
	_, err := ext.Authenticate(context.Background(), headers)
	assert.Error(t, err)
}

func TestAuthenticate_NotStarted(t *testing.T) {
	ext := &gcpSecretsManagerAuthServer{}

	headers := map[string][]string{
		"authorization": {"Basic dGVzdDp0ZXN0"},
	}
	_, err := ext.Authenticate(context.Background(), headers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

// Helpers

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
