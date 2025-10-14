// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"errors"
	"fmt"
	"regexp"
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

	// StaleDataThreshold defines the threshold for detecting stale cumulative state
	// If no data is received for longer than this duration, cumulative state will be reset
	// Default: 5 minutes (to handle laptop sleep scenarios)
	StaleDataThreshold time.Duration `mapstructure:"stale_data_threshold"`

	// Metrics defines metric-specific processing rules
	// If empty, all metrics are processed with default behavior
	Metrics []MetricConfig `mapstructure:"metrics"`

	// Note: NumWindows removed - double-buffer architecture always uses exactly 2 windows
}

// MetricConfig defines processing rules for specific metrics
type MetricConfig struct {
	// Match defines a regex pattern to match metric names
	// Required field - metrics matching this pattern will be processed
	Match string `mapstructure:"match"`

	// Future extensibility: aggregation methods, label handling, etc.
	// Aggregation []string `mapstructure:"aggregation"`
	// LabelSet    []LabelSetConfig `mapstructure:"label_set"`
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

	if cfg.StaleDataThreshold < 0 {
		return errors.New("stale_data_threshold cannot be negative")
	}

	// Validate metric configurations
	for i, metricConfig := range cfg.Metrics {
		if metricConfig.Match == "" {
			return fmt.Errorf("metrics[%d].match cannot be empty", i)
		}
		if _, err := regexp.Compile(metricConfig.Match); err != nil {
			return fmt.Errorf("metrics[%d].match is invalid regex: %w", i, err)
		}
	}

	return nil
}

// CreateDefaultConfig creates the default configuration for the processor
func CreateDefaultConfig() component.Config {
	return &Config{
		WindowSize:         30 * time.Second,  // Same as realworld-v2
		MaxMemoryMB:        100,               // Reasonable default
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
	
	if cfg.StaleDataThreshold == 0 {
		cfg.StaleDataThreshold = 5 * time.Minute
	}
}
