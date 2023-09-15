// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 10 * time.Second,
					InitialDelay:       time.Second,
					Timeout:            5 * time.Second,
				},
				APIVersion: defaultAPIVersion,
				Endpoint:   "unix:///run/podman/podman.sock",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all"),
			expected: &Config{
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 2 * time.Second,
					InitialDelay:       time.Second,
					Timeout:            20 * time.Second,
				},
				APIVersion: defaultAPIVersion,
				Endpoint:   "http://example.com/",
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
		})
	}
}
