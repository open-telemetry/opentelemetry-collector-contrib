// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// config_test.go - Comprehensive tests for configuration validation and parsing
package isolationforestprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	// Test that default configuration is valid and sensible
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Verify core algorithm parameters
	assert.Equal(t, 100, cfg.ForestSize, "Default forest size should be 100")
	assert.Equal(t, 256, cfg.SubsampleSize, "Default subsample size should be 256")
	assert.Equal(t, 0.1, cfg.ContaminationRate, "Default contamination rate should be 0.1")

	// Verify processing parameters
	assert.Equal(t, "enrich", cfg.Mode, "Default mode should be enrich")
	assert.Equal(t, 0.7, cfg.Threshold, "Default threshold should be 0.7")

	// Verify lifecycle parameters
	assert.Equal(t, "24h", cfg.TrainingWindow, "Default training window should be 24h")
	assert.Equal(t, "1h", cfg.UpdateFrequency, "Default update frequency should be 1h")
	assert.Equal(t, 1000, cfg.MinSamples, "Default min samples should be 1000")

	// Verify output configuration
	assert.Equal(t, "anomaly.isolation_score", cfg.ScoreAttribute)
	assert.Equal(t, "anomaly.is_anomaly", cfg.ClassificationAttribute)

	// Verify features configuration
	assert.Contains(t, cfg.Features.Traces, "duration")
	assert.Contains(t, cfg.Features.Traces, "error")
	assert.Contains(t, cfg.Features.Metrics, "value")
	assert.Contains(t, cfg.Features.Logs, "severity_number")

	// NEW: Verify adaptive window defaults
	require.NotNil(t, cfg.AdaptiveWindow, "Default config should have adaptive window configuration")
	assert.False(t, cfg.AdaptiveWindow.Enabled, "Adaptive window should be disabled by default")
	assert.Equal(t, 1000, cfg.AdaptiveWindow.MinWindowSize, "Default min window size should be 1000")
	assert.Equal(t, 100000, cfg.AdaptiveWindow.MaxWindowSize, "Default max window size should be 100000")
	assert.Equal(t, 256, cfg.AdaptiveWindow.MemoryLimitMB, "Default memory limit should be 256MB")
	assert.Equal(t, 0.1, cfg.AdaptiveWindow.AdaptationRate, "Default adaptation rate should be 0.1")
	assert.Equal(t, 50.0, cfg.AdaptiveWindow.VelocityThreshold, "Default velocity threshold should be 50")
	assert.Equal(t, "5m", cfg.AdaptiveWindow.StabilityCheckInterval, "Default stability check interval should be 5m")

	// NEW: Test adaptive window helper methods
	assert.False(t, cfg.IsAdaptiveWindowEnabled(), "IsAdaptiveWindowEnabled should return false by default")
	interval, err := cfg.GetStabilityCheckInterval()
	require.NoError(t, err, "GetStabilityCheckInterval should not error with default config")
	assert.Equal(t, 5*time.Minute, interval, "Default stability check interval should be 5 minutes")

	// Most importantly, verify the configuration validates
	err = cfg.Validate()
	require.NoError(t, err, "Default configuration should be valid")
}

