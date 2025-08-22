// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"errors"
	"fmt"
	"slices"
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

// AuthConfig defines the auth configuration for the NATS client.
//
// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro
type AuthConfig struct {
	// Token is the plaintext token used for token auth.
	Token string `mapstructure:"token"`

	// Username is the plaintext username used for username/password auth.
	Username string `mapstructure:"username"`
	// Password is the plaintext password used for username/password auth.
	Password string `mapstructure:"password"`

	// NKey is the public key used for basic NKey auth.
	//
	// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/nkey_auth
	NKey string `mapstructure:"nkey"`
	// Seed is the private key used for NKey auth.
	//
	// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/nkey_auth
	Seed string `mapstructure:"seed"`
	// JWT is the JWT used for decentralized NKey auth via JWT.
	//
	// See: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/jwt
	JWT string `mapstructure:"jwt"`
	// Creds is the path to the credentials file used for decentralized NKey auth via credentials file.
	//
	// See: https://docs.nats.io/using-nats/developer/connecting/creds
	Creds string `mapstructure:"creds"`

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

	// Auth holds the configuration for the NATS auth.
	Auth AuthConfig `mapstructure:"auth"`

	// Prevent unkeyed literal initialization
	_ struct{}
}

func (c SignalConfig) Validate() error {
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

func (c LogsConfig) Validate() error {
	return SignalConfig(c).Validate()
}

func (c MetricsConfig) Validate() error {
	errs := SignalConfig(c).Validate()

	if c.Marshaler == LogBodyMarshaler {
		errs = multierr.Append(errs, fmt.Errorf("unsupported marshaler: %s", c.Marshaler))
	}

	return errs
}

func (c TracesConfig) Validate() error {
	errs := SignalConfig(c).Validate()

	if c.Marshaler == LogBodyMarshaler {
		errs = multierr.Append(errs, fmt.Errorf("unsupported marshaler: %s", c.Marshaler))
	}

	return errs
}

func (c AuthConfig) Validate() error {
	const (
		hasNone     = 0
		hasToken    = 1 << 0
		hasUsername = 1 << 1
		hasPassword = 1 << 2
		hasSeed     = 1 << 3
		hasNKey     = 1 << 4
		hasJWT      = 1 << 5
		hasCreds    = 1 << 6
	)

	bitmask := 0
	for bit, isSet := range map[int]bool{
		hasToken:    c.Token != "",
		hasUsername: c.Username != "",
		hasPassword: c.Password != "",
		hasSeed:     c.Seed != "",
		hasNKey:     c.NKey != "",
		hasJWT:      c.JWT != "",
		hasCreds:    c.Creds != "",
	} {
		if isSet {
			bitmask |= bit
		}
	}

	validBitmasks := []int{
		hasNone,
		hasToken,
		hasUsername | hasPassword,
		hasNKey | hasSeed,
		hasJWT | hasSeed,
		hasCreds,
	}
	if !slices.Contains(validBitmasks, bitmask) {
		return errors.New("invalid auth configuration")
	}

	return nil
}

func (c Config) Validate() error {
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
