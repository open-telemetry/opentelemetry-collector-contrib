// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config holds common fields and embedded protocol-specific configurations
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
