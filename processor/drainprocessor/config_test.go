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

	t.Run("tree_depth < 3", func(t *testing.T) {
		cfg := valid()
		cfg.TreeDepth = 2
		assert.Error(t, cfg.Validate())
	})

	t.Run("tree_depth == 3 is valid", func(t *testing.T) {
		cfg := valid()
		cfg.TreeDepth = 3
		require.NoError(t, cfg.Validate())
	})

	t.Run("merge_threshold below range", func(t *testing.T) {
		cfg := valid()
		cfg.MergeThreshold = -0.1
		assert.Error(t, cfg.Validate())
	})

	t.Run("merge_threshold above range", func(t *testing.T) {
		cfg := valid()
		cfg.MergeThreshold = 1.1
		assert.Error(t, cfg.Validate())
	})

	t.Run("merge_threshold at boundaries", func(t *testing.T) {
		cfg := valid()
		cfg.MergeThreshold = 0.0
		require.NoError(t, cfg.Validate())
		cfg.MergeThreshold = 1.0
		require.NoError(t, cfg.Validate())
	})

	t.Run("warmup_min_clusters negative", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMinClusters = -1
		assert.Error(t, cfg.Validate())
	})

	t.Run("warmup_min_clusters zero is valid", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMinClusters = 0
		require.NoError(t, cfg.Validate())
	})

	t.Run("warmup_min_clusters positive is valid", func(t *testing.T) {
		cfg := valid()
		cfg.WarmupMinClusters = 20
		require.NoError(t, cfg.Validate())
	})
}
