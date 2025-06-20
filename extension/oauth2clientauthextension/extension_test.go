// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	grpcOAuth "google.golang.org/grpc/credentials/oauth"
)

func TestOAuthClientSettings(t *testing.T) {
	// test files for TLS testing
	var (
		testCAFile   = "testdata/testCA.pem"
		testCertFile = "testdata/test-cert.pem"
		testKeyFile  = "testdata/test-key.pem"
	)

	tests := []struct {
		name          string
		settings      *Config
		shouldError   bool
		expectedError string
	}{
		{
			name: "all_valid_settings",
			settings: &Config{
				ClientID:       "testclientid",
				ClientSecret:   "testsecret",
				EndpointParams: url.Values{"audience": []string{"someaudience"}},
				TokenURL:       "https://example.com/v1/token",
				Scopes:         []string{"resource.read"},
				Timeout:        2,
				ExpiryBuffer:   10 * time.Second,
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile:   testCAFile,
						CertFile: testCertFile,
						KeyFile:  testKeyFile,
					},
					Insecure:           false,
					InsecureSkipVerify: false,
				},
			},
			shouldError:   false,
			expectedError: "",
		},
		{
			name: "invalid_tls",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
				Timeout:      2,
				ExpiryBuffer: 15 * time.Second,
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile:   testCAFile,
						CertFile: "nonexistent.cert",
						KeyFile:  testKeyFile,
					},
					Insecure:           false,
					InsecureSkipVerify: false,
				},
			},
			shouldError:   true,
			expectedError: "failed to load TLS config: failed to load TLS cert and key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, err := newClientAuthenticator(test.settings, zap.NewNop())
			if test.shouldError {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}

			// test tls settings
			transport := rc.Transport().(*http.Transport)
			tlsClientConfig := transport.TLSClientConfig
			tlsTestSettingConfig, err := test.settings.TLS.LoadTLSConfig(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tlsClientConfig.Certificates, tlsTestSettingConfig.Certificates)
		})
	}
}

type testRoundTripper struct {
	testString string
}

func (b *testRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, nil
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
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			oauth2Authenticator, err := newClientAuthenticator(testcase.settings, zap.NewNop())
			if testcase.shouldError {
				assert.Error(t, err)
				assert.Nil(t, oauth2Authenticator)
				return
			}
			assert.NoError(t, err)
			perRPCCredentials, err := oauth2Authenticator.PerRPCCredentials()
			assert.NoError(t, err)
			// test perRPCCredentials is an grpc OAuthTokenSource
			_, ok := perRPCCredentials.(grpcOAuth.TokenSource)
			assert.True(t, ok)
		})
	}
}

func TestFailContactingOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("not-json"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	oauth2Authenticator, err := newClientAuthenticator(&Config{
		ClientID:     "dummy",
		ClientSecret: "ABC",
		TokenURL:     serverURL.String(),
	}, zap.NewNop())
	require.NoError(t, err)

	// Test for gRPC connections
	credential, err := oauth2Authenticator.PerRPCCredentials()
	require.NoError(t, err)

	_, err = credential.GetRequestMetadata(context.Background())
	assert.ErrorIs(t, err, errFailedToGetSecurityToken)
	assert.ErrorContains(t, err, serverURL.String())

	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseRoundTripper := (http.RoundTripper)(transport)
	roundTripper, err := oauth2Authenticator.RoundTripper(baseRoundTripper)
	require.NoError(t, err)

	client := &http.Client{
		Transport: roundTripper,
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.com/", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	assert.ErrorIs(t, err, errFailedToGetSecurityToken)
	assert.ErrorContains(t, err, serverURL.String())
}

func TestClientAuthenticatorMode(t *testing.T) {
	// test files for TLS testing

	tests := []struct {
		name          string
		settings      *Config
		shouldError   bool
		expectedError string
	}{
		{
			name: "test_create_two_legged_authenticator",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			shouldError:   false,
			expectedError: "",
		},
		{
			name: "test_create_sts_authenticator",
			settings: &Config{
				SubjectToken:     "testtoken",
				SubjectTokenType: "testtokentype",
				TokenURL:         "https://example.com/v1/token",
				Scopes:           []string{"resource.read"},
				Audience:         "testaudience",
			},
			shouldError:   false,
			expectedError: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, err := newClientAuthenticator(test.settings, zap.NewNop())

			if test.shouldError {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}

			assert.NoError(t, err)
			if test.settings.SubjectToken != "" {
				_, ok := rc.(*stsClientAuthenticator)
				assert.True(t, ok)
			} else {
				_, ok := rc.(*twoLeggedClientAuthenticator)
				assert.True(t, ok)
			}

		})
	}
}
