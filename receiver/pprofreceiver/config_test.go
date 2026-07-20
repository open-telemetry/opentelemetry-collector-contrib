// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestConfigValidate(t *testing.T) {
	t.Run("empty config is invalid", func(t *testing.T) {
		cfg := &Config{}
		require.Error(t, cfg.Validate())
	})

	t.Run("remote without endpoint is invalid", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"remote": map[string]any{},
		})
		cfg := createDefaultConfig().(*Config)
		require.NoError(t, conf.Unmarshal(cfg))
		assert.ErrorContains(t, cfg.Validate(), "remote.endpoint")
	})

	t.Run("file without include is invalid", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"file": map[string]any{},
		})
		cfg := createDefaultConfig().(*Config)
		require.NoError(t, conf.Unmarshal(cfg))
		assert.ErrorContains(t, cfg.Validate(), "file.include")
	})

	t.Run("self only is valid", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"self": map[string]any{},
		})
		cfg := createDefaultConfig().(*Config)
		require.NoError(t, conf.Unmarshal(cfg))
		require.NoError(t, cfg.Validate())
		assert.True(t, cfg.Self.HasValue())
		assert.False(t, cfg.Remote.HasValue())
	})

	t.Run("nested sections unmarshal", func(t *testing.T) {
		conf := confmap.NewFromStringMap(map[string]any{
			"remote": map[string]any{
				"endpoint":            "http://my-svc:9090",
				"collection_interval": "30s",
				"initial_delay":       "10s",
			},
			"server": map[string]any{
				"endpoint": "0.0.0.0:4040",
			},
			"file": map[string]any{
				"include":             "/tmp/pprof/*",
				"collection_interval": "10s",
			},
			"self": map[string]any{
				"collection_interval": "15s",
			},
		})
		cfg := createDefaultConfig().(*Config)
		require.NoError(t, conf.Unmarshal(cfg))
		require.NoError(t, cfg.Validate())
		assert.Equal(t, "http://my-svc:9090", cfg.Remote.Get().Endpoint)
		assert.Equal(t, "0.0.0.0:4040", cfg.Server.Get().NetAddr.Endpoint)
		assert.Equal(t, "/tmp/pprof/*", cfg.File.Get().Include)
		assert.True(t, cfg.Self.HasValue())
	})
}
