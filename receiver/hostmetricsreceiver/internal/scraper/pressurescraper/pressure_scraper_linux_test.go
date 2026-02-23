// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package pressurescraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	tests := []struct {
		name                string
		metricsConfig       metadata.MetricsBuilderConfig
		rootPath            string
		expectedMetricCount int
		expectedErr         bool
		bootTimeFunc        func(context.Context) (uint64, error)
		initializationErr   bool
	}{
		{
			name:                "defaults",
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			rootPath:            "testdata/procfs",
			expectedMetricCount: 4,
		},
		{
			name: "all metrics enabled",
			metricsConfig: func() metadata.MetricsBuilderConfig {
				allEnabled := metadata.DefaultMetricsBuilderConfig()
				allEnabled.Metrics.SystemCPULinuxPressure10s.Enabled = true
				allEnabled.Metrics.SystemCPULinuxPressure1m.Enabled = true
				allEnabled.Metrics.SystemCPULinuxPressure5m.Enabled = true
				allEnabled.Metrics.SystemMemoryLinuxPressure10s.Enabled = true
				allEnabled.Metrics.SystemMemoryLinuxPressure1m.Enabled = true
				allEnabled.Metrics.SystemMemoryLinuxPressure5m.Enabled = true
				allEnabled.Metrics.SystemIoLinuxPressure10s.Enabled = true
				allEnabled.Metrics.SystemIoLinuxPressure1m.Enabled = true
				allEnabled.Metrics.SystemIoLinuxPressure5m.Enabled = true
				allEnabled.Metrics.SystemIrqLinuxPressure10s.Enabled = true
				allEnabled.Metrics.SystemIrqLinuxPressure1m.Enabled = true
				allEnabled.Metrics.SystemIrqLinuxPressure5m.Enabled = true
				return allEnabled
			}(),
			rootPath:            "testdata/procfs",
			expectedMetricCount: 16,
		},
		{
			name:              "bad data",
			metricsConfig:     metadata.DefaultMetricsBuilderConfig(),
			rootPath:          "testdata/invalid_procfs",
			expectedErr:       true,
			initializationErr: false,
		},
		{
			name:              "boot time error",
			metricsConfig:     metadata.DefaultMetricsBuilderConfig(),
			rootPath:          "testdata",
			bootTimeFunc:      func(context.Context) (uint64, error) { return 0, assert.AnError },
			initializationErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{MetricsBuilderConfig: tt.metricsConfig}
			cfg.SetRootPath(tt.rootPath)

			s := newPressureScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)
			if tt.bootTimeFunc != nil {
				s.bootTime = tt.bootTimeFunc
			}

			err := s.start(t.Context(), componenttest.NewNopHost())
			if tt.initializationErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			md, err := s.scrape(t.Context())
			if tt.expectedErr {
				require.Error(t, err)
				assert.True(t, scrapererror.IsPartialScrapeError(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMetricCount, md.MetricCount())
		})
	}
}
