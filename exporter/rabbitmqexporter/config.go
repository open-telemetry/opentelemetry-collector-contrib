// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
)

type Config struct {
	Connection          ConnectionConfig          `mapstructure:"connection"`
	Routing             RoutingConfig             `mapstructure:"routing"`
	EncodingExtensionID *component.ID             `mapstructure:"encoding_extension"`
	Durable             bool                      `mapstructure:"durable"`
	RetrySettings       configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

type ConnectionConfig struct {
	Endpoint                   string                  `mapstructure:"endpoint"`
	VHost                      string                  `mapstructure:"vhost"`
	TLSConfig                  *configtls.ClientConfig `mapstructure:"tls"`
	Auth                       AuthConfig              `mapstructure:"auth"`
	ConnectionTimeout          time.Duration           `mapstructure:"connection_timeout"`
	Heartbeat                  time.Duration           `mapstructure:"heartbeat"`
	PublishConfirmationTimeout time.Duration           `mapstructure:"publish_confirmation_timeout"`
	Name                       string                  `mapstructure:"name"`
}

type RoutingConfig struct {
	Exchange   string `mapstructure:"exchange"`
	RoutingKey string `mapstructure:"routing_key"`
}

type AuthConfig struct {
	Plain PlainAuth `mapstructure:"plain"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type PlainAuth struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Connection.Endpoint == "" {
		return errors.New("connection.endpoint is required")
	}

	// Password-less users are possible so only validate username
	if cfg.Connection.Auth.Plain.Username == "" {
		return errors.New("connection.auth.plain.username is required")
	}

	return nil
}
