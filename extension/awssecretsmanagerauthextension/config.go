// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension"

import (
	"errors"
	"fmt"
	"time"
)

var (
	errNoCredentialSource     = errors.New("no credential source provided")
	errMultipleAuthenticators = errors.New("only one of `htpasswd` or `client_auth` can be specified")
)

// HtpasswdSettings configures server-side authentication. The secret value is
// treated as htpasswd file content. If ValueKey is set, the secret must be a
// JSON object and the value at that key is used as the htpasswd content.
type HtpasswdSettings struct {
	// ValueKey is the JSON key in the secret whose value contains the htpasswd content.
	// If empty, the entire secret string is used as-is.
	ValueKey string `mapstructure:"value_key"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// ClientAuthSettings configures client-side authentication. The secret must be
// a JSON object. UsernameKey and PasswordKey identify which fields to use.
type ClientAuthSettings struct {
	// UsernameKey is the JSON key whose value is used as the username.
	// Defaults to "username".
	UsernameKey string `mapstructure:"username_key"`
	// PasswordKey is the JSON key whose value is used as the password.
	// Defaults to "password".
	PasswordKey string `mapstructure:"password_key"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Config defines the configuration for the awssecretsmanagerauth extension.
type Config struct {
	// SecretARN is the ARN or name of the secret in AWS Secrets Manager.
	SecretARN string `mapstructure:"secret_arn"`
	// RefreshInterval controls how often the extension polls for a new secret version.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// Htpasswd configures server-side (inbound) authentication.
	// Exactly one of Htpasswd or ClientAuth must be set.
	Htpasswd *HtpasswdSettings `mapstructure:"htpasswd,omitempty"`
	// ClientAuth configures client-side (outbound) authentication.
	// Exactly one of Htpasswd or ClientAuth must be set.
	ClientAuth *ClientAuthSettings `mapstructure:"client_auth,omitempty"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	if cfg.SecretARN == "" {
		return errors.New("secret_arn must not be empty")
	}
	if cfg.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be positive, got %s", cfg.RefreshInterval)
	}

	serverSet := cfg.Htpasswd != nil
	clientSet := cfg.ClientAuth != nil

	if serverSet && clientSet {
		return errMultipleAuthenticators
	}
	if !serverSet && !clientSet {
		return errNoCredentialSource
	}
	return nil
}
