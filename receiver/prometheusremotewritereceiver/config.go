// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config holds common fields and embedded protocol-specific configurations
type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"`
	// ResourceCacheSize is the maximum number of job/instance pairs whose resource
	// attributes (from target_info) are kept in the LRU cache across requests.
	// Default is 1000.
	ResourceCacheSize int `mapstructure:"resource_cache_size"`
	// prevent unkeyed literal initialization
	_ struct{}
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.ResourceCacheSize <= 0 {
		return fmt.Errorf("resource_cache_size must be greater than 0, got %d", cfg.ResourceCacheSize)
	}
	return nil
}
