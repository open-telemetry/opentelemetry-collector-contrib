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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/oauth2/clientcredentials"
)

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
			rc, _ := newTwoLeggedClientAuthenticator(test.settings, zap.NewNop())
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
