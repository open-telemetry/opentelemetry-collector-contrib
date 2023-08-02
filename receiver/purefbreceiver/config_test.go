// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				Settings: &Settings{
					ReloadIntervals: &ReloadIntervals{
						Array:   15 * time.Second,
						Clients: 5 * time.Minute,
						Usage:   5 * time.Minute,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)

			expected := factory.CreateDefaultConfig().(*Config)

			require.Equal(t, expected, cfg)
		})
	}
}
