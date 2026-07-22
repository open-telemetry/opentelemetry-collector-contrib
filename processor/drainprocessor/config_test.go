// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	validCfg := func() *Config {
		return createDefaultConfig().(*Config)
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{
			name:    "default config is valid",
			mutate:  func(_ *Config) {},
			wantErr: false,
		},
		{
			name:    "tree_depth below minimum",
			mutate:  func(c *Config) { c.TreeDepth = 2 },
			wantErr: true,
		},
		{
			name:    "tree_depth at minimum",
			mutate:  func(c *Config) { c.TreeDepth = 3 },
			wantErr: false,
		},
		{
			name:    "merge_threshold below range",
			mutate:  func(c *Config) { c.MergeThreshold = -0.1 },
			wantErr: true,
		},
		{
			name:    "merge_threshold above range",
			mutate:  func(c *Config) { c.MergeThreshold = 1.1 },
			wantErr: true,
		},
		{
			name:    "merge_threshold at lower boundary",
			mutate:  func(c *Config) { c.MergeThreshold = 0.0 },
			wantErr: false,
		},
		{
			name:    "merge_threshold at upper boundary",
			mutate:  func(c *Config) { c.MergeThreshold = 1.0 },
			wantErr: false,
		},
		{
			name:    "warmup_min_clusters negative",
			mutate:  func(c *Config) { c.WarmupMinClusters = -1 },
			wantErr: true,
		},
		{
			name:    "warmup_min_clusters zero",
			mutate:  func(c *Config) { c.WarmupMinClusters = 0 },
			wantErr: false,
		},
		{
			name:    "warmup_min_clusters positive",
			mutate:  func(c *Config) { c.WarmupMinClusters = 20 },
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validCfg()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
