// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestConfig_Validate_BothSet(t *testing.T) {
	cfg := &Config{
		Htpasswd:   &HtpasswdSettings{SecretARN: "arn", Region: "us-east-1"},
		ClientAuth: &ClientAuthSettings{SecretARN: "arn", Region: "us-east-1", UsernameKey: "u", PasswordKey: "p"},
	}
	assert.ErrorIs(t, cfg.Validate(), errMultipleAuthenticators)
}

func TestConfig_Validate_NeitherSet(t *testing.T) {
	cfg := &Config{}
	assert.ErrorIs(t, cfg.Validate(), errNoCredentialSource)
}

func TestConfig_Validate_ClientAuth_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ClientAuthSettings
		err  error
	}{
		{"missing secret_arn", &ClientAuthSettings{Region: "us-east-1", UsernameKey: "u", PasswordKey: "p"}, errMissingSecretARN},
		{"missing region", &ClientAuthSettings{SecretARN: "arn", UsernameKey: "u", PasswordKey: "p"}, errMissingRegion},
		{"missing username_key", &ClientAuthSettings{SecretARN: "arn", Region: "us-east-1", PasswordKey: "p"}, errMissingUsernameKey},
		{"missing password_key", &ClientAuthSettings{SecretARN: "arn", Region: "us-east-1", UsernameKey: "u"}, errMissingPasswordKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ClientAuth: tt.cfg}
			assert.ErrorIs(t, cfg.Validate(), tt.err)
		})
	}
}

func TestConfig_Validate_Htpasswd_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  *HtpasswdSettings
		err  error
	}{
		{"missing secret_arn", &HtpasswdSettings{Region: "us-east-1"}, errMissingSecretARN},
		{"missing region", &HtpasswdSettings{SecretARN: "arn"}, errMissingRegion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Htpasswd: tt.cfg}
			assert.ErrorIs(t, cfg.Validate(), tt.err)
		})
	}
}

func TestConfig_Validate_NegativeRefreshInterval(t *testing.T) {
	cfg := &Config{ClientAuth: &ClientAuthSettings{
		SecretARN:       "arn",
		Region:          "us-east-1",
		UsernameKey:     "u",
		PasswordKey:     "p",
		RefreshInterval: -1 * time.Second,
	}}
	assert.ErrorIs(t, cfg.Validate(), errNegativeRefreshInterval)
}

func TestConfig_Validate_ZeroRefreshInterval_GetsDefault(t *testing.T) {
	cfg := &Config{ClientAuth: &ClientAuthSettings{
		SecretARN:   "arn",
		Region:      "us-east-1",
		UsernameKey: "u",
		PasswordKey: "p",
	}}
	require.NoError(t, cfg.Validate())
	assert.Equal(t, defaultRefreshInterval, cfg.ClientAuth.RefreshInterval)
}

func TestConfig_Validate_ValidClientAuth(t *testing.T) {
	cfg := &Config{ClientAuth: &ClientAuthSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		UsernameKey:     "username",
		PasswordKey:     "password",
		RefreshInterval: 10 * time.Minute,
	}}
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_ValidHtpasswd(t *testing.T) {
	cfg := &Config{Htpasswd: &HtpasswdSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: 10 * time.Minute,
	}}
	assert.NoError(t, cfg.Validate())
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       string
		expected *Config
	}{
		{
			id: "awssecretsmanagerauth/client",
			expected: &Config{
				ClientAuth: &ClientAuthSettings{
					SecretARN:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret",
					Region:          "us-east-1",
					UsernameKey:     "username",
					PasswordKey:     "password",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
		{
			id: "awssecretsmanagerauth/server",
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					SecretARN:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-htpasswd",
					Region:          "us-east-1",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
		{
			id: "awssecretsmanagerauth/server_json",
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					SecretARN:       "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-json-secret",
					Region:          "us-east-1",
					ValueKey:        "htpasswd_content",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			sub, err := cm.Sub(tt.id)
			require.NoError(t, err)

			cfg := &Config{}
			require.NoError(t, sub.Unmarshal(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
