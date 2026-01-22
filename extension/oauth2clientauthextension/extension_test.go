// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestOAuthClientCredentialsSettingsConfig(t *testing.T) {
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
			name: "all_values",
			settings: &Config{
				ClientID:       "testclientid",
				ClientSecret:   "testsecret",
				EndpointParams: url.Values{"audience": []string{"someaudience"}},
				TokenURL:       "https://example.com/v1/token",
				Scopes:         []string{"resource.read"},
				Timeout:        2,
				ExpiryBuffer:   10 * time.Second,
			},
			expectedClientConfig: &clientcredentials.Config{
				ClientID:       "testclientid",
				ClientSecret:   "testsecret",
				TokenURL:       "https://example.com/v1/token",
				Scopes:         []string{"resource.read"},
				EndpointParams: url.Values{"audience": []string{"someaudience"}},
			},
			shouldError:   false,
			expectedError: nil,
		},
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
			credentials := newClientCredentialsGrantTypeConfig(test.settings)
			cfg, err := credentials.createConfig()
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

func TestOAuthJwtGrantTypeSettings(t *testing.T) {
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
				ClientID:               "testclientid",
				ClientCertificateKey:   "testsecret",
				EndpointParams:         url.Values{"audience": []string{"someaudience"}},
				TokenURL:               "https://example.com/v1/token",
				Scopes:                 []string{"resource.read"},
				SignatureAlgorithm:     "RS512",
				Iss:                    "my-issuer",
				Audience:               "my-audience",
				ClientCertificateKeyID: "1234",
				Claims:                 map[string]any{"random": "value"},
				Timeout:                2,
				ExpiryBuffer:           10 * time.Second,
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
			name: "empty_signature_algorithm_defaults",
			settings: &Config{
				SignatureAlgorithm: "RS256",
			},
			shouldError: false,
		},
		{
			name: "invalid_signature_algorithm_fails",
			settings: &Config{
				ClientID:             "testclientid",
				ClientCertificateKey: "testsecret",
				TokenURL:             "https://example.com/v1/token",
				SignatureAlgorithm:   "INVALID_ALGORITHM",
			},
			shouldError:   true,
			expectedError: "invalid signature algorithm",
		},
		{
			name: "iss_falls_back_to_client_id",
			settings: &Config{
				ClientID:           "testclientid",
				SignatureAlgorithm: "RS256",
				Iss:                "testclientid",
			},
			shouldError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rc, err := newJwtGrantTypeConfig(test.settings)
			if test.shouldError {
				assert.ErrorContains(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.settings.Scopes, rc.Scopes)
			assert.Equal(t, test.settings.TokenURL, rc.TokenURL)
			assert.EqualValues(t, test.settings.ClientCertificateKey, rc.PrivateKey)
			assert.Equal(t, test.settings.ClientID, rc.Subject)
			assert.Equal(t, test.settings.SignatureAlgorithm, rc.SigningAlgorithm.Name)
			assert.Equal(t, test.settings.ClientCertificateKeyID, rc.PrivateKeyID)
			assert.Equal(t, test.settings.EndpointParams, rc.EndpointParams)
			assert.Equal(t, test.settings.Iss, rc.Iss)
			assert.Equal(t, test.settings.Claims, rc.PrivateClaims)
		})
	}
}

type testRoundTripper struct {
	testString string
}

