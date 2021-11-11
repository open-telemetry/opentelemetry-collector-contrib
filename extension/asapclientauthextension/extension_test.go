package asapclientauthextension

import (
	"context"
	"net/http"
	"testing"

	"bitbucket.org/atlassian/go-asap"

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
	return asap.NewPublicKey([]byte(TestPubKey))
}

var _ asap.KeyFetcher = (*mockKeyFetcher)(nil)

func TestRoundTripper(t *testing.T) {
	cfg := &Config{
		PrivateKey: TestPvtKey,
		Ttl:        60,
		Audience:   []string{"test"},
		Issuer:     "test_issuer",
		KeyId:      "test_issuer/test_kid",
	}

	asapAuth, err := createAsapClientAuthenticator(cfg)
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

	validateAsapJwt(t, cfg, tokenString)
}

func TestPerRPCCredentials(t *testing.T) {
	cfg := &Config{
		PrivateKey: TestPvtKey,
		Ttl:        60,
		Audience:   []string{"test"},
		Issuer:     "test_issuer",
		KeyId:      "test_issuer/test_kid",
	}

	asapAuth, _ := createAsapClientAuthenticator(cfg)
	credentials, err := asapAuth.PerRPCCredentials()
	assert.NoError(t, err)
	assert.NotNil(t, credentials)

	metadata, err := credentials.GetRequestMetadata(context.Background())
	tokenString := metadata["authorization"][7:]
	validateAsapJwt(t, cfg, tokenString)
}

func TestPerRPCAuth(t *testing.T) {
	metadata := map[string]string{
		"authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
	}

	// test meta data is properly stored
	perRPCAuth := &PerRPCAuth{metadata: metadata}
	md, err := perRPCAuth.GetRequestMetadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, md, metadata)

	// always true
	ok := perRPCAuth.RequireTransportSecurity()
	assert.True(t, ok)
}

// Helper function to validate token using the asap library and keypair from config_test.go
func validateAsapJwt(t *testing.T, cfg *Config, jwt string) {
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
