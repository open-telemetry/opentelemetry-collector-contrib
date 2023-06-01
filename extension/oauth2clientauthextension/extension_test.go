// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
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
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
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
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile:   testCAFile,
						CertFile: "doestexist.cert",
						KeyFile:  testKeyFile,
					},
					Insecure:           false,
					InsecureSkipVerify: false,
				},
			},
			shouldError:   true,
			expectedError: "failed to load TLS config: failed to load TLS cert and key",
		},
		{
			name: "missing_client_id",
			settings: &Config{
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			shouldError:   true,
			expectedError: errNoClientIDProvided.Error(),
		},
		{
			name: "missing_client_secret",
			settings: &Config{
				ClientID: "testclientid",
				TokenURL: "https://example.com/v1/token",
				Scopes:   []string{"resource.read"},
			},
			shouldError:   true,
			expectedError: errNoClientSecretProvided.Error(),
		},
		{
			name: "missing_token_url",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				Scopes:       []string{"resource.read"},
			},
			shouldError:   true,
			expectedError: errNoTokenURLProvided.Error(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, err := newClientAuthenticator(test.settings, zap.NewNop())
			if test.shouldError {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.settings.Scopes, rc.clientCredentials.Scopes)
			assert.Equal(t, test.settings.TokenURL, rc.clientCredentials.TokenURL)
			assert.EqualValues(t, test.settings.ClientSecret, rc.clientCredentials.ClientSecret)
			assert.Equal(t, test.settings.ClientID, rc.clientCredentials.ClientID)
			assert.Equal(t, test.settings.Timeout, rc.client.Timeout)
			assert.Equal(t, test.settings.EndpointParams, rc.clientCredentials.EndpointParams)

			// test tls settings
			transport := rc.client.Transport.(*http.Transport)
			tlsClientConfig := transport.TLSClientConfig
			tlsTestSettingConfig, err := test.settings.TLSSetting.LoadTLSConfig()
			assert.Nil(t, err)
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
		{
			name: "invalid_client_settings_should_error",
			settings: &Config{
				ClientID: "testclientid",
				TokenURL: "https://example.com/v1/token",
				Scopes:   []string{"resource.read"},
			},
			shouldError: true,
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
			roundTripper, err := oauth2Authenticator.roundTripper(baseRoundTripper)
			assert.Nil(t, err)

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
			name: "invalid_client_settings_should_error",
			settings: &Config{
				ClientID: "testclientid",
				TokenURL: "https://example.com/v1/token",
				Scopes:   []string{"resource.read"},
			},
			shouldError: true,
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
			perRPCCredentials, err := oauth2Authenticator.perRPCCredentials()
			assert.Nil(t, err)
			// test perRPCCredentials is an grpc OAuthTokenSource
			_, ok := perRPCCredentials.(grpcOAuth.TokenSource)
			assert.True(t, ok)
		})
	}
}

func TestFailContactingOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, err := w.Write([]byte("not-json"))
		assert.NoError(t, err)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	assert.NoError(t, err)

	oauth2Authenticator, err := newClientAuthenticator(&Config{
		ClientID:     "dummy",
		ClientSecret: "ABC",
		TokenURL:     serverURL.String(),
	}, zap.NewNop())
	assert.Nil(t, err)

	// Test for gRPC connections
	credential, err := oauth2Authenticator.perRPCCredentials()
	assert.Nil(t, err)

	_, err = credential.GetRequestMetadata(context.Background())
	assert.ErrorIs(t, err, errFailedToGetSecurityToken)
	assert.Contains(t, err.Error(), serverURL.String())

	// Test for HTTP connections
	setting := confighttp.HTTPClientSettings{
		Endpoint: "http://example.com/",
		CustomRoundTripper: func(next http.RoundTripper) (http.RoundTripper, error) {
			return oauth2Authenticator.roundTripper(next)
		},
	}

	client, _ := setting.ToClient(componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	req, err := http.NewRequest("POST", setting.Endpoint, nil)
	assert.NoError(t, err)
	_, err = client.Do(req)
	assert.ErrorIs(t, err, errFailedToGetSecurityToken)
	assert.Contains(t, err.Error(), serverURL.String())
}
