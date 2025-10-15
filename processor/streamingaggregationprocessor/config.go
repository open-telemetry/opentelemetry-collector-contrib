// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
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

// AggregationType defines the aggregation strategy for gauge metrics
type AggregationType string

const (
	// Last keeps the most recent value (default for gauges)
	Last AggregationType = "last"
	// Average calculates mean of all values
	Average AggregationType = "average"
	// Sum adds all values together
	Sum AggregationType = "sum"
	// Max keeps the maximum value
	Max AggregationType = "max"
	// Min keeps the minimum value
	Min AggregationType = "min"
)

// LabelType defines how to handle labels
type LabelType string

const (
	// Keep preserves specified labels and removes others
	Keep LabelType = "keep"
	// Remove removes specified labels and keeps others
	Remove LabelType = "remove"
	// DropAll removes all labels (default behavior)
	DropAll LabelType = "drop_all"
)

// LabelConfig defines label manipulation rules
type LabelConfig struct {
	// Type defines the label manipulation strategy
	Type LabelType `mapstructure:"type"`
	// Names lists the label names to keep/remove (depending on Type)
	Names []string `mapstructure:"names"`
}

// MetricConfig defines processing rules for specific metrics
type MetricConfig struct {
	// Match defines a regex pattern to match metric names
	// Required field - metrics matching this pattern will be processed
	Match string `mapstructure:"match"`

	// AggregateType defines aggregation strategy (currently only for gauge metrics)
	// Default: "last" for gauges
	AggregateType AggregationType `mapstructure:"aggregate_type"`

	// Labels defines label manipulation rules
	// Default: drop_all (removes all labels)
	Labels LabelConfig `mapstructure:"labels"`
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

		// Validate aggregation type
		if err := metricConfig.ValidateAggregationType(); err != nil {
			return fmt.Errorf("metrics[%d].aggregate_type: %w", i, err)
		}

		// Validate label configuration
		if err := metricConfig.ValidateLabelConfig(); err != nil {
			return fmt.Errorf("metrics[%d].labels: %w", i, err)
		}
	}

	return nil
}

// CreateDefaultConfig creates the default configuration for the processor
func CreateDefaultConfig() component.Config {
	return &Config{
		WindowSize:         30 * time.Second, // Same as realworld-v2
		MaxMemoryMB:        100,              // Reasonable default
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

	// Apply defaults to metric configurations
	for i := range cfg.Metrics {
		cfg.Metrics[i].applyDefaults()
	}
}

// ValidateAggregationType validates the aggregation type
func (mc *MetricConfig) ValidateAggregationType() error {
	if mc.AggregateType == "" {
		return nil // Empty is allowed, defaults to "last"
	}

	validTypes := []AggregationType{Last, Average, Sum, Max, Min}
	if !slices.Contains(validTypes, mc.AggregateType) {
		return fmt.Errorf("invalid aggregation type %q, must be one of: last, average, sum, max, min", mc.AggregateType)
	}

	return nil
}

// ValidateLabelConfig validates the label configuration
func (mc *MetricConfig) ValidateLabelConfig() error {
	if mc.Labels.Type == "" {
		return nil // Empty is allowed, defaults to "drop_all"
	}

	validTypes := []LabelType{Keep, Remove, DropAll}
	if !slices.Contains(validTypes, mc.Labels.Type) {
		return fmt.Errorf("invalid label type %q, must be one of: keep, remove, drop_all", mc.Labels.Type)
	}

	// If type is Keep or Remove, Names should not be empty
	if (mc.Labels.Type == Keep || mc.Labels.Type == Remove) && len(mc.Labels.Names) == 0 {
		return fmt.Errorf("label names cannot be empty when type is %q", mc.Labels.Type)
	}

	return nil
}

// applyDefaults applies default values to metric configuration
func (mc *MetricConfig) applyDefaults() {
	if mc.AggregateType == "" {
		mc.AggregateType = Last // Default aggregation type
	}

	if mc.Labels.Type == "" {
		mc.Labels.Type = DropAll // Default label behavior
	}
}

// getAggregateUtilType maps our AggregationType to aggregateutil.AggregationType
func (mc *MetricConfig) getAggregateUtilType() aggregateutil.AggregationType {
	switch mc.AggregateType {
	case Average:
		return aggregateutil.Mean
	case Sum:
		return aggregateutil.Sum
	case Max:
		return aggregateutil.Max
	case Min:
		return aggregateutil.Min
	default: // Last
		// Last doesn't have a direct mapping - we'll handle it specially
		return aggregateutil.Sum // Placeholder, won't be used for Last
	}
}
