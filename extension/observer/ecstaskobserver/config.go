// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecstaskobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver"

import (
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
)

const (
	defaultRefreshInterval = 30 * time.Second
	defaultPortLabel       = "ECS_TASK_OBSERVER_PORT"
)

type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`

	// RefreshInterval determines the frequency at which the observer
	// needs to poll for collecting new information about task containers.
	RefreshInterval time.Duration `mapstructure:"refresh_interval" yaml:"refresh_interval"`

	// PortLabels is a list of container Docker labels from which to obtain the observed Endpoint port.
	// The first label with valid port found will be used.  If no PortLabels provided, default of
	// ECS_TASK_OBSERVER_PORT will be used.
	PortLabels []string `mapstructure:"port_labels" yaml:"port_labels"`
}

func (c Config) Validate() error {
	if c.Endpoint != "" {
		if _, err := url.Parse(c.Endpoint); err != nil {
			return fmt.Errorf("failed to parse ecs task metadata endpoint %q: %w", c.Endpoint, err)
		}
	}
	return nil
}

func defaultConfig() Config {
	return Config{
		RefreshInterval: defaultRefreshInterval,
		PortLabels:      []string{defaultPortLabel},
	}
}
