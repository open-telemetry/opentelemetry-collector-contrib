// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package pressurescraper

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	tests := []struct {
		name           string
		procDir        string
		expected       map[string]float64
		expectedErr    string
		expectedFailed int
	}{
		{
			name:    "all resources",
			procDir: "proc",
			expected: map[string]float64{
				"system.cpu.linux.pressure.some_time":    1.5,
				"system.memory.linux.pressure.some_time": 2.5,
				"system.memory.linux.pressure.full_time": 1.25,
				"system.io.linux.pressure.some_time":     4,
				"system.io.linux.pressure.full_time":     3,
				"system.irq.linux.pressure.full_time":    0.0005,
			},
		},
		{
			name:    "irq not available",
			procDir: "proc_no_irq",
			expected: map[string]float64{
				"system.cpu.linux.pressure.some_time":    1.5,
				"system.memory.linux.pressure.some_time": 2.5,
				"system.memory.linux.pressure.full_time": 1.25,
				"system.io.linux.pressure.some_time":     4,
				"system.io.linux.pressure.full_time":     3,
			},
		},
		{
			name:    "invalid cpu file",
			procDir: "proc_invalid",
			expected: map[string]float64{
				"system.memory.linux.pressure.some_time": 2.5,
				"system.memory.linux.pressure.full_time": 1.25,
				"system.io.linux.pressure.some_time":     4,
				"system.io.linux.pressure.full_time":     3,
				"system.irq.linux.pressure.full_time":    0.0005,
			},
			expectedErr:    "invalid syntax",
			expectedFailed: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := withProcDir(t, test.procDir)

			s := newPressureScraper(scrapertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
			s.bootTime = func(context.Context) (uint64, error) { return 0, nil }
			require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

			md, err := s.scrape(ctx)
			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
				var partialErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &partialErr)
				assert.Equal(t, test.expectedFailed, partialErr.Failed)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.expected, sumValues(md))
		})
	}
}

func TestScrapeInvalidRootPath(t *testing.T) {
	ctx := withProcDir(t, "does_not_exist")

	s := newPressureScraper(scrapertest.NewNopSettings(metadata.Type), createDefaultConfig().(*Config))
	s.bootTime = func(context.Context) (uint64, error) { return 0, nil }
	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	_, err := s.scrape(ctx)
	assert.ErrorContains(t, err, "no such file or directory")
}

func withProcDir(t *testing.T, dir string) context.Context {
	return context.WithValue(t.Context(), common.EnvKey, common.EnvMap{
		common.HostProcEnvKey: filepath.Join("testdata", dir),
	})
}
