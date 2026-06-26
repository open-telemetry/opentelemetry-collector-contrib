// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

var (
	errNoCredentialSource              = errors.New("no credential source provided")
	errMultipleAuthenticators          = errors.New("only one of `htpasswd` or `client_auth` can be specified")
	errSecretProviderAndOtherSource    = errors.New("only one credential source allowed: choose `secret_provider` or inline/file, not both")
	errSecretProviderMissingID         = errors.New("`secret_provider.id` is required")
	errSecretProviderMissingKeys       = errors.New("`secret_provider.username_key` and `secret_provider.password_key` are required for client_auth")
)

// SecretProviderConfig references an external secret-providing extension by component ID.
// The referenced extension must implement the SecretProvider interface.
type SecretProviderConfig struct {
	// ID is the component ID of the secret provider extension.
	ID component.ID `mapstructure:"id"`
	// UsernameKey is the JSON key for the username in the secret value (client_auth only).
	UsernameKey string `mapstructure:"username_key,omitempty"`
	// PasswordKey is the JSON key for the password in the secret value (client_auth only).
	PasswordKey string `mapstructure:"password_key,omitempty"`
}

type HtpasswdSettings struct {
	// Path to the htpasswd file.
	File string `mapstructure:"file"`
	// Inline contents of the htpasswd file.
	Inline string `mapstructure:"inline"`
	// SecretProvider references an external extension that supplies the htpasswd content.
	SecretProvider *SecretProviderConfig `mapstructure:"secret_provider,omitempty"`
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
	// SecretProvider references an external extension that supplies credentials as a JSON secret.
	SecretProvider *SecretProviderConfig `mapstructure:"secret_provider,omitempty"`
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
	if c.SecretProvider != nil {
		if c.Username != "" || c.UsernameFile != "" || string(c.Password) != "" || c.PasswordFile != "" {
			return errSecretProviderAndOtherSource
		}
		return c.SecretProvider.validate(true)
	}
	return nil
}

func (h *HtpasswdSettings) validate() error {
	if h.SecretProvider != nil {
		if h.File != "" || h.Inline != "" {
			return errSecretProviderAndOtherSource
		}
		return h.SecretProvider.validate(false)
	}
	return nil
}

func (c *SecretProviderConfig) validate(clientMode bool) error {
	if c.ID == (component.ID{}) {
		return errSecretProviderMissingID
	}
	if clientMode && (c.UsernameKey == "" || c.PasswordKey == "") {
		return errSecretProviderMissingKeys
	}
	return nil
}
