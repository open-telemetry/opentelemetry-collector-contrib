// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// config.go - CORRECTED VERSION with proper interface implementations

package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config represents the configuration for the isolation forest processor.
type Config struct {
	ForestSize              int               `mapstructure:"forest_size"`
	SubsampleSize           int               `mapstructure:"subsample_size"`
	ContaminationRate       float64           `mapstructure:"contamination_rate"`
	Mode                    string            `mapstructure:"mode"`
	Threshold               float64           `mapstructure:"threshold"`
	TrainingWindow          string            `mapstructure:"training_window"`
	UpdateFrequency         string            `mapstructure:"update_frequency"`
	MinSamples              int               `mapstructure:"min_samples"`
	ScoreAttribute          string            `mapstructure:"score_attribute"`
	ClassificationAttribute string            `mapstructure:"classification_attribute"`
	Features                FeatureConfig     `mapstructure:"features"`
	Models                  []ModelConfig     `mapstructure:"models"`
	Performance             PerformanceConfig `mapstructure:"performance"`

	// Adaptive window sizing configuration
	AdaptiveWindow *AdaptiveWindowConfig `mapstructure:"adaptive_window"`
}

// AdaptiveWindowConfig configures automatic window size adjustment based on traffic patterns
type AdaptiveWindowConfig struct {
	// Core configuration
	Enabled        bool    `mapstructure:"enabled"`         // Enable adaptive sizing
	MinWindowSize  int     `mapstructure:"min_window_size"` // Minimum samples to keep
	MaxWindowSize  int     `mapstructure:"max_window_size"` // Maximum samples (memory protection)
	MemoryLimitMB  int     `mapstructure:"memory_limit_mb"` // Auto-shrink when exceeded
	AdaptationRate float64 `mapstructure:"adaptation_rate"` // Adjustment speed (0.0-1.0)

	// Optional parameters with defaults
	VelocityThreshold      float64 `mapstructure:"velocity_threshold"`       // Grow when >N samples/sec
	StabilityCheckInterval string  `mapstructure:"stability_check_interval"` // Check model accuracy interval
}

type FeatureConfig struct {
	Traces  []string `mapstructure:"traces"`
	Metrics []string `mapstructure:"metrics"`
	Logs    []string `mapstructure:"logs"`
}

type ModelConfig struct {
	Name              string            `mapstructure:"name"`
	Selector          map[string]string `mapstructure:"selector"`
	Features          []string          `mapstructure:"features"`
	Threshold         float64           `mapstructure:"threshold"`
	ForestSize        int               `mapstructure:"forest_size"`
	SubsampleSize     int               `mapstructure:"subsample_size"`
	ContaminationRate float64           `mapstructure:"contamination_rate"`
}

type PerformanceConfig struct {
	MaxMemoryMB     int `mapstructure:"max_memory_mb"`
	BatchSize       int `mapstructure:"batch_size"`
	ParallelWorkers int `mapstructure:"parallel_workers"`
}

// createDefaultConfig returns a configuration with sensible defaults
// Note: This function returns component.Config to match the expected signature.
func createDefaultConfig() component.Config {
	return &Config{
		ForestSize:        100,
		SubsampleSize:     256,
		ContaminationRate: 0.1,
		Mode:              "enrich",
		Threshold:         0.7,
		TrainingWindow:    "24h",
		UpdateFrequency:   "1h",
		MinSamples:        1000,

		ScoreAttribute:          "anomaly.isolation_score",
		ClassificationAttribute: "anomaly.is_anomaly",

		Features: FeatureConfig{
			Traces:  []string{"duration", "error", "http.status_code"},
			Metrics: []string{"value", "rate_of_change"},
			Logs:    []string{"severity_number", "timestamp_gap"},
		},

		Performance: PerformanceConfig{
			MaxMemoryMB:     512,
			BatchSize:       1000,
			ParallelWorkers: 4,
		},

		// Default adaptive window configuration (disabled by default for backward compatibility)
		AdaptiveWindow: &AdaptiveWindowConfig{
			Enabled:                false,  // Disabled by default - backward compatibility
			MinWindowSize:          1000,   // Match MinSamples for consistency
			MaxWindowSize:          100000, // Reasonable upper bound
			MemoryLimitMB:          256,    // Half of total processor memory
			AdaptationRate:         0.1,    // Conservative adjustment speed
			VelocityThreshold:      50,     // Default growth threshold
			StabilityCheckInterval: "5m",   // Check model stability every 5 minutes
		},
	}
}

