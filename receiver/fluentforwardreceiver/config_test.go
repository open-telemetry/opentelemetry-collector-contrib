// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		expected *Config
	}{
		{
			name:     "default",
			file:     "config.yaml",
			expected: createDefaultConfig().(*Config),
		},
		{
			name:     "max_connections",
			file:     "config_max_connections.yaml",
			expected: &Config{MaxConnections: 5},
		},
		{
			name:     "refuse_over_limit",
			file:     "config_refuse_over_limit.yaml",
			expected: &Config{MaxConnections: 5, RefuseOverLimit: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.file))
			require.NoError(t, err)

			cfg := NewFactory().CreateDefaultConfig()
			sub, err := cm.Sub(metadata.Type.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateRejectsNegativeMaxConnections(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxConnections = -1
	require.ErrorIs(t, cfg.Validate(), errNegativeMaxConnections)
}

func TestValidateRejectsRefuseWithoutLimit(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxConnections = 0
	cfg.RefuseOverLimit = true
	require.ErrorIs(t, cfg.Validate(), errRefuseWithoutLimit)
}
