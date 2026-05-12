// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
)

const defaultRefreshInterval = time.Hour

var (
	errNoCredentialSource     = errors.New("no credential source provided")
	errMultipleAuthenticators = errors.New("only one of `htpasswd` or `client_auth` can be specified")
)

type HtpasswdSettings struct {
	// Path to the htpasswd file.
	File string `mapstructure:"file"`
	// Inline contents of the htpasswd file.
	Inline string `mapstructure:"inline"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// AWSSecretsManagerSettings configures credential retrieval from AWS Secrets Manager.
// The secret must be a JSON object containing the username and password fields.
// Credentials are polled at the configured interval and rotated in place without restarting the collector.
type AWSSecretsManagerSettings struct {
	// SecretARN is the ARN (or name) of the secret in AWS Secrets Manager. Required.
	SecretARN string `mapstructure:"secret_arn"`
	// Region is the AWS region. If empty, the SDK default chain is used (AWS_REGION env var, instance metadata, etc.).
	Region string `mapstructure:"region"`
	// RefreshInterval controls how often the secret is polled for changes. Defaults to 1h.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// UsernameKey is the JSON key for the username field. Defaults to "username".
	UsernameKey string `mapstructure:"username_key"`
	// PasswordKey is the JSON key for the password field. Defaults to "password".
	PasswordKey string `mapstructure:"password_key"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (s *AWSSecretsManagerSettings) usernameKey() string {
	if s.UsernameKey != "" {
		return s.UsernameKey
	}
	return "username"
}

func (s *AWSSecretsManagerSettings) passwordKey() string {
	if s.PasswordKey != "" {
		return s.PasswordKey
	}
	return "password"
}

func (s *AWSSecretsManagerSettings) refreshInterval() time.Duration {
	if s.RefreshInterval > 0 {
		return s.RefreshInterval
	}
	return defaultRefreshInterval
}

type ClientAuthSettings struct {
	// Username holds the username to use for client authentication.
	Username string `mapstructure:"username"`
	// UsernameFile points to a file that contains the username.
	// If set, takes precedence over Username. The file is watched for changes.
	UsernameFile string `mapstructure:"username_file"`
	// Password holds the password to use for client authentication.
	Password configopaque.String `mapstructure:"password"`
	// PasswordFile points to a file that contains the password.
	// If set, takes precedence over Password. The file is watched for changes.
	PasswordFile string `mapstructure:"password_file"`
	// AWSSecretsManager configures credential retrieval from AWS Secrets Manager.
	// Mutually exclusive with Username, UsernameFile, Password, and PasswordFile.
	AWSSecretsManager *AWSSecretsManagerSettings `mapstructure:"aws_secrets_manager,omitempty"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *ClientAuthSettings) Validate() error {
	if c.AWSSecretsManager != nil {
		if c.Username != "" || c.UsernameFile != "" || string(c.Password) != "" || c.PasswordFile != "" {
			return errors.New("aws_secrets_manager cannot be combined with username, username_file, password, or password_file")
		}
		if c.AWSSecretsManager.SecretARN == "" {
			return errors.New("aws_secrets_manager.secret_arn is required")
		}
	}
	return nil
}

type Config struct {
	// Htpasswd settings.
	Htpasswd *HtpasswdSettings `mapstructure:"htpasswd,omitempty"`

	// ClientAuth settings
	ClientAuth *ClientAuthSettings `mapstructure:"client_auth,omitempty"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	serverCondition := cfg.Htpasswd != nil
	clientCondition := cfg.ClientAuth != nil

	if serverCondition && clientCondition {
		return errMultipleAuthenticators
	}

	if !serverCondition && !clientCondition {
		return errNoCredentialSource
	}

	if clientCondition {
		return cfg.ClientAuth.Validate()
	}

	return nil
}