func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name          string
		modifyConfig  func(*Config)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid configuration",
			modifyConfig: func(_ *Config) {
				// Use default config - should be valid
			},
			expectError: false,
		},
		{
			name:          "zero forest size",
			modifyConfig:  func(cfg *Config) { cfg.ForestSize = 0 },
			expectError:   true,
			errorContains: "forest_size must be positive",
		},
		{
			name:          "negative forest size",
			modifyConfig:  func(cfg *Config) { cfg.ForestSize = -1 },
			expectError:   true,
			errorContains: "forest_size must be positive",
		},
		{
			name:          "excessive forest size",
			modifyConfig:  func(cfg *Config) { cfg.ForestSize = 1500 },
			expectError:   true,
			errorContains: "forest_size should not exceed 1000",
		},
		{
			name:         "boundary forest size - maximum valid",
			modifyConfig: func(cfg *Config) { cfg.ForestSize = 1000 },
			expectError:  false,
		},
		{
			name:          "boundary forest size - minimum invalid",
			modifyConfig:  func(cfg *Config) { cfg.ForestSize = 1001 },
			expectError:   true,
			errorContains: "forest_size should not exceed 1000",
		},
		{
			name:         "contamination rate - minimum valid boundary",
			modifyConfig: func(cfg *Config) { cfg.ContaminationRate = 0.0 },
			expectError:  false,
		},
		{
			name:         "contamination rate - maximum valid boundary",
			modifyConfig: func(cfg *Config) { cfg.ContaminationRate = 1.0 },
			expectError:  false,
		},
		{
			name:          "invalid contamination rate - too high",
			modifyConfig:  func(cfg *Config) { cfg.ContaminationRate = 1.5 },
			expectError:   true,
			errorContains: "contamination_rate must be between 0.0 and 1.0",
		},
		{
			name:          "invalid contamination rate - negative",
			modifyConfig:  func(cfg *Config) { cfg.ContaminationRate = -0.1 },
			expectError:   true,
			errorContains: "contamination_rate must be between 0.0 and 1.0",
		},
		{
			name:         "threshold - minimum valid boundary",
			modifyConfig: func(cfg *Config) { cfg.Threshold = 0.0 },
			expectError:  false,
		},
		{
			name:         "threshold - maximum valid boundary",
			modifyConfig: func(cfg *Config) { cfg.Threshold = 1.0 },
			expectError:  false,
		},
		{
			name:          "invalid threshold - too high",
			modifyConfig:  func(cfg *Config) { cfg.Threshold = 1.5 },
			expectError:   true,
			errorContains: "threshold must be between 0.0 and 1.0",
		},
		{
			name:          "invalid threshold - negative",
			modifyConfig:  func(cfg *Config) { cfg.Threshold = -0.1 },
			expectError:   true,
			errorContains: "threshold must be between 0.0 and 1.0",
		},
		{
			name:         "valid mode - enrich",
			modifyConfig: func(cfg *Config) { cfg.Mode = "enrich" },
			expectError:  false,
		},
		{
			name:         "valid mode - filter",
			modifyConfig: func(cfg *Config) { cfg.Mode = "filter" },
			expectError:  false,
		},
		{
			name:         "valid mode - both",
			modifyConfig: func(cfg *Config) { cfg.Mode = "both" },
			expectError:  false,
		},
		{
			name:          "invalid mode",
			modifyConfig:  func(cfg *Config) { cfg.Mode = "invalid_mode" },
			expectError:   true,
			errorContains: "mode must be 'enrich', 'filter', or 'both'",
		},
		{
			name:          "invalid training window",
			modifyConfig:  func(cfg *Config) { cfg.TrainingWindow = "invalid_duration" },
			expectError:   true,
			errorContains: "training_window is not a valid duration",
		},
		{
			name:          "invalid update frequency",
			modifyConfig:  func(cfg *Config) { cfg.UpdateFrequency = "not_a_duration" },
			expectError:   true,
			errorContains: "update_frequency is not a valid duration",
		},
		{
			name: "duplicate attribute names",
			modifyConfig: func(cfg *Config) {
				cfg.ScoreAttribute = "same_name"
				cfg.ClassificationAttribute = "same_name"
			},
			expectError:   true,
			errorContains: "score_attribute and classification_attribute must be different",
		},
		{
			name: "empty features",
			modifyConfig: func(cfg *Config) {
				cfg.Features = FeatureConfig{
					Traces:  []string{},
					Metrics: []string{},
					Logs:    []string{},
				}
			},
			expectError:   true,
			errorContains: "at least one feature type must be configured",
		},
		{
			name: "features with only traces",
			modifyConfig: func(cfg *Config) {
				cfg.Features = FeatureConfig{
					Traces:  []string{"duration"},
					Metrics: []string{},
					Logs:    []string{},
				}
			},
			expectError: false,
		},
		{
			name: "features with only metrics",
			modifyConfig: func(cfg *Config) {
				cfg.Features = FeatureConfig{
					Traces:  []string{},
					Metrics: []string{"value"},
					Logs:    []string{},
				}
			},
			expectError: false,
		},
		{
			name: "features with only logs",
			modifyConfig: func(cfg *Config) {
				cfg.Features = FeatureConfig{
					Traces:  []string{},
					Metrics: []string{},
					Logs:    []string{"severity"},
				}
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := createDefaultConfig()
			cfg, ok := raw.(*Config)
			require.True(t, ok, "createDefaultConfig should return *Config")

			tt.modifyConfig(cfg)
			err := cfg.Validate()
			if tt.expectError {
				require.Error(t, err, "Expected validation error for %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err, "Expected no validation error for %s", tt.name)
			}
		})
	}
}

