// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
)

type Config struct {
	Connection          ConnectionConfig          `mapstructure:"connection"`
	Topic               TopicConfig               `mapstructure:"topic"`
	EncodingExtensionID *component.ID             `mapstructure:"encoding_extension"`
	QoS                 byte                      `mapstructure:"qos"`
	Retain              bool                      `mapstructure:"retain"`
	RetrySettings       configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

type ConnectionConfig struct {
	Endpoint                   string                  `mapstructure:"endpoint"`
	TLSConfig                  *configtls.ClientConfig `mapstructure:"tls"`
	Auth                       AuthConfig              `mapstructure:"auth"`
	ConnectionTimeout          time.Duration           `mapstructure:"connection_timeout"`
	KeepAlive                  time.Duration           `mapstructure:"keep_alive"`
	PublishConfirmationTimeout time.Duration           `mapstructure:"publish_confirmation_timeout"`
	ClientID                   string                  `mapstructure:"client_id"`
}

type TopicConfig struct {
	Topic string `mapstructure:"topic"`
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
