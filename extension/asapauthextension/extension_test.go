// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension

import (
	"context"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/atlassian/go-asap/v2"
	"github.com/stretchr/testify/assert"
)

// mockRoundTripper copies the request headers to the response.
// So, the caller can make assertions using the returned response.
type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: map[string][]string{}}
	for k, v := range req.Header {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

var _ http.RoundTripper = (*mockRoundTripper)(nil)

// mockKeyFetcher implements asap.KeyFetcher, eliminating the need to contact a key server.
type mockKeyFetcher struct{}

func (k *mockKeyFetcher) Fetch(_ string) (interface{}, error) {
	return asap.NewPublicKey([]byte(publicKey))
}

var _ asap.KeyFetcher = (*mockKeyFetcher)(nil)

func TestRoundTripper(t *testing.T) {
	cfg := &Config{
		PrivateKey: privateKey,
		TTL:        60 * time.Second,
		Audience:   []string{"test"},
		Issuer:     "test_issuer",
		KeyID:      "test_issuer/test_kid",
	}

	asapAuth, err := createASAPClientAuthenticator(cfg)
	assert.NoError(t, err)

	base := &mockRoundTripper{}
	roundTripper, err := asapAuth.RoundTripper(base)
	assert.NoError(t, err)
	assert.NotNil(t, roundTripper)

	req := &http.Request{Method: "Get", Header: map[string][]string{}}
	resp, err := roundTripper.RoundTrip(req)
	assert.NoError(t, err)
	authHeaderValue := resp.Header.Get("Authorization")
	tokenString := authHeaderValue[7:] // Remove prefix "Bearer "

	validateAsapJWT(t, cfg, tokenString)
}

// TestRPCAuth tests RPC authentication, obtaining an auth header and validating the JWT inside it
func TestRPCAuth(t *testing.T) {
	cfg := &Config{
		PrivateKey: privateKey,
		TTL:        60 * time.Second,
		Audience:   []string{"test"},
		Issuer:     "test_issuer",
		KeyID:      "test_issuer/test_kid",
	}

	asapAuth, err := createASAPClientAuthenticator(cfg)
	assert.NoError(t, err)

	// Setup credentials
	credentials, err := asapAuth.PerRPCCredentials()
	assert.NoError(t, err)
	assert.NotNil(t, credentials)
	assert.True(t, credentials.RequireTransportSecurity())

	// Generate auth header
	metadata, err := credentials.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	tokenString := metadata["authorization"][7:] // Remove "Bearer " from front
	validateAsapJWT(t, cfg, tokenString)
}

// Helper function to validate token using the asap library and keypair from config_test.go
func validateAsapJWT(t *testing.T, cfg *Config, jwt string) {
	validator := asap.NewValidatorChain(
		asap.NewSignatureValidator(&mockKeyFetcher{}),
		asap.NewAllowedAudienceValidator(cfg.Audience[0]),
		asap.DefaultValidator,
	)
	token, err := asap.ParseToken(jwt)
	assert.NotNil(t, token)
	assert.NoError(t, err)

	err = validator.Validate(token)
	assert.NoError(t, err)
}