// NEW: Test adaptive window configuration validation
func TestAdaptiveWindowValidation(t *testing.T) {
	tests := []struct {
		name          string
		modifyConfig  func(*Config)
		expectError   bool
		errorContains string
	}{
		{
			name: "valid adaptive window config",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
			},
			expectError: false,
		},
		{
			name: "nil adaptive window config",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow = nil
			},
			expectError: false,
		},
		{
			name: "zero min window size",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MinWindowSize = 0
			},
			expectError:   true,
			errorContains: "min_window_size must be positive",
		},
		{
			name: "negative min window size",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MinWindowSize = -100
			},
			expectError:   true,
			errorContains: "min_window_size must be positive",
		},
		{
			name: "max window size less than min",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MinWindowSize = 2000
				cfg.AdaptiveWindow.MaxWindowSize = 1000
			},
			expectError:   true,
			errorContains: "max_window_size must be greater than min_window_size",
		},
		{
			name: "max window size equal to min",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MinWindowSize = 1000
				cfg.AdaptiveWindow.MaxWindowSize = 1000
			},
			expectError:   true,
			errorContains: "max_window_size must be greater than min_window_size",
		},
		{
			name: "min window size less than min samples",
			modifyConfig: func(cfg *Config) {
				cfg.MinSamples = 2000
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MinWindowSize = 1000
			},
			expectError:   true,
			errorContains: "adaptive_window.min_window_size (1000) should be >= min_samples (2000) for consistency",
		},
		{
			name: "zero memory limit",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MemoryLimitMB = 0
			},
			expectError:   true,
			errorContains: "memory_limit_mb must be positive",
		},
		{
			name: "negative memory limit",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MemoryLimitMB = -100
			},
			expectError:   true,
			errorContains: "memory_limit_mb must be positive",
		},
		{
			name: "memory limit exceeds processor max",
			modifyConfig: func(cfg *Config) {
				cfg.Performance.MaxMemoryMB = 256
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.MemoryLimitMB = 512
			},
			expectError:   true,
			errorContains: "adaptive_window.memory_limit_mb (512) should not exceed performance.max_memory_mb (256)",
		},
		{
			name: "adaptation rate too low",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.AdaptationRate = -0.1
			},
			expectError:   true,
			errorContains: "adaptation_rate must be between 0.0 and 1.0",
		},
		{
			name: "adaptation rate too high",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.AdaptationRate = 1.5
			},
			expectError:   true,
			errorContains: "adaptation_rate must be between 0.0 and 1.0",
		},
		{
			name: "adaptation rate boundary - minimum",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.AdaptationRate = 0.0
			},
			expectError: false,
		},
		{
			name: "adaptation rate boundary - maximum",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.AdaptationRate = 1.0
			},
			expectError: false,
		},
		{
			name: "negative velocity threshold",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.VelocityThreshold = -10
			},
			expectError:   true,
			errorContains: "velocity_threshold must be non-negative",
		},
		{
			name: "zero velocity threshold (valid)",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.VelocityThreshold = 0
			},
			expectError: false,
		},
		{
			name: "invalid stability check interval",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.StabilityCheckInterval = "invalid_duration"
			},
			expectError:   true,
			errorContains: "stability_check_interval is not a valid duration",
		},
		{
			name: "empty stability check interval (valid - uses default)",
			modifyConfig: func(cfg *Config) {
				cfg.AdaptiveWindow.Enabled = true
				cfg.AdaptiveWindow.StabilityCheckInterval = ""
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := createDefaultConfig()
			cfg, ok := raw.(*Config)
			require.True(t, ok, "createDefaultConfig should return *Config")

			tt.modifyConfig(cfg)
			err := cfg.Validate()
			if tt.expectError {
				require.Error(t, err, "Expected validation error for %s", tt.name)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err, "Expected no validation error for %s", tt.name)
			}
		})
	}
}

