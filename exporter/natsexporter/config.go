// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var (
	errNoURLs       = errors.New("nats: at least one url is required")
	errMultipleAuth = errors.New("nats: only one authentication method may be configured")
	errNKeySeedFile = errors.New("nats: auth.nkey.seed_file is required when nkey authentication is configured")
	errCredsFile    = errors.New("nats: auth.credentials.file is required when credentials authentication is configured")
)

// Config defines the configuration for the NATS exporter.
type Config struct {
	// TimeoutSettings configures the timeout applied to each publish. Squashed
	// so `timeout` sits at the top level of the exporter config.
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"`
	// QueueSettings configures the sending queue and batching behavior.
	QueueSettings configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	// BackOffConfig configures the retry-on-failure behavior.
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	// URLs is the list of NATS server addresses to connect to (a single URL,
	// or several for a cluster), e.g. ["nats://localhost:4222"].
	URLs []string `mapstructure:"urls"`
	// Name is an optional connection name reported to the NATS server.
	Name string `mapstructure:"name"`
	// Connection controls dial timeout and reconnect behavior.
	Connection ConnectionConfig `mapstructure:"connection"`
	// TLS configures a TLS client connection. When absent the connection is
	// established without TLS (unless the URL scheme requires it).
	TLS configoptional.Optional[configtls.ClientConfig] `mapstructure:"tls"`
	// Auth configures client authentication.
	Auth Authentication `mapstructure:"auth"`
	// JetStream, when present, publishes via JetStream instead of core NATS.
	JetStream configoptional.Optional[JetStreamConfig] `mapstructure:"jetstream"`

	// Traces configures the traces signal (subject and encoding).
	Traces SignalConfig `mapstructure:"traces"`
	// Metrics configures the metrics signal (subject and encoding).
	Metrics SignalConfig `mapstructure:"metrics"`
	// Logs configures the logs signal (subject and encoding).
	Logs SignalConfig `mapstructure:"logs"`
}

// SignalConfig is the per-signal configuration.
type SignalConfig struct {
	// Subject is the NATS subject messages are published to.
	Subject string `mapstructure:"subject"`
	// Encoding selects the payload encoding. Built-in values are "otlp_proto"
	// and "otlp_json"; any other value is treated as the component ID of a
	// registered encoding extension.
	Encoding string `mapstructure:"encoding"`
}

// ConnectionConfig controls how the client connects and reconnects. The
// reconnect behavior is delegated to the NATS client itself.
type ConnectionConfig struct {
	// Timeout is the dial timeout for establishing a connection.
	Timeout time.Duration `mapstructure:"timeout"`
	// MaxReconnects is the maximum number of reconnect attempts (-1 = infinite).
	MaxReconnects int `mapstructure:"max_reconnects"`
	// ReconnectWait is the delay between reconnect attempts.
	ReconnectWait time.Duration `mapstructure:"reconnect_wait"`
}

// Authentication holds the (mutually exclusive) authentication mechanisms.
type Authentication struct {
	// Token authenticates with a bearer token.
	Token configoptional.Optional[TokenAuth] `mapstructure:"token"`
	// UserPassword authenticates with a username and password.
	UserPassword configoptional.Optional[UserPasswordAuth] `mapstructure:"user_password"`
	// NKey authenticates with an NKey seed file.
	NKey configoptional.Optional[NKeyAuth] `mapstructure:"nkey"`
	// Credentials authenticates with a NATS credentials (.creds) file.
	Credentials configoptional.Optional[CredentialsAuth] `mapstructure:"credentials"`
}

// TokenAuth configures token authentication.
type TokenAuth struct {
	Token configopaque.String `mapstructure:"token"`
}

// UserPasswordAuth configures username/password authentication.
type UserPasswordAuth struct {
	Username string              `mapstructure:"username"`
	Password configopaque.String `mapstructure:"password"`
}

// NKeyAuth configures NKey authentication.
type NKeyAuth struct {
	SeedFile string `mapstructure:"seed_file"`
}

// CredentialsAuth configures credentials-file authentication.
type CredentialsAuth struct {
	File string `mapstructure:"file"`
}

// JetStreamConfig configures JetStream publishing.
type JetStreamConfig struct {
	// PublishTimeout bounds each JetStream publish (default 5s).
	PublishTimeout time.Duration `mapstructure:"publish_timeout"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the configuration and fails fast on missing/invalid fields.
func (c *Config) Validate() error {
	if len(c.URLs) == 0 {
		return errNoURLs
	}

	var configured int
	if c.Auth.Token.HasValue() {
		configured++
	}
	if c.Auth.UserPassword.HasValue() {
		configured++
	}
	if c.Auth.NKey.HasValue() {
		configured++
	}
	if c.Auth.Credentials.HasValue() {
		configured++
	}
	if configured > 1 {
		return errMultipleAuth
	}

	if c.Auth.NKey.HasValue() {
		nkey := c.Auth.NKey.Get()
		if nkey.SeedFile == "" {
			return errNKeySeedFile
		}
	}
	if c.Auth.Credentials.HasValue() {
		creds := c.Auth.Credentials.Get()
		if creds.File == "" {
			return errCredsFile
		}
	}

	return nil
}
