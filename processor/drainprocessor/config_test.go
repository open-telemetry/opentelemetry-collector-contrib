// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	valid := func() *Config {
		cfg := createDefaultConfig().(*Config)
		return cfg
	}

	t.Run("valid default config", func(t *testing.T) {
		require.NoError(t, valid().Validate())
	})

	t.Run("log_cluster_depth < 3", func(t *testing.T) {
		cfg := valid()
		cfg.LogClusterDepth = 2
		assert.Error(t, cfg.Validate())
	})

	t.Run("log_cluster_depth == 3 is valid", func(t *testing.T) {
		cfg := valid()
		cfg.LogClusterDepth = 3
		require.NoError(t, cfg.Validate())
	})

	t.Run("sim_threshold below range", func(t *testing.T) {
		cfg := valid()
		cfg.SimThreshold = -0.1
		assert.Error(t, cfg.Validate())
	})

	t.Run("sim_threshold above range", func(t *testing.T) {
		cfg := valid()
		cfg.SimThreshold = 1.1
		assert.Error(t, cfg.Validate())
	})

	t.Run("sim_threshold at boundaries", func(t *testing.T) {
		cfg := valid()
		cfg.SimThreshold = 0.0
		require.NoError(t, cfg.Validate())
		cfg.SimThreshold = 1.0
		require.NoError(t, cfg.Validate())
	})

	t.Run("invalid warmup_mode", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMode = "invalid"
		assert.Error(t, cfg.Validate())
	})

	t.Run("buffer mode requires warmup_min_clusters > 0", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMode = warmupModeBuffer
		cfg.WarmupMinClusters = 0
		cfg.WarmupBufferMaxLogs = 100
		assert.Error(t, cfg.Validate())
	})

	t.Run("buffer mode requires warmup_buffer_max_logs > 0", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMode = warmupModeBuffer
		cfg.WarmupBufferMaxLogs = 0
		assert.Error(t, cfg.Validate())
	})

	t.Run("buffer mode valid", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMode = warmupModeBuffer
		cfg.WarmupBufferMaxLogs = 100
		require.NoError(t, cfg.Validate())
	})
}
