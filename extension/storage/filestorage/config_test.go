// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: func() component.Config {
				ret := NewFactory().CreateDefaultConfig()
				ret.(*Config).Directory = "."
				return ret
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				Directory: ".",
				Compaction: &CompactionConfig{
					Directory:                  ".",
					OnStart:                    true,
					OnRebound:                  true,
					MaxTransactionSize:         2048,
					ReboundTriggerThresholdMiB: 16,
					ReboundNeededThresholdMiB:  128,
					CheckInterval:              time.Second * 5,
					CleanupOnStart:             true,
				},
				Timeout: 2 * time.Second,
				FSync:   true,
			},
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

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestHandleNonExistingDirectoryWithAnError(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = "/not/a/dir"

	err := component.ValidateConfig(cfg)
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), "directory must exist: "))
}

func TestHandleProvidingFilePathAsDirWithAnError(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)

	file, err := os.CreateTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, file.Close())
		require.NoError(t, os.Remove(file.Name()))
	})

	cfg.Directory = file.Name()

	err = component.ValidateConfig(cfg)
	require.Error(t, err)
	require.EqualError(t, err, file.Name()+" is not a directory")
}

func TestCompactionDirectory(t *testing.T) {
	f := NewFactory()
	tests := []struct {
		name   string
		config func(*testing.T) *Config
		err    error
	}{
		{
			name: "directory-must-exists-error",
			config: func(t *testing.T) *Config {
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = t.TempDir()             // actual directory
				cfg.Compaction.Directory = "/not/a/dir" // not a directory
				cfg.Compaction.OnRebound = true
				cfg.Compaction.OnStart = true
				return cfg
			},
			err: os.ErrNotExist,
		},
		{
			name: "directory-must-exists-error-on-start",
			config: func(t *testing.T) *Config {
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = t.TempDir()             // actual directory
				cfg.Compaction.Directory = "/not/a/dir" // not a directory
				cfg.Compaction.OnRebound = false
				cfg.Compaction.OnStart = true
				return cfg
			},
			err: os.ErrNotExist,
		},
		{
			name: "directory-must-exists-error-on-rebound",
			config: func(t *testing.T) *Config {
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = t.TempDir()             // actual directory
				cfg.Compaction.Directory = "/not/a/dir" // not a directory
				cfg.Compaction.OnRebound = true
				cfg.Compaction.OnStart = false
				return cfg
			},
			err: os.ErrNotExist,
		},
		{
			name: "compaction-disabled-no-error",
			config: func(t *testing.T) *Config {
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = t.TempDir()             // actual directory
				cfg.Compaction.Directory = "/not/a/dir" // not a directory
				cfg.Compaction.OnRebound = false
				cfg.Compaction.OnStart = false
				return cfg
			},
			err: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorIs(t, component.ValidateConfig(test.config(t)), test.err)
		})
	}

}
