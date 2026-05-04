// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import (
	"errors" // Config contains the configuration for the cgroup runtime extension.
	"time"
)

type Config struct {
	GoMaxProcs GoMaxProcsConfig `mapstructure:"gomaxprocs"`
	GoMemLimit GoMemLimitConfig `mapstructure:"gomemlimit"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type GoMaxProcsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type GoMemLimitConfig struct {
	Enabled bool    `mapstructure:"enabled"`
	Ratio   float64 `mapstructure:"ratio"`
	// RefreshInterval configures the refresh interval for GOMEMLIMIT.
	// Setting the duration to 0 (default) disables periodic auto update.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.GoMemLimit.Ratio <= 0 || cfg.GoMemLimit.Ratio > 1 {
		return errors.New("gomemlimit ratio must be in the (0.0,1.0] range")
	}
	if cfg.GoMemLimit.RefreshInterval < 0 {
		return errors.New("refresh_interval: requires non negative value")
	}
	return nil
}
