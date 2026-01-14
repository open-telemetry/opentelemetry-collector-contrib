// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"
)

// createDefaultConfig creates a config with type and version
func createDefaultConfig() component.Config {
	return &WindowsLogConfig{
		BaseConfig: adapter.BaseConfig{
			Operators:      []operator.Config{},
			RetryOnFailure: consumerretry.NewDefaultConfig(),
		},
		InputConfig: *windows.NewConfig(),
	}
}

// WindowsLogConfig defines configuration for the windowseventlog receiver
type WindowsLogConfig struct {
	InputConfig        windows.Config `mapstructure:",squash"`
	adapter.BaseConfig `mapstructure:",squash"`

	// ResolveSIDs contains configuration for SID-to-username resolution
	ResolveSIDs ResolveSIDsConfig `mapstructure:"resolve_sids"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ResolveSIDsConfig contains configuration for SID resolution
type ResolveSIDsConfig struct {
	// Enabled controls whether SID resolution is active
	Enabled bool `mapstructure:"enabled"`

	// CacheSize is the maximum number of SIDs to cache (LRU eviction)
	// Default: 10000
	CacheSize int `mapstructure:"cache_size"`

	// CacheTTL is how long cache entries remain valid
	// Default: 15m
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

// Validate checks if the configuration is valid
func (*ResolveSIDsConfig) Validate() error {
	// No validation needed - all fields have sensible defaults in the cache
	return nil
}