// Test adaptive window helper methods
func TestAdaptiveWindowHelperMethods(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Test IsAdaptiveWindowEnabled with default config (disabled)
	assert.False(t, cfg.IsAdaptiveWindowEnabled(), "Should return false when adaptive window is disabled")

	// Test IsAdaptiveWindowEnabled with enabled config
	cfg.AdaptiveWindow.Enabled = true
	assert.True(t, cfg.IsAdaptiveWindowEnabled(), "Should return true when adaptive window is enabled")

	// Test IsAdaptiveWindowEnabled with nil adaptive config
	cfg.AdaptiveWindow = nil
	assert.False(t, cfg.IsAdaptiveWindowEnabled(), "Should return false when adaptive window config is nil")

	// Test GetStabilityCheckInterval with valid duration
	cfg.AdaptiveWindow = &AdaptiveWindowConfig{
		StabilityCheckInterval: "10m",
	}
	interval, err := cfg.GetStabilityCheckInterval()
	require.NoError(t, err, "Should not error with valid duration")
	assert.Equal(t, 10*time.Minute, interval, "Should return correct duration")

	// Test GetStabilityCheckInterval with empty duration (uses default)
	cfg.AdaptiveWindow.StabilityCheckInterval = ""
	interval, err = cfg.GetStabilityCheckInterval()
	require.NoError(t, err, "Should not error with empty duration")
	assert.Equal(t, 5*time.Minute, interval, "Should return default duration")

	// Test GetStabilityCheckInterval with nil adaptive config
	cfg.AdaptiveWindow = nil
	interval, err = cfg.GetStabilityCheckInterval()
	require.NoError(t, err, "Should not error with nil adaptive config")
	assert.Equal(t, 5*time.Minute, interval, "Should return default duration when nil")

	// Test GetStabilityCheckInterval with invalid duration
	cfg.AdaptiveWindow = &AdaptiveWindowConfig{
		StabilityCheckInterval: "invalid_duration",
	}
	_, err = cfg.GetStabilityCheckInterval()
	require.Error(t, err, "Should error with invalid duration")

	assert.Contains(t, err.Error(), "invalid duration", "Should contain duration error message")
}

