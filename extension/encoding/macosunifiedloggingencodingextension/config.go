// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"errors"
)

// Config defines the configuration for the macOS Unified Logging encoding extension
type Config struct {
	// ParsePrivateLogs determines whether to attempt parsing of private/masked log data
	ParsePrivateLogs bool `mapstructure:"parse_private_logs"`

	// IncludeSignpostEvents determines whether to include signpost events in parsing
	IncludeSignpostEvents bool `mapstructure:"include_signpost_events"`

	// IncludeActivityEvents determines whether to include activity events in parsing
	IncludeActivityEvents bool `mapstructure:"include_activity_events"`

	// MaxLogSize sets the maximum size for individual log entries in bytes
	MaxLogSize int `mapstructure:"max_log_size"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if c.MaxLogSize <= 0 {
		return errors.New("max_log_size must be greater than 0")
	}
	if c.MaxLogSize > 1024*1024 { // 1MB limit
		return errors.New("max_log_size cannot exceed 1MB")
	}
	return nil
}
