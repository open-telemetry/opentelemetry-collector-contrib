// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

// Config defines the configuration for the macOS Unified Logging encoding extension.
// This extension is designed to decode macOS Unified Logging binary files.
type Config struct {
	// DebugMode enables additional debug information during decoding
	DebugMode bool `mapstructure:"debug_mode"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// No validation needed for basic config
	return nil
}