func TestMultiModelConfiguration(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Test single-model mode (default)
	assert.False(t, cfg.IsMultiModelMode(), "Should not detect multi-model mode by default")

	// Test model selection when not in multi-model mode
	attrs := map[string]any{"service.name": "frontend"}
	selectedModel := cfg.GetModelForAttributes(attrs)
	assert.Nil(t, selectedModel, "Should return nil when not in multi-model mode")

	// Add multiple models with different configurations
	cfg.Models = []ModelConfig{
		{
			Name: "web_service",
			Selector: map[string]string{
				"service.name": "frontend",
				"environment":  "production",
			},
			Features:          []string{"duration", "error", "http.status_code"},
			Threshold:         0.8,
			ForestSize:        150,
			SubsampleSize:     200,
			ContaminationRate: 0.05,
		},
		{
			Name: "database_service",
			Selector: map[string]string{
				"service.name": "database",
			},
			Features:          []string{"duration", "db.statement"},
			Threshold:         0.6,
			ForestSize:        100,
			SubsampleSize:     256,
			ContaminationRate: 0.15,
		},
	}

	// Verify multi-model mode is detected correctly
	assert.True(t, cfg.IsMultiModelMode(), "Should detect multi-model mode")

	// Test model selection with multiple selector conditions
	webServiceAttrs := map[string]any{
		"service.name": "frontend",
		"environment":  "production",
		"http.method":  "GET",
	}
	selectedModel = cfg.GetModelForAttributes(webServiceAttrs)
	require.NotNil(t, selectedModel, "Should find matching model for frontend service")
	assert.Equal(t, "web_service", selectedModel.Name)
	assert.Equal(t, 0.8, selectedModel.Threshold)

	// Test with partial match (missing required selector attribute)
	partialMatchAttrs := map[string]any{
		"service.name": "frontend",
		// Missing "environment": "production"
	}
	selectedModel = cfg.GetModelForAttributes(partialMatchAttrs)
	assert.Nil(t, selectedModel, "Should return nil for partial selector match")

	// Test with single selector condition match
	dbServiceAttrs := map[string]any{
		"service.name": "database",
		"db.type":      "postgresql",
	}
	selectedModel = cfg.GetModelForAttributes(dbServiceAttrs)
	require.NotNil(t, selectedModel, "Should find matching model for database service")
	assert.Equal(t, "database_service", selectedModel.Name)

	// Test with non-matching attributes
	unknownServiceAttrs := map[string]any{
		"service.name": "unknown_service",
	}
	selectedModel = cfg.GetModelForAttributes(unknownServiceAttrs)
	assert.Nil(t, selectedModel, "Should return nil for non-matching attributes")

	// Test with nil attributes map
	selectedModel = cfg.GetModelForAttributes(nil)
	assert.Nil(t, selectedModel, "Should handle nil attributes map gracefully")

	// Test with empty attributes map
	selectedModel = cfg.GetModelForAttributes(map[string]any{})
	assert.Nil(t, selectedModel, "Should handle empty attributes map gracefully")

	// Test type conversion in model selection
	typeConversionAttrs := map[string]any{
		"service.name": "frontend", // string matches string
		"environment":  "production",
	}
	selectedModel = cfg.GetModelForAttributes(typeConversionAttrs)
	require.NotNil(t, selectedModel, "Should handle type conversion correctly")
	assert.Equal(t, "web_service", selectedModel.Name)

	// Test with different types that convert to same string
	numericTypeAttrs := map[string]any{
		"service.name": "database",
	}
	selectedModel = cfg.GetModelForAttributes(numericTypeAttrs)
	require.NotNil(t, selectedModel, "Should find matching model with type conversion")
	assert.Equal(t, "database_service", selectedModel.Name)

	// Verify configuration is still valid
	err := cfg.Validate()
	require.NoError(t, err, "Multi-model configuration should be valid")
}

func TestDurationParsing(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Test valid durations
	validDurations := []string{"1h", "24h", "30m", "1h30m", "2h45m30s"}
	for _, duration := range validDurations {
		cfg.TrainingWindow = duration
		cfg.UpdateFrequency = duration

		trainingDur, err := cfg.GetTrainingWindowDuration()
		require.NoError(t, err)
		assert.Positive(t, trainingDur)

		updateDur, err := cfg.GetUpdateFrequencyDuration()
		require.NoError(t, err)
		assert.Positive(t, updateDur)
	}

	// Test invalid durations for GetTrainingWindowDuration
	cfg.TrainingWindow = "invalid_duration"
	_, err := cfg.GetTrainingWindowDuration()
	require.Error(t, err, "Should return error for invalid training window duration")

	// Test invalid durations for GetUpdateFrequencyDuration
	cfg.UpdateFrequency = "not_a_duration"
	_, err = cfg.GetUpdateFrequencyDuration()
	require.Error(t, err, "Should return error for invalid update frequency duration")
}

