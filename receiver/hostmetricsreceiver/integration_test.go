// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

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
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				pCfg := (&processscraper.Factory{}).CreateDefaultConfig().(*processscraper.Config)
				pCfg.MuteProcessExeError = true
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
			pmetrictest.IgnoreResourceAttributeValue("process.owner"),
			pmetrictest.IgnoreResourceAttributeValue("process.parent_pid"),
			pmetrictest.IgnoreResourceAttributeValue("process.pid"),
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
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rootPath := filepath.Join("testdata", "e2e")
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				rCfg.RootPath = rootPath
				pCfg := (&processscraper.Factory{}).CreateDefaultConfig().(*processscraper.Config)
				pCfg.SetRootPath(rootPath)
				pCfg.SetEnvMap(setGoPsutilEnvVars(rootPath, &osEnv{}))
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

func Test_ProcessScrapeWithBadRootPathAndEnvVar(t *testing.T) {
	rootPath := filepath.Join("testdata", "e2e", "proc")
	badRootPath := filepath.Join("testdata", "NOT A VALID FOLDER")
	expectedFile := filepath.Join("testdata", "e2e", "expected_process_separate_proc.yaml")

	t.Setenv("HOST_PROC", rootPath)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				pCfg := (&processscraper.Factory{}).CreateDefaultConfig().(*processscraper.Config)
				pCfg.SetRootPath(badRootPath)
				pCfg.SetEnvMap(setGoPsutilEnvVars(badRootPath, &osEnv{}))
				rCfg.Scrapers = map[string]internal.Config{
					"process": pCfg,
				}
				rCfg.RootPath = badRootPath
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
