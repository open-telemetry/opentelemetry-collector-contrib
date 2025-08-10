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

	return nil
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

func (cfg *Config) GetModelForAttributes(attributes map[string]interface{}) *ModelConfig {
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
