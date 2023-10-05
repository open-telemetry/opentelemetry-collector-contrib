// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package hostmetricsreceiver

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"
)

func Test_ProcessScrape(t *testing.T) {
	expectedFile := filepath.Join("testdata", "e2e", "expected_process.yaml")
	cmd := exec.Command("/bin/sleep", "300")
	require.NoError(t, cmd.Start())
	defer func() {
		require.NoError(t, cmd.Process.Kill())
	}()

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				pCfg := (&processscraper.Factory{}).CreateDefaultConfig().(*processscraper.Config)
				pCfg.Include = processscraper.MatchConfig{
					Config: filterset.Config{MatchType: filterset.Regexp},
					Names:  []string{"sleep"},
				}
				rCfg.Scrapers = map[string]internal.Config{
					"process": pCfg,
				}
			}),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)

}

func Test_ProcessScrapeWithCustomRootPath(t *testing.T) {
	expectedFile := filepath.Join("testdata", "e2e", "expected_process_separate_proc.yaml")

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				pCfg := (&processscraper.Factory{}).CreateDefaultConfig().(*processscraper.Config)
				pCfg.Include = processscraper.MatchConfig{
					Config: filterset.Config{MatchType: filterset.Regexp},
					Names:  []string{"sleep"},
				}
				rCfg.Scrapers = map[string]internal.Config{
					"process": pCfg,
				}
				rCfg.RootPath = filepath.Join("testdata", "e2e")
			}),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)

}

func Test_ProcessScrapeWithBadRootPathAndEnvVar(t *testing.T) {
	expectedFile := filepath.Join("testdata", "e2e", "expected_process_separate_proc.yaml")
	t.Setenv("HOST_PROC", filepath.Join("testdata", "e2e", "proc"))
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				pCfg := (&processscraper.Factory{}).CreateDefaultConfig().(*processscraper.Config)
				pCfg.Include = processscraper.MatchConfig{
					Config: filterset.Config{MatchType: filterset.Regexp},
					Names:  []string{"sleep"},
				}
				rCfg.Scrapers = map[string]internal.Config{
					"process": pCfg,
				}
				rCfg.RootPath = filepath.Join("testdata", "NOT A VALID FOLDER")
			}),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)

}
