// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
)

type Config struct {
	Connection          ConnectionConfig          `mapstructure:"connection"`
	Routing             RoutingConfig             `mapstructure:"routing"`
	MessageBodyEncoding string                    `mapstructure:"message_body_encoding"`
	Durable             bool                      `mapstructure:"durable"`
	RetrySettings       configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

type ConnectionConfig struct {
	Endpoint string     `mapstructure:"endpoint"`
	VHost    string     `mapstructure:"vhost"`
	Auth     AuthConfig `mapstructure:"auth"`
}

type RoutingConfig struct {
	RoutingKey string `mapstructure:"routing_key"`
}

type AuthConfig struct {
	SASL SASLConfig `mapstructure:"sasl"`
}

type SASLConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
