// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cgroupruntimeextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension"

import "errors" // Config contains the configuration for the cgroup runtime extension.
type Config struct {
	GoMaxProcs GoMaxProcsConfig `mapstructure:"gomaxprocs"`
	GoMemLimit GoMemLimitConfig `mapstructure:"gomemlimit"`
}

type GoMaxProcsConfig struct {
	Enabled bool `mapstructure:"enable"`
}

type GoMemLimitConfig struct {
	Enabled bool    `mapstructure:"enable"`
	Ratio   float64 `mapstructure:"ratio"`
}

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.GoMemLimit.Ratio < 0 || cfg.GoMemLimit.Ratio > 1 {
		return errors.New("gomemlimit ratio must be between 0 and 1")
	}
	return nil
}
