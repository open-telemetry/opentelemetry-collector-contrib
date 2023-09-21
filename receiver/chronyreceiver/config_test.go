// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chronyreceiver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "custom").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	scs := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	scs.Timeout = 10 * time.Second

	assert.Equal(t, &Config{
		ScraperControllerSettings: scs,
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
		Endpoint:                  "udp://localhost:3030",
	}, cfg)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		conf     Config
		err      error
	}{
		{
			scenario: "Valid udp configuration",
			conf: Config{
				Endpoint: "udp://localhost:323",
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: time.Minute,
					InitialDelay:       time.Second,
					Timeout:            10 * time.Second,
				},
			},
			err: nil,
		},
		{
			scenario: "Invalid udp hostname",
			conf: Config{
				Endpoint: "udp://:323",
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: time.Minute,
					InitialDelay:       time.Second,
					Timeout:            10 * time.Second,
				},
			},
			err: chrony.ErrInvalidNetwork,
		},
		{
			scenario: "Invalid udp port",
			conf: Config{
				Endpoint: "udp://localhost",
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: time.Minute,
					InitialDelay:       time.Second,
					Timeout:            10 * time.Second,
				},
			},
			err: chrony.ErrInvalidNetwork,
		},
		{
			scenario: "Valid unix path",
			conf: Config{
				Endpoint: fmt.Sprintf("unix://%s", t.TempDir()),
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: time.Minute,
					InitialDelay:       time.Second,
					Timeout:            10 * time.Second,
				},
			},
			err: nil,
		},
		{
			scenario: "Invalid unix path",
			conf: Config{
				Endpoint: "unix:///no/dir/to/socket",
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: time.Minute,
					InitialDelay:       time.Second,
					Timeout:            10 * time.Second,
				},
			},
			err: os.ErrNotExist,
		},
		{
			scenario: "Invalid timeout set",
			conf: Config{
				Endpoint: "unix://no/dir/to/socket",
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: time.Minute,
					InitialDelay:       time.Second,
					Timeout:            0,
				},
			},
			err: errInvalidValue,
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			assert.ErrorIs(t, tc.conf.Validate(), tc.err, "Must match the expected error")
		})
	}
}
