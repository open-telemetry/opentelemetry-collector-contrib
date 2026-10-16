// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"

import (
	"errors"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
)

// ClientConfig defines the NATS connection settings shared by the NATS
// exporter and receiver. Components embed it with `mapstructure:",squash"` so
// its fields sit at the top level of their configuration.
type ClientConfig struct {
	// Endpoint is the NATS server URL.
	Endpoint string `mapstructure:"endpoint"`

	// Pedantic enables NATS pedantic mode, which makes the server strictly
	// validate the subjects this connection publishes or subscribes to. It
	// defaults to false to mirror the NATS client default.
	//
	// See: https://docs.nats.io/reference/reference-protocols/nats-protocol#connect
	Pedantic bool `mapstructure:"pedantic"`

	// TLS holds the TLS configuration for the NATS client.
	TLS configtls.ClientConfig `mapstructure:"tls"`

	// Auth holds the configuration for NATS auth.
	Auth AuthConfig `mapstructure:"auth"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// NewDefaultClientConfig returns the default NATS connection settings.
func NewDefaultClientConfig() ClientConfig {
	return ClientConfig{
		Endpoint: nats.DefaultURL,
		TLS:      configtls.NewDefaultClientConfig(),
	}
}

// TokenConfig defines the configuration for token auth.
type TokenConfig struct {
	Token configopaque.String `mapstructure:"token"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// UserConfig defines the configuration for username/password auth.
type UserConfig struct {
	Username string              `mapstructure:"username"`
	Password configopaque.String `mapstructure:"password"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// NkeyConfig defines the configuration for NKey auth.
type NkeyConfig struct {
	PublicKey string `mapstructure:"public_key"`
	Seed      []byte `mapstructure:"seed"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// NkeyJWTConfig defines the configuration for NKey auth via JWT.
type NkeyJWTConfig struct {
	JWT  configopaque.String `mapstructure:"jwt"`
	Seed []byte              `mapstructure:"seed"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// NkeyUserFileConfig defines the configuration for NKey auth via a credentials
// (user) file.
type NkeyUserFileConfig struct {
	UserFilePath string `mapstructure:"user_file"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// AuthConfig defines the auth configuration for the NATS client. At most one
// auth method may be configured.
//
// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro
type AuthConfig struct {
	Token        *TokenConfig        `mapstructure:"token"`
	User         *UserConfig         `mapstructure:"user"`
	Nkey         *NkeyConfig         `mapstructure:"nkey"`
	NkeyJWT      *NkeyJWTConfig      `mapstructure:"nkey_jwt"`
	NkeyUserFile *NkeyUserFileConfig `mapstructure:"nkey_user_file"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

func (c *TokenConfig) Validate() error {
	if c.Token == "" {
		return errors.New("incomplete token auth configuration")
	}
	return nil
}

func (c *UserConfig) Validate() error {
	if c.Username == "" || c.Password == "" {
		return errors.New("incomplete username/password auth configuration")
	}
	return nil
}

func (c *NkeyConfig) Validate() error {
	if c.PublicKey == "" || c.Seed == nil {
		return errors.New("incomplete NKey auth configuration")
	}
	return nil
}

func (c *NkeyJWTConfig) Validate() error {
	if c.JWT == "" || c.Seed == nil {
		return errors.New("incomplete NKey auth (via JWT) configuration")
	}
	return nil
}

func (c *NkeyUserFileConfig) Validate() error {
	if c.UserFilePath == "" {
		return errors.New("incomplete NKey auth (via user file) configuration")
	}
	return nil
}

func (c *AuthConfig) Validate() error {
	var errs error
	configured := 0
	if c.Token != nil {
		configured++
		errs = multierr.Append(errs, c.Token.Validate())
	}
	if c.User != nil {
		configured++
		errs = multierr.Append(errs, c.User.Validate())
	}
	if c.Nkey != nil {
		configured++
		errs = multierr.Append(errs, c.Nkey.Validate())
	}
	if c.NkeyJWT != nil {
		configured++
		errs = multierr.Append(errs, c.NkeyJWT.Validate())
	}
	if c.NkeyUserFile != nil {
		configured++
		errs = multierr.Append(errs, c.NkeyUserFile.Validate())
	}

	// At most one auth method may be configured.
	if configured > 1 {
		errs = multierr.Append(errs, errors.New("more than one auth method configured"))
	}
	return errs
}
