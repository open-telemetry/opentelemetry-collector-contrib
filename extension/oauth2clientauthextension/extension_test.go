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
	"golang.org/x/oauth2/clientcredentials"
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
			assert.NoError(t, err)
			assert.Equal(t, test.settings.Scopes, rc.clientCredentials.Scopes)
			assert.Equal(t, test.settings.TokenURL, rc.clientCredentials.TokenURL)
			assert.EqualValues(t, test.settings.ClientSecret, rc.clientCredentials.ClientSecret)
			assert.Equal(t, test.settings.ClientID, rc.clientCredentials.ClientID)
			assert.Equal(t, test.settings.Timeout, rc.client.Timeout)
			assert.Equal(t, test.settings.ExpiryBuffer, rc.clientCredentials.ExpiryBuffer)
			assert.Equal(t, test.settings.EndpointParams, rc.clientCredentials.EndpointParams)

			// test tls settings
			transport := rc.client.Transport.(*http.Transport)
			tlsClientConfig := transport.TLSClientConfig
			tlsTestSettingConfig, err := test.settings.TLS.LoadTLSConfig(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tlsClientConfig.Certificates, tlsTestSettingConfig.Certificates)
		})
	}
}

func TestOAuthClientSettingsCredsConfig(t *testing.T) {
	var (
		testCredsFile        = "testdata/test-cred.txt"
		testCredsEmptyFile   = "testdata/test-cred-empty.txt"
		testCredsMissingFile = "testdata/test-cred-missing.txt"
	)

	tests := []struct {
		name                 string
		settings             *Config
		expectedClientConfig *clientcredentials.Config
		shouldError          bool
		expectedError        error
	}{
		{
			name: "client_id_file",
			settings: &Config{
				ClientIDFile: testCredsFile,
				ClientSecret: "testsecret",
			},
			expectedClientConfig: &clientcredentials.Config{
				ClientID:     "testcreds",
				ClientSecret: "testsecret",
			},
			shouldError:   false,
			expectedError: nil,
		},
		{
			name: "client_secret_file",
			settings: &Config{
				ClientID:         "testclientid",
				ClientSecretFile: testCredsFile,
			},
			expectedClientConfig: &clientcredentials.Config{
				ClientID:     "testclientid",
				ClientSecret: "testcreds",
			},
			shouldError:   false,
			expectedError: nil,
		},
		{
			name: "empty_client_creds_file",
			settings: &Config{
				ClientIDFile: testCredsEmptyFile,
				ClientSecret: "testsecret",
			},
			shouldError:   true,
			expectedError: errNoClientIDProvided,
		},
		{
			name: "missing_client_creds_file",
			settings: &Config{
				ClientID:         "testclientid",
				ClientSecretFile: testCredsMissingFile,
			},
			shouldError:   true,
			expectedError: errNoClientSecretProvided,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, _ := newClientAuthenticator(test.settings, zap.NewNop())
			cfg, err := rc.clientCredentials.createConfig()
			if test.shouldError {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedClientConfig.ClientID, cfg.ClientID)
			assert.Equal(t, test.expectedClientConfig.ClientSecret, cfg.ClientSecret)
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
