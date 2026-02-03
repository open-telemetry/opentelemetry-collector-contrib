// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/jsonfileobserver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				Path:            "/path/to/endpoints.json",
				RefreshInterval: 30 * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "fast-refresh"),
			expected: &Config{
				Path:            "/path/to/fast.json",
				RefreshInterval: 1 * time.Second,
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_no_path"),
			expectedErr: "path must be specified",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_negative_interval"),
			expectedErr: "refresh_interval must be non-negative",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			if tt.expectedErr != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.expectedErr)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      Config
		expectedErr string
	}{
		{
			name: "valid config",
			config: Config{
				Path:            "/some/path.json",
				RefreshInterval: 10 * time.Second,
			},
			expectedErr: "",
		},
		{
			name: "valid config with zero interval",
			config: Config{
				Path:            "/some/path.json",
				RefreshInterval: 0,
			},
			expectedErr: "",
		},
		{
			name: "missing path",
			config: Config{
				RefreshInterval: 10 * time.Second,
			},
			expectedErr: "path must be specified",
		},
		{
			name: "negative interval",
			config: Config{
				Path:            "/some/path.json",
				RefreshInterval: -1 * time.Second,
			},
			expectedErr: "refresh_interval must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
