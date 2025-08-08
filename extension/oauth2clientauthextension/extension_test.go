// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type testRoundTripper struct {
	testString string
}

func (b *testRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestOAuthClientSettings(t *testing.T) {
	// test files for TLS testing
	var (
		testCAFile     = "testdata/ca.crt"
		serverCertFile = "testdata/server.crt"
		serverKeyFile  = "testdata/server-key.pem"
	)
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	assert.NoError(t, err)

	// Create a TLS configuration with the custom certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
	}

	// Create a listener

	authServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseMap := map[string]string{
			"access_token":  "SlAV32hkKG",
			"token_type":    "Bearer",
			"refresh_token": "8xLotade",
		}
		response, _ := json.Marshal(responseMap)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, err = w.Write([]byte(response))
		assert.NoError(t, err)
	}))

	authServer.TLS = tlsConfig
	authServer.StartTLS()

	authURL, err := url.Parse(authServer.URL)
	assert.NoError(t, err)

	defer authServer.Close()

	tests := []struct {
		name          string
		settings      *Config
		shouldError   bool
		expectedError string
	}{
		{
			name: "request_fails_without_custom_CA",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     authURL.String(),
				TLS: configtls.ClientConfig{
					Insecure:           false,
					InsecureSkipVerify: false,
				},
			},
			shouldError: true,
			// Server CA should be rejected
			expectedError: "failed to get security token from token endpoint",
		},
		{
			name: "use_custom_tls_config",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     authURL.String(),
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: testCAFile,
					},
					Insecure:           false,
					InsecureSkipVerify: false,
				},
			},
			shouldError: true,
			// This error means a the request was successful, but the test token is not valid
			expectedError: "unable to transfer TokenSource PerRPCCredentials: AuthInfo is nil",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, err := newClientAuthenticator(test.settings, zap.NewNop())
			require.NoError(t, err)

			// test tls settings
			credential, err := rc.PerRPCCredentials()
			require.NoError(t, err)

			_, err = credential.GetRequestMetadata(context.Background())
			if test.shouldError {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)

		})
	}
}

func TestRoundTripper(t *testing.T) {
	tests := []struct {
		name        string
		settings    *Config
		shouldError bool
	}{
		{
			name: "returns_http_round_tripper",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			shouldError: false,
		},
	}

	testString := "TestString"
	baseRoundTripper := &testRoundTripper{testString}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			oauth2Authenticator, err := newClientAuthenticator(testcase.settings, zap.NewNop())
			if testcase.shouldError {
				assert.Error(t, err)
				assert.Nil(t, oauth2Authenticator)
				return
			}

			assert.NotNil(t, oauth2Authenticator)
			roundTripper, err := oauth2Authenticator.RoundTripper(baseRoundTripper)
			assert.NoError(t, err)

			// test roundTripper is an OAuth RoundTripper
			oAuth2Transport, ok := roundTripper.(*oauth2.Transport)
			assert.True(t, ok)

			// test oAuthRoundTripper wrapped the base roundTripper properly
			wrappedRoundTripper, ok := oAuth2Transport.Base.(*testRoundTripper)
			assert.True(t, ok)
			assert.Equal(t, wrappedRoundTripper.testString, testString)
		})
	}
}

func TestOAuth2PerRPCCredentials(t *testing.T) {
	tests := []struct {
		name        string
		settings    *Config
		shouldError bool
		expectedErr error
	}{
		{
			name: "returns_http_round_tripper",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
				Timeout:      1,
			},
			shouldError: false,
		},
		{
			name: "test_create_sts_authenticator",
			settings: &Config{
				SubjectToken:     "testtoken",
				SubjectTokenType: "testtokentype",
				TokenURL:         "https://example.com/v1/token",
				Scopes:           []string{"resource.read"},
				Audience:         "testaudience",
				AuthMode:         "sts",
			},
			shouldError: false,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			oauth2Authenticator, err := newClientAuthenticator(testcase.settings, zap.NewNop())
			assert.NoError(t, err)
			perRPCCredentials, err := oauth2Authenticator.PerRPCCredentials()
			assert.NoError(t, err)
			assert.NotNil(t, perRPCCredentials)
		})
	}
}

func TestClientAuthenticatorMode(t *testing.T) {

	tests := []struct {
		name         string
		settings     *Config
		expectedType any
	}{
		{
			name: "test_defaults_to_client_credentials_mode",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			expectedType: &twoLeggedClientAuthenticator{},
		},
		{
			name: "test_create_client_credentials",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
				AuthMode:     "client-credentials",
			},
			expectedType: &twoLeggedClientAuthenticator{},
		},
		{
			name: "test_create_sts_authenticator",
			settings: &Config{
				SubjectToken:     "testtoken",
				SubjectTokenType: "testtokentype",
				TokenURL:         "https://example.com/v1/token",
				Scopes:           []string{"resource.read"},
				Audience:         "testaudience",
				AuthMode:         "sts",
			},
			expectedType: &stsClientAuthenticator{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, err := newClientAuthenticator(test.settings, zap.NewNop())
			assert.NoError(t, err)
			assert.IsType(t, test.expectedType, rc)
		})
	}
}
