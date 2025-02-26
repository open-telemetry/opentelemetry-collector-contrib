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
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id              component.ID
		expected        component.Config
		expectedErrMsgs []string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 10 * time.Second,
					InitialDelay:       time.Second,
					Timeout:            5 * time.Second,
				},
				APIVersion:           defaultAPIVersion,
				Endpoint:             "unix:///run/podman/podman.sock",
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all"),
			expected: &Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 2 * time.Second,
					InitialDelay:       time.Second,
					Timeout:            20 * time.Second,
				},
				APIVersion:           defaultAPIVersion,
				Endpoint:             "http://example.com/",
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			id:              component.NewIDWithName(metadata.Type, "empty_endpoint"),
			expectedErrMsgs: []string{"config.Endpoint must be specified"},
		},
		{
			id:              component.NewIDWithName(metadata.Type, "invalid_collection_interval"),
			expectedErrMsgs: []string{`config.CollectionInterval must be specified`, `"collection_interval": requires positive value`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if len(tt.expectedErrMsgs) > 0 {
				for _, msg := range tt.expectedErrMsgs {
					assert.ErrorContains(t, xconfmap.Validate(cfg), msg)
				}
				return
			}

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