// Validate checks the configuration for logical consistency and valid parameter ranges.
func (cfg *Config) Validate() error {
	if cfg.ForestSize <= 0 {
		return errors.New("forest_size must be positive")
	}
	// Upper bound required by tests
	if cfg.ForestSize > 1000 {
		return errors.New("forest_size should not exceed 1000")
	}

	if cfg.ContaminationRate < 0.0 || cfg.ContaminationRate > 1.0 {
		return errors.New("contamination_rate must be between 0.0 and 1.0")
	}
	if cfg.Mode != "enrich" && cfg.Mode != "filter" && cfg.Mode != "both" {
		return errors.New("mode must be 'enrich', 'filter', or 'both'")
	}
	if cfg.Threshold < 0.0 || cfg.Threshold > 1.0 {
		return errors.New("threshold must be between 0.0 and 1.0")
	}

	if _, err := time.ParseDuration(cfg.TrainingWindow); err != nil {
		return fmt.Errorf("training_window is not a valid duration: %w", err)
	}
	if _, err := time.ParseDuration(cfg.UpdateFrequency); err != nil {
		return fmt.Errorf("update_frequency is not a valid duration: %w", err)
	}

	if cfg.ScoreAttribute == cfg.ClassificationAttribute {
		return errors.New("score_attribute and classification_attribute must be different")
	}

	// Require at least one feature type configured
	if len(cfg.Features.Traces) == 0 && len(cfg.Features.Metrics) == 0 && len(cfg.Features.Logs) == 0 {
		return errors.New("at least one feature type must be configured")
	}

	// Validate adaptive window configuration
	if cfg.AdaptiveWindow != nil {
		if err := cfg.validateAdaptiveWindow(); err != nil {
			return fmt.Errorf("adaptive_window validation failed: %w", err)
		}
	}

	return nil
}

// validateAdaptiveWindow validates the adaptive window configuration
func (cfg *Config) validateAdaptiveWindow() error {
	aw := cfg.AdaptiveWindow

	if aw.MinWindowSize <= 0 {
		return errors.New("min_window_size must be positive")
	}

	if aw.MaxWindowSize <= aw.MinWindowSize {
		return errors.New("max_window_size must be greater than min_window_size")
	}

	// Ensure consistency with main config
	if aw.MinWindowSize < cfg.MinSamples {
		return fmt.Errorf("adaptive_window.min_window_size (%d) should be >= min_samples (%d) for consistency",
			aw.MinWindowSize, cfg.MinSamples)
	}

	if aw.MemoryLimitMB <= 0 {
		return errors.New("memory_limit_mb must be positive")
	}

	// Memory limit should be reasonable compared to total processor memory
	if aw.MemoryLimitMB > cfg.Performance.MaxMemoryMB {
		return fmt.Errorf("adaptive_window.memory_limit_mb (%d) should not exceed performance.max_memory_mb (%d)",
			aw.MemoryLimitMB, cfg.Performance.MaxMemoryMB)
	}

	if aw.AdaptationRate < 0.0 || aw.AdaptationRate > 1.0 {
		return errors.New("adaptation_rate must be between 0.0 and 1.0")
	}

	if aw.VelocityThreshold < 0 {
		return errors.New("velocity_threshold must be non-negative")
	}

	if aw.StabilityCheckInterval != "" {
		if _, err := time.ParseDuration(aw.StabilityCheckInterval); err != nil {
			return fmt.Errorf("stability_check_interval is not a valid duration: %w", err)
		}
	}

	return nil
}

// IsAdaptiveWindowEnabled returns true if adaptive window sizing is enabled
func (cfg *Config) IsAdaptiveWindowEnabled() bool {
	return cfg.AdaptiveWindow != nil && cfg.AdaptiveWindow.Enabled
}

// GetStabilityCheckInterval returns the stability check interval duration
func (cfg *Config) GetStabilityCheckInterval() (time.Duration, error) {
	if cfg.AdaptiveWindow == nil || cfg.AdaptiveWindow.StabilityCheckInterval == "" {
		return 5 * time.Minute, nil // Default
	}
	return time.ParseDuration(cfg.AdaptiveWindow.StabilityCheckInterval)
}

func (cfg *Config) GetTrainingWindowDuration() (time.Duration, error) {
	return time.ParseDuration(cfg.TrainingWindow)
}

func (cfg *Config) GetUpdateFrequencyDuration() (time.Duration, error) {
	return time.ParseDuration(cfg.UpdateFrequency)
}

func (cfg *Config) IsMultiModelMode() bool {
	return len(cfg.Models) > 0
}

func (cfg *Config) GetModelForAttributes(attributes map[string]any) *ModelConfig {
	if !cfg.IsMultiModelMode() {
		return nil
	}
	for _, model := range cfg.Models {
		matches := true
		for key, expectedValue := range model.Selector {
			actualValue, exists := attributes[key]
			if !exists {
				matches = false
				break
			}
			if fmt.Sprintf("%v", actualValue) != expectedValue {
				matches = false
				break
			}
		}
		if matches {
			return &model
		}
	}
	return nil
}
