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

// UserInfoConfig defines the configuration for username/password auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserInfo
type UserInfoConfig struct {
	// User is the username to use for username/password auth.
	User string `mapstructure:"user"`
	// Password is the password to use for username/password auth.
	Password string `mapstructure:"password"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// NKeyConfig defines the configuration for NKey auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#Nkey
type NKeyConfig struct {
	// PublicKey is the public key to use for NKey auth.
	PublicKey string `mapstructure:"public_key"`
	// Seed is the seed to use for NKey auth.
	Seed string `mapstructure:"seed"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// UserJWTConfig defines the configuration for NKey auth via JWT.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserJWT
type UserJWTConfig struct {
	// JWT is the JWT to use for NKey auth via JWT.
	JWT string `mapstructure:"jwt"`
	// Seed is the seed to use for NKey auth via JWT.
	Seed string `mapstructure:"seed"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// UserCredentialsConfig defines the configuration for NKey auth via credentials file.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserCredentials
type UserCredentialsConfig struct {
	// UserFile is the path to the user credentials file to use for NKey auth via credentials file.
	UserFile string `mapstructure:"user_file"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// AuthConfig defines the auth configuration for the NATS client.
//
// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro
type AuthConfig struct {
	// Token holds the configuration for token auth.
	Token *TokenConfig `mapstructure:"token"`

	// UserInfo holds the configuration for username/password auth.
	UserInfo *UserInfoConfig `mapstructure:"user_info"`

	// NKey holds the configuration for NKey auth.
	NKey *NKeyConfig `mapstructure:"nkey"`

	// UserJWT holds the configuration for JWT auth.
	UserJWT *UserJWTConfig `mapstructure:"user_jwt"`

	// UserCredentials holds the configuration for credentials file auth.
	UserCredentials *UserCredentialsConfig `mapstructure:"user_credentials"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// Config defines the configuration for the NATS core exporter.
type Config struct {
	// Endpoint is the NATS server URL.
	Endpoint string `mapstructure:"endpoint"`

	// Pedantic enables/disables NATS pedantic mode.
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
		errs = multierr.Append(errs, errors.New("built-in marshaler and encoding extension configured simultaneously"))
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
		return errors.New("incomplete token configuration")
	}
	return nil
}

func (c *UserInfoConfig) Validate() error {
	if c.User == "" || c.Password == "" {
		return errors.New("incomplete user_info configuration")
	}
	return nil
}

func (c *NKeyConfig) Validate() error {
	if c.PublicKey == "" || c.Seed == "" {
		return errors.New("incomplete nkey configuration")
	}
	return nil
}

func (c *UserJWTConfig) Validate() error {
	if c.JWT == "" || c.Seed == "" {
		return errors.New("incomplete user_jwt configuration")
	}
	return nil
}

func (c *UserCredentialsConfig) Validate() error {
	if c.UserFile == "" {
		return errors.New("incomplete user_credentials configuration")
	}
	return nil
}

func (c *AuthConfig) Validate() error {
	var errs error
	authMethodsCount := 0
	if c.Token != nil {
		authMethodsCount++
		if err := c.Token.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.UserInfo != nil {
		authMethodsCount++
		if err := c.UserInfo.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.NKey != nil {
		authMethodsCount++
		if err := c.NKey.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.UserJWT != nil {
		authMethodsCount++
		if err := c.UserJWT.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if c.UserCredentials != nil {
		authMethodsCount++
		if err := c.UserCredentials.Validate(); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	if authMethodsCount > 1 {
		errs = multierr.Append(errs, errors.New("multiple auth methods configured simultaneously"))
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
