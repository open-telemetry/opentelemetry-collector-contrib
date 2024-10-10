// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqmessagereciever // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqmessagereciever"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	Connection          ConnectionConfig          `mapstructure:"connection"`
	EncodingExtensionID *component.ID             `mapstructure:"encoding_extension"`
	RetrySettings       configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

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

type ConnectionConfig struct {
	Name              string                  `mapstructure:"name"`
	Endpoint          string                  `mapstructure:"endpoint"`
	VHost             string                  `mapstructure:"vhost"`
	TLSConfig         *configtls.ClientConfig `mapstructure:"tls"`
	Auth              AuthConfig              `mapstructure:"auth"`
	Queue             []QueueConfig           `mapstructure:"queue"`
	ConnectionTimeout time.Duration           `mapstructure:"connection_timeout"`
	Heartbeat         time.Duration           `mapstructure:"heartbeat"`
	EncodingExtension component.ID            `mapstructure:"encoding_extension"`
	Durable           bool                    `mapstructure:"durable"`
}

type AuthConfig struct {
	Plain PlainAuth `mapstructure:"plain"`
}

type PlainAuth struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type QueueConfig struct {
	Name      string `mapstructure:"name"`
	Consumer  string `mapstructure:"consumer"`
	Exclusive bool   `mapstructure:"exclusive"`
}
