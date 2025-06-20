// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
