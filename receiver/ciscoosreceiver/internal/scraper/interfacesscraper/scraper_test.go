// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper/internal/metadata"
)

func TestInterfacesScraper_Start(t *testing.T) {
	logger := zap.NewNop()

	config := &Config{
		Device: connection.DeviceConfig{
			Device: connection.DeviceInfo{
				Host: connection.HostInfo{
					Name: "test-device",
					IP:   "192.168.1.1",
					Port: 22,
				},
			},
			Auth: connection.AuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
		},
	}
	config.MetricsBuilderConfig = metadata.DefaultMetricsBuilderConfig()

	scraper := &interfacesScraper{
		logger: logger,
		config: config,
	}

	err := scraper.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, scraper.mb, "MetricsBuilder should be initialized")
	assert.Equal(t, "192.168.1.1", scraper.deviceTarget)
}

func TestInterfacesScraper_Start_EmptyIP(t *testing.T) {
	logger := zap.NewNop()

	config := &Config{
		Device: connection.DeviceConfig{
			Device: connection.DeviceInfo{
				Host: connection.HostInfo{
					Name: "test-device",
					IP:   "",
					Port: 22,
				},
			},
		},
	}
	config.MetricsBuilderConfig = metadata.DefaultMetricsBuilderConfig()

	scraper := &interfacesScraper{
		logger: logger,
		config: config,
	}

	err := scraper.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, scraper.mb, "MetricsBuilder should be initialized even with empty IP")
}

func TestInterfacesScraper_Shutdown(t *testing.T) {
	logger := zap.NewNop()

	scraper := &interfacesScraper{
		logger: logger,
	}

	err := scraper.Shutdown(t.Context())
	require.NoError(t, err)
}
