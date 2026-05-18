// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpsecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/gcpsecretsmanagerauthextension"

import (
	"errors"
	"time"
)

const defaultRefreshInterval = 5 * time.Minute

var (
	errNoCredentialSource      = errors.New("either htpasswd or client_auth must be set")
	errMultipleAuthenticators  = errors.New("htpasswd and client_auth are mutually exclusive")
	errMissingProject          = errors.New("project is required")
	errMissingSecretName       = errors.New("secret_name is required")
	errMissingUsernameKey      = errors.New("username_key is required")
	errMissingPasswordKey      = errors.New("password_key is required")
	errNegativeRefreshInterval = errors.New("refresh_interval must not be negative")
)

type Config struct {
	Htpasswd   *HtpasswdSettings   `mapstructure:"htpasswd,omitempty"`
	ClientAuth *ClientAuthSettings `mapstructure:"client_auth,omitempty"`
}

type HtpasswdSettings struct {
	Project         string        `mapstructure:"project"`
	SecretName      string        `mapstructure:"secret_name"`
	ValueKey        string        `mapstructure:"value_key,omitempty"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

type ClientAuthSettings struct {
	Project         string        `mapstructure:"project"`
	SecretName      string        `mapstructure:"secret_name"`
	UsernameKey     string        `mapstructure:"username_key"`
	PasswordKey     string        `mapstructure:"password_key"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

func (cfg *Config) Validate() error {
	if cfg.Htpasswd != nil && cfg.ClientAuth != nil {
		return errMultipleAuthenticators
	}
	if cfg.Htpasswd == nil && cfg.ClientAuth == nil {
		return errNoCredentialSource
	}

	if cfg.Htpasswd != nil {
		return cfg.Htpasswd.validate()
	}
	return cfg.ClientAuth.validate()
}

func (h *HtpasswdSettings) validate() error {
	if h.Project == "" {
		return errMissingProject
	}
	if h.SecretName == "" {
		return errMissingSecretName
	}
	if h.RefreshInterval < 0 {
		return errNegativeRefreshInterval
	}
	if h.RefreshInterval == 0 {
		h.RefreshInterval = defaultRefreshInterval
	}
	return nil
}

func (c *ClientAuthSettings) validate() error {
	if c.Project == "" {
		return errMissingProject
	}
	if c.SecretName == "" {
		return errMissingSecretName
	}
	if c.UsernameKey == "" {
		return errMissingUsernameKey
	}
	if c.PasswordKey == "" {
		return errMissingPasswordKey
	}
	if c.RefreshInterval < 0 {
		return errNegativeRefreshInterval
	}
	if c.RefreshInterval == 0 {
		c.RefreshInterval = defaultRefreshInterval
	}
	return nil
}
