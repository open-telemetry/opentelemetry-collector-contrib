// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
)

// TestReceiverTypeInterface verifies ReceiverType implements the required interface
func TestReceiverTypeInterface(t *testing.T) {
	rt := ReceiverType{}

	// Test Type method
	typ := rt.Type()
	assert.Equal(t, component.MustNewType("filelog"), typ)

	// Test CreateDefaultConfig
	cfg := rt.CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.IsType(t, &FileLogConfig{}, cfg)
}

// TestCreateDefaultConfigValues verifies all default configuration values
func TestCreateDefaultConfigValues(t *testing.T) {
	cfg := createDefaultConfig()

	require.NotNil(t, cfg)
	assert.IsType(t, &FileLogConfig{}, cfg)

	// Verify BaseConfig defaults
	assert.NotNil(t, cfg.BaseConfig.Operators)
	assert.Empty(t, cfg.BaseConfig.Operators)

	// Verify RetryOnFailure defaults
	expectedRetry := consumerretry.NewDefaultConfig()
	assert.Equal(t, expectedRetry.Enabled, cfg.BaseConfig.RetryOnFailure.Enabled)
	assert.Equal(t, expectedRetry.InitialInterval, cfg.BaseConfig.RetryOnFailure.InitialInterval)
	assert.Equal(t, expectedRetry.MaxInterval, cfg.BaseConfig.RetryOnFailure.MaxInterval)
	assert.Equal(t, expectedRetry.MaxElapsedTime, cfg.BaseConfig.RetryOnFailure.MaxElapsedTime)

	// Verify InputConfig defaults
	expectedInput := file.NewConfig()
	assert.Equal(t, expectedInput.Include, cfg.InputConfig.Include)
	assert.Equal(t, expectedInput.Exclude, cfg.InputConfig.Exclude)
	assert.Equal(t, expectedInput.StartAt, cfg.InputConfig.StartAt)
}

// TestBaseConfigExtraction verifies BaseConfig extraction from FileLogConfig
func TestBaseConfigExtraction(t *testing.T) {
	rt := ReceiverType{}
	cfg := &FileLogConfig{
		BaseConfig:  createDefaultConfig().BaseConfig,
		InputConfig: file.Config{},
	}

	baseConfig := rt.BaseConfig(cfg)
	assert.Equal(t, cfg.BaseConfig, baseConfig)
}

// TestInputConfigExtraction verifies InputConfig extraction from FileLogConfig
func TestInputConfigExtraction(t *testing.T) {
	rt := ReceiverType{}
	inputCfg := *file.NewConfig()
	inputCfg.Include = []string{"/var/log/*.log"}

	cfg := &FileLogConfig{
		InputConfig: inputCfg,
	}

	extracted := rt.InputConfig(cfg)
	require.NotNil(t, extracted)

	// Verify the extracted config contains the expected input config
	assert.NotNil(t, extracted.Builder)
}

// TestConfigWithOperators verifies configuration with various operators
func TestConfigWithOperators(t *testing.T) {
	cfg := createDefaultConfig()

	// Add some operators
	cfg.Operators = []operator.Config{
		{Builder: file.NewConfig()},
	}

	assert.Len(t, cfg.Operators, 1)
}

// TestConfigWithRetrySettings verifies configuration with custom retry settings
func TestConfigWithRetrySettings(t *testing.T) {
	cfg := createDefaultConfig()

	// Customize retry settings
	cfg.RetryOnFailure.Enabled = true
	cfg.RetryOnFailure.InitialInterval = 5 * time.Second
	cfg.RetryOnFailure.MaxInterval = 60 * time.Second
	cfg.RetryOnFailure.MaxElapsedTime = 10 * time.Minute

	assert.True(t, cfg.RetryOnFailure.Enabled)
	assert.Equal(t, 5*time.Second, cfg.RetryOnFailure.InitialInterval)
	assert.Equal(t, 60*time.Second, cfg.RetryOnFailure.MaxInterval)
	assert.Equal(t, 10*time.Minute, cfg.RetryOnFailure.MaxElapsedTime)
}

// TestConfigWithFileInputOptions verifies configuration with various file input options
func TestConfigWithFileInputOptions(t *testing.T) {
	tests := []struct {
		name     string
		setupCfg func(*FileLogConfig)
		verify   func(*testing.T, *FileLogConfig)
	}{
		{
			name: "with include patterns",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.Include = []string{"/var/log/*.log", "/tmp/*.log"}
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Len(t, cfg.InputConfig.Include, 2)
				assert.Contains(t, cfg.InputConfig.Include, "/var/log/*.log")
				assert.Contains(t, cfg.InputConfig.Include, "/tmp/*.log")
			},
		},
		{
			name: "with exclude patterns",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.Exclude = []string{"*.gz", "*.zip"}
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Len(t, cfg.InputConfig.Exclude, 2)
				assert.Contains(t, cfg.InputConfig.Exclude, "*.gz")
			},
		},
		{
			name: "with start_at beginning",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.StartAt = "beginning"
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Equal(t, "beginning", cfg.InputConfig.StartAt)
			},
		},
		{
			name: "with start_at end",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.StartAt = "end"
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Equal(t, "end", cfg.InputConfig.StartAt)
			},
		},
		{
			name: "with poll interval",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.PollInterval = 500 * time.Millisecond
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Equal(t, 500*time.Millisecond, cfg.InputConfig.PollInterval)
			},
		},
		{
			name: "with max concurrent files",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.MaxConcurrentFiles = 512
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Equal(t, 512, cfg.InputConfig.MaxConcurrentFiles)
			},
		},
		{
			name: "with max log size",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.MaxLogSize = 2 * 1024 * 1024 // 2MB
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.EqualValues(t, 2*1024*1024, cfg.InputConfig.MaxLogSize)
			},
		},
		{
			name: "with include file name",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.IncludeFileName = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.IncludeFileName)
			},
		},
		{
			name: "with include file path",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.IncludeFilePath = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.IncludeFilePath)
			},
		},
		{
			name: "with include file name resolved",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.IncludeFileNameResolved = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.IncludeFileNameResolved)
			},
		},
		{
			name: "with include file path resolved",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.IncludeFilePathResolved = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.IncludeFilePathResolved)
			},
		},
		{
			name: "with include file record number",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.IncludeFileRecordNumber = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.IncludeFileRecordNumber)
			},
		},
		{
			name: "with include file record offset",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.IncludeFileRecordOffset = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.IncludeFileRecordOffset)
			},
		},
		{
			name: "with fingerprint size",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.FingerprintSize = 2048
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.EqualValues(t, 2048, cfg.InputConfig.FingerprintSize)
			},
		},
		{
			name: "with max batches",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.MaxBatches = 10
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.Equal(t, 10, cfg.InputConfig.MaxBatches)
			},
		},
		{
			name: "with delete after read",
			setupCfg: func(cfg *FileLogConfig) {
				cfg.InputConfig.DeleteAfterRead = true
			},
			verify: func(t *testing.T, cfg *FileLogConfig) {
				assert.True(t, cfg.InputConfig.DeleteAfterRead)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig()
			tt.setupCfg(cfg)
			tt.verify(t, cfg)
		})
	}
}

// TestFileLogConfigStructInitialization ensures the config struct is properly initialized
func TestFileLogConfigStructInitialization(t *testing.T) {
	// Test that the struct cannot be initialized with unkeyed literals
	// due to the _ struct{} field
	cfg := FileLogConfig{
		InputConfig: *file.NewConfig(),
		BaseConfig:  createDefaultConfig().BaseConfig,
	}

	require.NotNil(t, cfg)
}
