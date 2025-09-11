// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// WindowSize defines the size of the aggregation window
	// Default: 30s
	WindowSize time.Duration `mapstructure:"window_size"`

	// MaxMemoryMB defines the maximum memory usage in MB
	// Default: 100MB
	MaxMemoryMB int `mapstructure:"max_memory_mb"`

	// NumWindows defines the number of time windows to maintain
	// Default: 4 (to handle late arriving data)
	NumWindows int `mapstructure:"num_windows"`
	
	// StaleDataThreshold defines the threshold for detecting stale cumulative state
	// If no data is received for longer than this duration, cumulative state will be reset
	// Default: 5 minutes (to handle laptop sleep scenarios)
	StaleDataThreshold time.Duration `mapstructure:"stale_data_threshold"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	// All fields are optional with sensible defaults
	
	if cfg.WindowSize < 0 {
		return errors.New("window_size cannot be negative")
	}
	
	if cfg.MaxMemoryMB < 0 {
		return errors.New("max_memory_mb cannot be negative")
	}
	
	
	if cfg.NumWindows < 0 {
		return errors.New("num_windows cannot be negative")
	}
	
	if cfg.StaleDataThreshold < 0 {
		return errors.New("stale_data_threshold cannot be negative")
	}
	
	return nil
}

// CreateDefaultConfig creates the default configuration for the processor
func CreateDefaultConfig() component.Config {
	return &Config{
		WindowSize:         30 * time.Second,  // Same as realworld-v2
		MaxMemoryMB:        100,               // Reasonable default
		NumWindows:         4,                 // 4 windows for late data handling
		StaleDataThreshold: 5 * time.Minute,  // Reset state after 5 minutes of no data
	}
}

func (cfg *Config) applyDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 30 * time.Second
	}
	
	if cfg.MaxMemoryMB == 0 {
		cfg.MaxMemoryMB = 100
	}
	
	
	if cfg.NumWindows == 0 {
		cfg.NumWindows = 4
	}
	
	if cfg.StaleDataThreshold == 0 {
		cfg.StaleDataThreshold = 5 * time.Minute
	}
}
