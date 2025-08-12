// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// config_test.go - Comprehensive tests for configuration validation and parsing
package isolationforestprocessor

import (
	"testing"

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

	// Most importantly, verify the configuration validates
	err := cfg.Validate()
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
			modifyConfig: func(cfg *Config) {
				// Use default config - should be valid
			},
			expectError: false,
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
			name:          "invalid mode",
			modifyConfig:  func(cfg *Config) { cfg.Mode = "invalid_mode" },
			expectError:   true,
			errorContains: "mode must be 'enrich', 'filter', or 'both'",
		},
		{
			name:          "invalid threshold - too high",
			modifyConfig:  func(cfg *Config) { cfg.Threshold = 1.5 },
			expectError:   true,
			errorContains: "threshold must be between 0.0 and 1.0",
		},
		{
			name:          "invalid training window",
			modifyConfig:  func(cfg *Config) { cfg.TrainingWindow = "invalid_duration" },
			expectError:   true,
			errorContains: "training_window is not a valid duration",
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

func TestMultiModelConfiguration(t *testing.T) {
	raw := createDefaultConfig()
	cfg, ok := raw.(*Config)
	require.True(t, ok, "createDefaultConfig should return *Config")

	// Add multiple models with different configurations
	cfg.Models = []ModelConfig{
		{
			Name: "web_service",
			Selector: map[string]string{
				"service.name": "frontend",
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

	// Test model selection based on attributes
	webServiceAttrs := map[string]interface{}{
		"service.name": "frontend",
		"http.method":  "GET",
	}
	selectedModel := cfg.GetModelForAttributes(webServiceAttrs)
	require.NotNil(t, selectedModel, "Should find matching model for frontend service")
	assert.Equal(t, "web_service", selectedModel.Name)
	assert.Equal(t, 0.8, selectedModel.Threshold)

	// Test with non-matching attributes
	unknownServiceAttrs := map[string]interface{}{
		"service.name": "unknown_service",
	}
	selectedModel = cfg.GetModelForAttributes(unknownServiceAttrs)
	assert.Nil(t, selectedModel, "Should return nil for non-matching attributes")

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
		assert.True(t, trainingDur > 0)

		updateDur, err := cfg.GetUpdateFrequencyDuration()
		require.NoError(t, err)
		assert.True(t, updateDur > 0)
	}
}