func (*testRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

func TestRoundTripper(t *testing.T) {
	tests := []struct {
		name        string
		settings    *Config
		shouldError bool
	}{
		{
			name: "returns_empty_grant_http_round_tripper",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			shouldError: false,
		},
		{
			name: "returns_client_credentials_grant_http_round_tripper",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				GrantType:    "client_credentials",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			shouldError: false,
		},
		{
			name: "returns_jwt_bearer_grant_http_round_tripper",
			settings: &Config{
				ClientID:             "testclientid",
				ClientCertificateKey: "testsecret",
				GrantType:            "urn:ietf:params:oauth:grant-type:jwt-bearer",
				TokenURL:             "https://example.com/v1/token",
				Scopes:               []string{"resource.read"},
			},
			shouldError: false,
		},
		{
			name: "invalid_grant_type_returns_error",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				GrantType:    "worong_grant_type",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
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
			name: "returns_client_credentials_grant_http_round_tripper",
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
			name: "returns_jwt_bearer_grant_http_round_tripper",
			settings: &Config{
				ClientID:             "testclientid",
				ClientCertificateKey: "testsecret",
				GrantType:            "urn:ietf:params:oauth:grant-type:jwt-bearer",
				TokenURL:             "https://example.com/v1/token",
				Scopes:               []string{"resource.read"},
				Timeout:              1,
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

	_, err = credential.GetRequestMetadata(t.Context())
	assert.ErrorIs(t, err, errFailedToGetSecurityToken)
	assert.ErrorContains(t, err, serverURL.String())

	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseRoundTripper := http.RoundTripper(transport)
	roundTripper, err := oauth2Authenticator.RoundTripper(baseRoundTripper)
	require.NoError(t, err)

	client := &http.Client{
		Transport: roundTripper,
	}

	req, err := http.NewRequest(http.MethodPost, "http://example.com/", http.NoBody)
	require.NoError(t, err)
	_, err = client.Do(req)
	assert.ErrorIs(t, err, errFailedToGetSecurityToken)
	assert.ErrorContains(t, err, serverURL.String())
}

func TestJwtOAuth(t *testing.T) {
	tokenTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()

		auth := r.Header.Get("Authorization")
		if auth == "" {
			assert.NoErrorf(t, r.ParseForm(), "Failed to parse form")
			auth = r.FormValue("assertion")
		}

		jwtParts := strings.Split(auth, ".")
		assert.Lenf(t, jwtParts, 3, "Expected JWT to have 3 parts, got %d", len(jwtParts))

		// Decode the JWT payload.
		payload, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
		assert.NoErrorf(t, err, "Failed to decode JWT payload: %v", err)

		var jwt struct {
			Aud     string `json:"aud"`
			Scope   string `json:"scope"`
			Sub     string `json:"sub"`
			Iss     string `json:"iss"`
			Integer int    `json:"integer"`
		}

		err = json.Unmarshal(payload, &jwt)
		assert.NoErrorf(t, err, "Failed to unmarshal JWT payload: %v", err)

		assert.Equalf(t, "common-test", jwt.Aud, "Expected aud to be 'common-test', got '%s'", jwt.Aud)
		assert.Equalf(t, "A B", jwt.Scope, "Expected scope to be 'A B', got '%s'", jwt.Scope)
		assert.Equalf(t, "common", jwt.Sub, "Expected sub to be 'common', got '%s'", jwt.Sub)
		assert.Equalf(t, "https://example.com", jwt.Iss, "Expected iss to be 'https://example.com', got '%s'", jwt.Iss)
		assert.Equalf(t, 1, jwt.Integer, "Expected integer to be 1, got '%d'", jwt.Integer)

		w.Header().Add("Content-Type", "application/json")

		type oauth2TestServerResponse struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
		}
		res, _ := json.Marshal(oauth2TestServerResponse{
			AccessToken: "12345",
			TokenType:   "Bearer",
		})
		w.Header().Add("Content-Type", "application/json")
		_, _ = w.Write(res)
	}))
	defer tokenTS.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		assert.Equalf(t, "Bearer 12345", auth, "bad auth, expected %s, got %s", "Bearer 12345", auth)
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	serverURL, err := url.Parse(tokenTS.URL)
	require.NoError(t, err)

	oauth2Authenticator, err := newClientAuthenticator(&Config{
		ClientID:                 "1",
		ClientCertificateKeyFile: "testdata/client.key",
		GrantType:                "urn:ietf:params:oauth:grant-type:jwt-bearer",
		Scopes:                   []string{"A", "B"},
		TokenURL:                 serverURL.String(),
		Claims: map[string]any{
			"iss":     "https://example.com",
			"aud":     "common-test",
			"sub":     "common",
			"integer": 1,
		},
		EndpointParams: url.Values{"hi": []string{"hello"}},
	}, zap.NewNop())
	require.NoError(t, err)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	baseRoundTripper := http.RoundTripper(transport)
	roundTripper, err := oauth2Authenticator.RoundTripper(baseRoundTripper)
	require.NoError(t, err)

	client := &http.Client{
		Transport: roundTripper,
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL, http.NoBody)
	require.NoError(t, err)
	_, err = client.Do(req)
	assert.NoError(t, err)
}