func TestComplexModelSelection(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Test with models that have complex selectors
	cfg.Models = []ModelConfig{
		{
			Name: "complex_model_1",
			Selector: map[string]string{
				"service.name":    "api-gateway",
				"service.version": "v2.0",
				"environment":     "staging",
			},
			Features: []string{"duration", "error_rate"},
		},
		{
			Name: "complex_model_2",
			Selector: map[string]string{
				"service.name": "api-gateway",
				"environment":  "production",
			},
			Features: []string{"duration", "throughput"},
		},
	}

	// Test exact match for complex model 1
	exactMatchAttrs := map[string]any{
		"service.name":    "api-gateway",
		"service.version": "v2.0",
		"environment":     "staging",
		"extra.field":     "ignored",
	}
	selectedModel := cfg.GetModelForAttributes(exactMatchAttrs)
	require.NotNil(t, selectedModel, "Should find exact match for complex model 1")
	assert.Equal(t, "complex_model_1", selectedModel.Name)

	// Test partial match should fail
	partialMatchAttrs := map[string]any{
		"service.name": "api-gateway",
		"environment":  "staging",
		// Missing "service.version": "v2.0"
	}
	selectedModel = cfg.GetModelForAttributes(partialMatchAttrs)
	assert.Nil(t, selectedModel, "Should not match with missing selector field")

	// Test match for complex model 2
	model2MatchAttrs := map[string]any{
		"service.name": "api-gateway",
		"environment":  "production",
	}
	selectedModel = cfg.GetModelForAttributes(model2MatchAttrs)
	require.NotNil(t, selectedModel, "Should find match for complex model 2")
	assert.Equal(t, "complex_model_2", selectedModel.Name)

	// Test with wrong value for selector
	wrongValueAttrs := map[string]any{
		"service.name": "api-gateway",
		"environment":  "development", // Wrong value
	}
	selectedModel = cfg.GetModelForAttributes(wrongValueAttrs)
	assert.Nil(t, selectedModel, "Should not match with wrong selector value")
}

func TestEmptyModelsSlice(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Explicitly set empty models slice (different from nil)
	cfg.Models = []ModelConfig{}

	// Should not be in multi-model mode
	assert.False(t, cfg.IsMultiModelMode(), "Empty models slice should not be multi-model mode")

	// Should return nil for any attributes
	attrs := map[string]any{"service.name": "test"}
	selectedModel := cfg.GetModelForAttributes(attrs)
	assert.Nil(t, selectedModel, "Should return nil when models slice is empty")
}

func TestAttributeTypeHandling(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	cfg.Models = []ModelConfig{
		{
			Name: "type_test_model",
			Selector: map[string]string{
				"numeric_field": "123",
				"bool_field":    "true",
				"string_field":  "test_value",
			},
			Features: []string{"duration"},
		},
	}

	// Test with different types that should convert to matching strings
	typeTestAttrs := map[string]any{
		"numeric_field": 123,          // int -> "123"
		"bool_field":    true,         // bool -> "true"
		"string_field":  "test_value", // string -> "test_value"
	}
	selectedModel := cfg.GetModelForAttributes(typeTestAttrs)
	require.NotNil(t, selectedModel, "Should match with type conversion")
	assert.Equal(t, "type_test_model", selectedModel.Name)

	// Test with types that don't match after conversion
	nonMatchingAttrs := map[string]any{
		"numeric_field": 456,          // int -> "456" (doesn't match "123")
		"bool_field":    true,         // bool -> "true"
		"string_field":  "test_value", // string -> "test_value"
	}
	selectedModel = cfg.GetModelForAttributes(nonMatchingAttrs)
	assert.Nil(t, selectedModel, "Should not match when converted values don't match")
}
