// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver"

import (
	"errors"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// Endpoint identifies the expected encoding of messages
	// received from Pub/Sub.
	Encoding component.ID `mapstructure:"encoding"`
}

const defaultEndpoint = "0.0.0.0:8080"

// createDefaultConfig creates the default configuration for the receiver.
func createDefaultConfig() component.Config {
	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.Endpoint = defaultEndpoint
	serverConfig.TLS = configoptional.None[configtls.ServerConfig]()

	return &Config{
		ServerConfig: serverConfig,
	}
}

// Validate checks if the receiver configuration is valid.
func (c *Config) Validate() error {
	var errs []error

	if c.Encoding == (component.ID{}) {
		errs = append(errs, errors.New("encoding must be set"))
	}

	_, _, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		errs = append(errs, fmt.Errorf("misformatted endpoint: %w", err))
	}

	return errors.Join(errs...)
}
