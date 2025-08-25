// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"errors"
	"fmt"
	"text/template"

	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
)

type MarshalerType string

const (
	OtlpProtoMarshaler MarshalerType = "otlp_proto"
	OtlpJsonMarshaler  MarshalerType = "otlp_json"
	LogBodyMarshaler   MarshalerType = "log_body"
)

// SignalConfig defines the configuration for a signal type.
type SignalConfig struct {
	// Subject is the `http/template` template string used to construct the NATS
	// subject.
	//
	// See: https://pkg.go.dev/text/template
	//
	// See: https://docs.nats.io/nats-concepts/subjects#subject-based-filtering-and-security
	Subject string `mapstructure:"subject"`

	// Marshaler is the name of the marshaler to use when marshaling the signal
	// type.
	//
	// Supported marshalers:
	//  - otlp_proto
	//  - otlp_json
	//  - log_body (only supported for logs)
	Marshaler MarshalerType `mapstructure:"marshaler"`
	// Encoder is the name of the encoding extension to use when marshaling the
	// signal type.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding
	Encoder string `mapstructure:"encoder"`

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
	Token string `mapstructure:"token"`
}

// UserInfoConfig defines the configuration for username/password auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserInfo
type UserInfoConfig struct {
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// NKeyConfig defines the configuration for NKey auth.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#Nkey
type NKeyConfig struct {
	PubKey string `mapstructure:"pub_key"`
	SigKey string `mapstructure:"sig_key"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// UserJWTConfig defines the configuration for NKey auth via JWT.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserJWT
type UserJWTConfig struct {
	JWT    string `mapstructure:"jwt"`
	SigKey string `mapstructure:"sig_key"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

// UserCredentialsConfig defines the configuration for NKey auth via credentials file.
//
// See: https://pkg.go.dev/github.com/nats-io/nats.go#UserCredentials
type UserCredentialsConfig struct {
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

	if c.Subject != "" {
		if _, err := template.New("subject").Parse(c.Subject); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error parsing subject: %v", err))
		}
	}

	if c.Marshaler != "" && c.Encoder != "" {
		errs = multierr.Append(errs, errors.New("marshaler and encoder cannot be configured simultaneously"))
	}
	if c.Marshaler != "" {
		if c.Marshaler != OtlpProtoMarshaler && c.Marshaler != OtlpJsonMarshaler && c.Marshaler != LogBodyMarshaler {
			errs = multierr.Append(errs, fmt.Errorf("unsupported marshaler: %s", c.Marshaler))
		}
	}

	return errs
}

func (c *LogsConfig) Validate() error {
	return (*SignalConfig)(c).Validate()
}

func (c *MetricsConfig) Validate() error {
	errs := (*SignalConfig)(c).Validate()

	if c.Marshaler == LogBodyMarshaler {
		errs = multierr.Append(errs, fmt.Errorf("unsupported marshaler for metrics: %s", c.Marshaler))
	}

	return errs
}

func (c *TracesConfig) Validate() error {
	errs := (*SignalConfig)(c).Validate()

	if c.Marshaler == LogBodyMarshaler {
		errs = multierr.Append(errs, fmt.Errorf("unsupported marshaler for traces: %s", c.Marshaler))
	}

	return errs
}

func (c *TokenConfig) Validate() error {
	if c == nil || c.Token != "" {
		return nil
	}

	return errors.New("incomplete token configuration")
}

func (c *UserInfoConfig) Validate() error {
	if c == nil || (c.User != "" && c.Password != "") {
		return nil
	}

	return errors.New("incomplete user_info configuration")
}

func (c *NKeyConfig) Validate() error {
	if c == nil || (c.PubKey != "" && c.SigKey != "") {
		return nil
	}

	return errors.New("incomplete nkey configuration")
}

func (c *UserJWTConfig) Validate() error {
	if c == nil || (c.JWT != "" && c.SigKey != "") {
		return nil
	}

	return errors.New("incomplete user_jwt configuration")
}

func (c *UserCredentialsConfig) Validate() error {
	if c == nil || c.UserFile != "" {
		return nil
	}

	return errors.New("incomplete user_credentials configuration")
}

func (c *AuthConfig) Validate() error {
	var errs error

	if err := c.Token.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}

	if err := c.UserInfo.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}

	if err := c.NKey.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}

	if err := c.UserJWT.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}

	if err := c.UserCredentials.Validate(); err != nil {
		errs = multierr.Append(errs, err)
	}

	if c.NKey != nil && (c.UserJWT != nil || c.UserCredentials != nil) {
		errs = multierr.Append(errs, errors.New("nkey and user_jwt/user_credentials cannot be configured simultaneously"))
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
