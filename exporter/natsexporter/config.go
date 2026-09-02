// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
)

const (
	// marshalerOTLPProto encodes payloads as OTLP protobuf.
	marshalerOTLPProto = "otlp_proto"
	// marshalerOTLPJSON encodes payloads as OTLP JSON.
	marshalerOTLPJSON = "otlp_json"
)

// SignalConfig defines the configuration for a single signal type (logs,
// metrics, or traces).
type SignalConfig struct {
	// Subject is the OTTL value expression used to construct the NATS subject
	// the signal is published to.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md
	// See: https://docs.nats.io/nats-concepts/subjects
	Subject string `mapstructure:"subject"`

	// Marshaler selects a built-in marshaler for outgoing payloads. Mutually
	// exclusive with EncodingExtension.
	//
	// Supported marshalers:
	//   - otlp_proto (default)
	//   - otlp_json
	Marshaler string `mapstructure:"marshaler"`

	// EncodingExtension is the component ID of an encoding extension used to
	// marshal outgoing payloads. Mutually exclusive with Marshaler.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding
	EncodingExtension string `mapstructure:"encoding_extension"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// JetStreamConfig configures publishing via NATS JetStream (durable,
// acknowledged delivery) instead of core NATS. When present, every exported
// payload is published with JetStream and the publish blocks until the server
// acknowledges persistence.
//
// A stream whose subjects capture the configured signal subjects must already
// exist on the server; this exporter does not create or manage streams.
//
// See: https://docs.nats.io/nats-concepts/jetstream
type JetStreamConfig struct {
	// Domain optionally selects a JetStream domain, e.g. when publishing through
	// a leaf node to a hub. Empty uses the server's default domain.
	Domain string `mapstructure:"domain"`

	// PublishTimeout bounds how long to wait for each publish acknowledgement.
	// Zero means no exporter-imposed deadline (the surrounding context still
	// applies).
	PublishTimeout time.Duration `mapstructure:"publish_timeout"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// TokenConfig defines the configuration for token auth.
type TokenConfig struct {
	Token string `mapstructure:"token"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// UserConfig defines the configuration for username/password auth.
type UserConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`

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
	JWT  string `mapstructure:"jwt"`
	Seed []byte `mapstructure:"seed"`

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

// Config defines the configuration for the NATS exporter.
type Config struct {
	// Endpoint is the NATS server URL.
	Endpoint string `mapstructure:"endpoint"`

	// Pedantic enables/disables NATS pedantic mode.
	Pedantic bool `mapstructure:"pedantic"`

	// TLS holds the TLS configuration for the NATS client.
	TLS configtls.ClientConfig `mapstructure:"tls"`

	// JetStream, when set, publishes via NATS JetStream (durable, acknowledged
	// delivery) instead of core NATS.
	JetStream *JetStreamConfig `mapstructure:"jetstream"`

	// Logs holds the configuration for the logs signal.
	Logs SignalConfig `mapstructure:"logs"`
	// Metrics holds the configuration for the metrics signal.
	Metrics SignalConfig `mapstructure:"metrics"`
	// Traces holds the configuration for the traces signal.
	Traces SignalConfig `mapstructure:"traces"`

	// Auth holds the configuration for NATS auth.
	Auth AuthConfig `mapstructure:",squash"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

func (c *SignalConfig) Validate() error {
	if c.Marshaler != "" && c.EncodingExtension != "" {
		return errors.New("marshaler configured more than once")
	}
	if c.Marshaler != "" {
		switch c.Marshaler {
		case marshalerOTLPProto, marshalerOTLPJSON:
		default:
			return fmt.Errorf("unsupported marshaler: %q", c.Marshaler)
		}
	}
	if c.EncodingExtension != "" {
		var id component.ID
		if err := id.UnmarshalText([]byte(c.EncodingExtension)); err != nil {
			return fmt.Errorf("failed to parse encoding extension name: %w", err)
		}
	}
	return nil
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
		errs = multierr.Append(errs, c.Token.Validate())
	}
	if c.User != nil {
		errs = multierr.Append(errs, c.User.Validate())
	}
	if c.Nkey != nil {
		errs = multierr.Append(errs, c.Nkey.Validate())
	}
	if c.NkeyJWT != nil {
		errs = multierr.Append(errs, c.NkeyJWT.Validate())
	}
	if c.NkeyUserFile != nil {
		errs = multierr.Append(errs, c.NkeyUserFile.Validate())
	}

	nkeyConfigured := 0
	for _, isSet := range []bool{c.Nkey != nil, c.NkeyJWT != nil, c.NkeyUserFile != nil} {
		if isSet {
			nkeyConfigured++
		}
	}
	if nkeyConfigured > 1 {
		errs = multierr.Append(errs, errors.New("NKey auth configured more than once"))
	}
	return errs
}

func (c *JetStreamConfig) Validate() error {
	if c.PublishTimeout < 0 {
		return errors.New("jetstream publish_timeout must not be negative")
	}
	return nil
}

func (c *Config) Validate() error {
	var errs error
	errs = multierr.Append(errs, c.TLS.Validate())
	errs = multierr.Append(errs, c.Logs.Validate())
	errs = multierr.Append(errs, c.Metrics.Validate())
	errs = multierr.Append(errs, c.Traces.Validate())
	errs = multierr.Append(errs, c.Auth.Validate())
	if c.JetStream != nil {
		errs = multierr.Append(errs, c.JetStream.Validate())
	}
	return errs
}
