// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	_, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	require.NotNil(t, factory)
	require.NotNil(t, factory.CreateDefaultConfig())
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			config: &Config{
				CacheTTL:    60,
				Attributes:  []string{"^aws.*", "^docker.*"},
				ContainerID: ContainerID{Sources: []string{"container.id"}},
			},
		},
		{
			name: "valid config with multiple sources",
			config: &Config{
				CacheTTL:    300,
				Attributes:  []string{".*"},
				ContainerID: ContainerID{Sources: []string{"container.id", "log.file.name"}},
			},
		},
		{
			name: "valid config with empty attributes",
			config: &Config{
				CacheTTL:    60,
				Attributes:  []string{},
				ContainerID: ContainerID{Sources: []string{"container.id"}},
			},
		},
		{
			name: "invalid - no container ID sources",
			config: &Config{
				CacheTTL:    60,
				Attributes:  []string{"^aws.*"},
				ContainerID: ContainerID{Sources: []string{}},
			},
			wantErr: true,
			errMsg:  "at least one container ID source must be specified",
		},
		{
			name: "invalid - cache TTL too low",
			config: &Config{
				CacheTTL:    30,
				Attributes:  []string{"^aws.*"},
				ContainerID: ContainerID{Sources: []string{"container.id"}},
			},
			wantErr: true,
			errMsg:  "cache_ttl cannot be less than 60 seconds",
		},
		{
			name: "invalid - bad regex pattern",
			config: &Config{
				CacheTTL:    60,
				Attributes:  []string{"?="},
				ContainerID: ContainerID{Sources: []string{"container.id"}},
			},
			wantErr: true,
			errMsg:  "invalid expression found under attributes pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg, ok := NewFactory().CreateDefaultConfig().(*Config)
	require.True(t, ok)
	require.NoError(t, cfg.Validate())

	// By default no attribute patterns are configured, so all available ECS
	// metadata attributes are collected.
	require.Empty(t, cfg.Attributes)
}

func TestConfigInit(t *testing.T) {
	t.Run("compiles patterns", func(t *testing.T) {
		cfg := &Config{
			CacheTTL:    60,
			Attributes:  []string{"^aws.*", "^docker.*"},
			ContainerID: ContainerID{Sources: []string{"container.id"}},
		}
		require.NoError(t, cfg.init())
		require.Len(t, cfg.attrExpressions, 2)
	})

	t.Run("is idempotent", func(t *testing.T) {
		cfg := &Config{
			CacheTTL:    60,
			Attributes:  []string{"^aws.*"},
			ContainerID: ContainerID{Sources: []string{"container.id"}},
		}
		require.NoError(t, cfg.init())
		require.NoError(t, cfg.init())
		require.Len(t, cfg.attrExpressions, 1)
	})

	t.Run("fails validation", func(t *testing.T) {
		cfg := &Config{CacheTTL: 60, ContainerID: ContainerID{Sources: nil}}
		require.Error(t, cfg.init())
	})
}

func TestConfigAllowAttr(t *testing.T) {
	t.Run("with patterns", func(t *testing.T) {
		cfg := &Config{
			CacheTTL:    60,
			Attributes:  []string{"^aws.*", "^docker.*", "^image.*"},
			ContainerID: ContainerID{Sources: []string{"container.id"}},
		}
		require.NoError(t, cfg.init())
		require.True(t, cfg.allowAttr("aws.ecs.cluster"))
		require.True(t, cfg.allowAttr("docker.id"))
		require.True(t, cfg.allowAttr("image.id"))
		require.False(t, cfg.allowAttr("random.attribute"))
	})

	t.Run("empty patterns allow all", func(t *testing.T) {
		cfg := &Config{
			CacheTTL:    60,
			Attributes:  nil,
			ContainerID: ContainerID{Sources: []string{"container.id"}},
		}
		require.NoError(t, cfg.init())
		require.True(t, cfg.allowAttr("anything"))
	})
}
