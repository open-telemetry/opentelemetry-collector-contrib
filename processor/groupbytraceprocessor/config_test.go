// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	valid := func() *Config {
		return &Config{
			NumTraces:    100,
			NumWorkers:   2,
			WaitDuration: time.Second,
			EmitStrategy: EmitStrategyTrace,
		}
	}

	for _, tt := range []struct {
		name        string
		mutate      func(*Config)
		expectedErr string
	}{
		{
			name:   "defaults",
			mutate: func(*Config) {},
		},
		{
			name:   "service strategy",
			mutate: func(cfg *Config) { cfg.EmitStrategy = EmitStrategyService },
		},
		{
			name:        "unknown emit strategy",
			mutate:      func(cfg *Config) { cfg.EmitStrategy = EmitStrategy("invalid") },
			expectedErr: `unknown emit_strategy "invalid"`,
		},
		{
			name:        "discard orphans not supported",
			mutate:      func(cfg *Config) { cfg.DiscardOrphans = true },
			expectedErr: "option 'discard orphans' not supported in this release",
		},
		{
			name:        "store on disk not supported",
			mutate:      func(cfg *Config) { cfg.StoreOnDisk = true },
			expectedErr: "option 'disk storage' not supported in this release",
		},
		{
			name:        "zero wait duration",
			mutate:      func(cfg *Config) { cfg.WaitDuration = 0 },
			expectedErr: "wait_duration must be greater than 0",
		},
		{
			name:        "negative wait duration",
			mutate:      func(cfg *Config) { cfg.WaitDuration = -time.Second },
			expectedErr: "wait_duration must be greater than 0",
		},
		{
			name:        "zero num traces",
			mutate:      func(cfg *Config) { cfg.NumTraces = 0 },
			expectedErr: "num_traces must be greater than 0",
		},
		{
			name:        "negative num traces",
			mutate:      func(cfg *Config) { cfg.NumTraces = -1 },
			expectedErr: "num_traces must be greater than 0",
		},
		{
			name:        "zero num workers",
			mutate:      func(cfg *Config) { cfg.NumWorkers = 0 },
			expectedErr: "num_workers must be greater than 0",
		},
		{
			name:        "negative num workers",
			mutate:      func(cfg *Config) { cfg.NumWorkers = -1 },
			expectedErr: "num_workers must be greater than 0",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(cfg)

			err := cfg.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	require.NoError(t, createDefaultConfig().(*Config).Validate())
}
