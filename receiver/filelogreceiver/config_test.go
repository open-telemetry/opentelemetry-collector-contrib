// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
