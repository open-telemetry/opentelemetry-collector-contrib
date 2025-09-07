// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

// SignalConfig defines the configuration for a signal type.
type SignalConfig struct {
	// Subject is the OTTL value expression used to construct the NATS subject.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md
	//
	// See: https://docs.nats.io/nats-concepts/subjects#subject-based-filtering-and-security
	Subject string `mapstructure:"subject"`

	// BuiltinMarshalerName is the name of the built-in marshaler to use when marshaling the signal type.
	//
	// Supported marshalers:
	//  - otlp_proto
	//  - otlp_json
	BuiltinMarshalerName marshaler.BuiltinMarshalerName `mapstructure:"marshaler"`
	// EncodingExtensionName is the name of the encoding extension to use when marshaling the signal type.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding
	EncodingExtensionName string `mapstructure:"encoding_extension"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// LogsConfig defines the configuration for logs.
type LogsConfig SignalConfig

// MetricsConfig defines the configuration for metrics.
type MetricsConfig SignalConfig

// TracesConfig defines the configuration for traces.
type TracesConfig SignalConfig

// TokenConfig defines the configuration for token auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#Token
type TokenConfig struct {
	// Token is the token to use for token auth.
	Token string `mapstructure:"token"`
}

// UserConfig defines the configuration for username/password auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserInfo
type UserConfig struct {
	// User is the username to use for username/password auth.
	Username string `mapstructure:"username"`
	// Password is the password to use for username/password auth.
	Password string `mapstructure:"password"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// NkeyConfig defines the configuration for NKey auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#Nkey
type NkeyConfig struct {
	// PublicKey is the public key to use for NKey auth.
	PublicKey string `mapstructure:"public_key"`
	// Seed is the seed to use for NKey auth.
	Seed []byte `mapstructure:"seed"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// NkeyJWTConfig defines the configuration for NKey auth via JWT.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserJWT
type NkeyJWTConfig struct {
	// JWT is the JWT to use for NKey auth via JWT.
	JWT string `mapstructure:"jwt"`
	// Seed is the seed to use for NKey auth via JWT.
	Seed []byte `mapstructure:"seed"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// NkeyUserFileConfig defines the configuration for NKey auth via user file.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserCredentials
type NkeyUserFileConfig struct {
	// UserFilePath is the path to the user file to use for NKey auth via user file.
	UserFilePath string `mapstructure:"user_file"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// AuthConfig defines the auth configuration for the NATS client.
//
// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro
type AuthConfig struct {
	// Token holds the configuration for token auth.
	Token *TokenConfig `mapstructure:"token"`

	// User holds the configuration for username/password auth.
	User *UserConfig `mapstructure:"user"`

	// Nkey holds the configuration for NKey auth.
	Nkey *NkeyConfig `mapstructure:"nkey"`

	// NkeyJWT holds the configuration for NKey auth via JWT.
	NkeyJWT *NkeyJWTConfig `mapstructure:"nkey_jwt"`

	// NkeyUserFile holds the configuration for NKey auth via user file.
	NkeyUserFile *NkeyUserFileConfig `mapstructure:"nkey_user_file"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// Config defines the configuration for the NATS core exporter.
type Config struct {
	// Endpoint is the NATS server URL.
	Endpoint string `mapstructure:"endpoint"`

	// Pedantic is the option to enable/disable NATS pedantic mode.
	Pedantic bool `mapstructure:"pedantic"`

	// TLS holds the TLS configuration for the NATS client.
	TLS configtls.ClientConfig `mapstructure:"tls"`

	// Logs holds the configuration for the logs signal.
	Logs LogsConfig `mapstructure:"logs"`
	// Metrics holds the configuration for the metrics signal.
	Metrics MetricsConfig `mapstructure:"metrics"`
	// Traces holds the configuration for the traces signal.
	Traces TracesConfig `mapstructure:"traces"`

	// Auth holds the configuration for NATS auth.
	Auth AuthConfig `mapstructure:",squash"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

func (c *SignalConfig) Validate() error {
	var errs error

	if c.BuiltinMarshalerName != "" && c.EncodingExtensionName != "" {
		errs = multierr.Append(errs, errors.New("marshaler configured more than once"))
	}

	if c.BuiltinMarshalerName != "" {
		if c.BuiltinMarshalerName != marshaler.OtlpProtoBuiltinMarshalerName &&
			c.BuiltinMarshalerName != marshaler.OtlpJSONBuiltinMarshalerName {
			errs = multierr.Append(errs, fmt.Errorf("unsupported built-in marshaler: %s", c.BuiltinMarshalerName))
		}
	}

	if c.EncodingExtensionName != "" {
		var id component.ID
		if err := id.UnmarshalText([]byte(c.EncodingExtensionName)); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to unmarshal encoding extension name: %w", err))
		}
	}

	return errs
}

func (c *LogsConfig) Validate() error {
	errs := (*SignalConfig)(c).Validate()

	if c.Subject != "" {
		parser, err := ottllog.NewParser(
			ottlfuncs.StandardConverters[ottllog.TransformContext](),
			componenttest.NewNopTelemetrySettings(),
		)
		if err != nil {
			panic(fmt.Errorf("failed to create logs parser: %w", err))
		}

		if _, err = parser.ParseValueExpression(c.Subject); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to parse logs subject: %w", err))
		}
	}

	return errs
}

func (c *MetricsConfig) Validate() error {
	errs := (*SignalConfig)(c).Validate()

	if c.Subject != "" {
		parser, err := ottlmetric.NewParser(
			ottlfuncs.StandardConverters[ottlmetric.TransformContext](),
			componenttest.NewNopTelemetrySettings(),
		)
		if err != nil {
			panic(fmt.Errorf("failed to create metrics parser: %w", err))
		}

		if _, err = parser.ParseValueExpression(c.Subject); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to parse metrics subject: %w", err))
		}
	}

	return errs
}

func (c *TracesConfig) Validate() error {
	errs := (*SignalConfig)(c).Validate()

	if c.Subject != "" {
		parser, err := ottlspan.NewParser(
			ottlfuncs.StandardConverters[ottlspan.TransformContext](),
			componenttest.NewNopTelemetrySettings(),
		)
		if err != nil {
			panic(fmt.Errorf("failed to create traces parser: %w", err))
		}

		if _, err = parser.ParseValueExpression(c.Subject); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to parse traces subject: %w", err))
		}
	}

	return errs
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

	if c.Token != nil {
		if err := c.Token.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.User != nil {
		if err := c.User.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.Nkey != nil {
		if err := c.Nkey.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.NkeyJWT != nil {
		if err := c.NkeyJWT.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.NkeyUserFile != nil {
		if err := c.NkeyUserFile.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	isConfiguredCount := 0
	for _, isConfigured := range []bool{
		c.Nkey != nil,
		c.NkeyJWT != nil,
		c.NkeyUserFile != nil,
	} {
		if isConfigured {
			isConfiguredCount++
		}
	}
	if isConfiguredCount > 1 {
		errs = multierr.Append(errs, errors.New("NKey auth configured more than once"))
	}

	return errs
}

func (c *Config) Validate() error {
	var errs error
	if err := c.TLS.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := c.Logs.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := c.Metrics.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := c.Traces.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	if err := c.Auth.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}
	return errs
}
