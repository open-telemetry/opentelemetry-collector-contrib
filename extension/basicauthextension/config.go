// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
)

var (
	errNoCredentialSource        = errors.New("no credential source provided")
	errMultipleAuthenticators    = errors.New("only one of `htpasswd` or `client_auth` can be specified")
	errAWSSecretAndOtherSource   = errors.New("only one credential source allowed: choose `aws_secret` or inline/file, not both")
	errAWSSecretMissingARN       = errors.New("`aws_secret.secret_arn` is required")
	errAWSSecretMissingRegion    = errors.New("`aws_secret.region` is required")
	errAWSSecretMissingKeys      = errors.New("`aws_secret.username_key` and `aws_secret.password_key` are required for client_auth")
	errAWSSecretNegativeInterval = errors.New("`aws_secret.refresh_interval` must not be negative")
)

const defaultRefreshInterval = 5 * time.Minute

// AWSSecretClientConfig configures AWS Secrets Manager as a credential source for client auth.
// The secret must be a JSON object containing fields for username and password.
type AWSSecretClientConfig struct {
	SecretARN       string        `mapstructure:"secret_arn"`
	Region          string        `mapstructure:"region"`
	UsernameKey     string        `mapstructure:"username_key"`
	PasswordKey     string        `mapstructure:"password_key"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

// AWSSecretHtpasswdConfig configures AWS Secrets Manager as a credential source for server auth.
// The secret can be raw htpasswd content, or a JSON object with a field containing htpasswd content.
type AWSSecretHtpasswdConfig struct {
	SecretARN       string        `mapstructure:"secret_arn"`
	Region          string        `mapstructure:"region"`
	ValueKey        string        `mapstructure:"value_key"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

type HtpasswdSettings struct {
	// Path to the htpasswd file.
	File string `mapstructure:"file"`
	// Inline contents of the htpasswd file.
	Inline string `mapstructure:"inline"`
	// AWSSecret configures AWS Secrets Manager as the htpasswd source.
	AWSSecret *AWSSecretHtpasswdConfig `mapstructure:"aws_secret,omitempty"`
	// prevent unkeyed literal initialization
	_ struct{}
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
	// AWSSecret configures AWS Secrets Manager as the credential source.
	AWSSecret *AWSSecretClientConfig `mapstructure:"aws_secret,omitempty"`
	// prevent unkeyed literal initialization
	_ struct{}
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
		if err := cfg.ClientAuth.validate(); err != nil {
			return err
		}
	}

	if serverCondition {
		if err := cfg.Htpasswd.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *ClientAuthSettings) validate() error {
	if c.AWSSecret != nil {
		if c.Username != "" || c.UsernameFile != "" || string(c.Password) != "" || c.PasswordFile != "" {
			return errAWSSecretAndOtherSource
		}
		return c.AWSSecret.validate(true)
	}
	return nil
}

func (h *HtpasswdSettings) validate() error {
	if h.AWSSecret != nil {
		if h.File != "" || h.Inline != "" {
			return errAWSSecretAndOtherSource
		}
		return h.AWSSecret.validate()
	}
	return nil
}

func (c *AWSSecretClientConfig) validate(requireKeys bool) error {
	if c.SecretARN == "" {
		return errAWSSecretMissingARN
	}
	if c.Region == "" {
		return errAWSSecretMissingRegion
	}
	if requireKeys && (c.UsernameKey == "" || c.PasswordKey == "") {
		return errAWSSecretMissingKeys
	}
	if c.RefreshInterval < 0 {
		return errAWSSecretNegativeInterval
	}
	return nil
}

func (c *AWSSecretHtpasswdConfig) validate() error {
	if c.SecretARN == "" {
		return errAWSSecretMissingARN
	}
	if c.Region == "" {
		return errAWSSecretMissingRegion
	}
	if c.RefreshInterval < 0 {
		return errAWSSecretNegativeInterval
	}
	return nil
}
