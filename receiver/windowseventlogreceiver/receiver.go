// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowseventlogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver"

import (
	"fmt"
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

// WindowsLogConfig defines configuration for the Windows Event Log receiver.
type WindowsLogConfig struct {
	InputConfig        windows.Config `mapstructure:",squash"`
	adapter.BaseConfig `mapstructure:",squash"`

	// ResolveSIDs contains configuration for SID-to-username resolution
	ResolveSIDs ResolveSIDsConfig `mapstructure:"resolve_sids"`

	// DiscoverDomainControllers controls whether to attempt auto-discovery of domain controllers for joined machines with remote credentials
	DiscoverDomainControllers bool `mapstructure:"discover_domain_controllers"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// ResolveSIDsConfig contains configuration for SID resolution
type ResolveSIDsConfig struct {
	// Enabled controls whether SID resolution is active
	Enabled bool `mapstructure:"enabled"`

	// CacheSize is the maximum number of SIDs to cache (LRU eviction)
	// Default: 10000
	CacheSize uint `mapstructure:"cache_size"`

	// CacheTTL is how long cache entries remain valid
	// Default: 15m
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

// Validate checks if the configuration is valid
func (c *ResolveSIDsConfig) Validate() error {
	if c.CacheTTL < 0 {
		return fmt.Errorf("cache_ttl must not be negative, got: %s", c.CacheTTL)
	}
	return nil
}
