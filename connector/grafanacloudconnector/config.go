// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/confmap/xconfmap"
)

// Config defines the configuration options for the Grafana Cloud connector.
type Config struct {
	// HostIdentifiers defines the list of resource attributes used to derive
	// a unique `grafana.host.id` value. In most cases, this should be [ "host.id" ]
	HostIdentifiers      []string      `mapstructure:"host_identifiers"`
	MetricsFlushInterval time.Duration `mapstructure:"metrics_flush_interval"`
	// prevent unkeyed literal initialization
	_ struct{}
}

var _ xconfmap.Validator = (*Config)(nil)

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	if len(c.HostIdentifiers) == 0 {
		return errors.New("at least one host identifier is required")
	}

	if c.MetricsFlushInterval > 5*time.Minute || c.MetricsFlushInterval < 15*time.Second {
		return fmt.Errorf("%q is not a valid flush interval between 15s and 5m", c.MetricsFlushInterval)
	}

	return nil
}
