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
