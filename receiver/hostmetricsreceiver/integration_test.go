// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package hostmetricsreceiver

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"
)

// createFakeProcExeFixture creates the fake /proc/1/exe symlink used by the
// e2e process scraper tests below at test time rather than committing a
// symlink to the repo: a symlink checked into git needs to actually resolve
// for some tooling (e.g. GitHub Actions' same-repo action archive download)
// even though gopsutil only reads the link's target string, never the file
// it points to, so a dangling symlink on disk breaks tooling for no test
// benefit.
func createFakeProcExeFixture(t *testing.T) {
	t.Helper()
	link := filepath.Join("testdata", "e2e", "proc", "1", "exe")
	require.NoError(t, os.Symlink(filepath.Join("testdata", "e2e", "bin", "bash"), link))
	t.Cleanup(func() {
		require.NoError(t, os.Remove(link))
	})
}

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
				rCfg.ControllerConfig.CollectionInterval = time.Second
				f := processscraper.NewFactory()
				pCfg := f.CreateDefaultConfig().(*processscraper.Config)
				pCfg.MuteProcessExeError = true
				pCfg.Include = processscraper.MatchConfig{
					Config: filterset.Config{MatchType: filterset.Regexp},
					Names:  []string{"sleep"},
				}
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): pCfg,
				}
			},
		),
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
	createFakeProcExeFixture(t)
	expectedFile := filepath.Join("testdata", "e2e", "expected_process_separate_proc.yaml")

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rootPath := filepath.Join("testdata", "e2e")
				rCfg := cfg.(*Config)
				rCfg.ControllerConfig.CollectionInterval = time.Second
				rCfg.RootPath = rootPath
				f := processscraper.NewFactory()
				pCfg := f.CreateDefaultConfig().(*processscraper.Config)
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): pCfg,
				}
			},
		),
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
	createFakeProcExeFixture(t)
	rootPath := filepath.Join("testdata", "e2e", "proc")
	badRootPath := filepath.Join("testdata", "NOT A VALID FOLDER")
	expectedFile := filepath.Join("testdata", "e2e", "expected_process_separate_proc.yaml")

	t.Setenv("HOST_PROC", rootPath)
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ControllerConfig.CollectionInterval = time.Second
				rCfg.RootPath = badRootPath
				f := processscraper.NewFactory()
				pCfg := f.CreateDefaultConfig().(*processscraper.Config)
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): pCfg,
				}
			},
		),
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

func Test_HardwareScrape(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("hardwarescraper only supported on linux")
	}

	expectedFile := filepath.Join("testdata", "e2e", "expected_hardware.yaml")
	rootPath := filepath.Join("testdata", "e2e")

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ControllerConfig.CollectionInterval = time.Second
				rCfg.RootPath = rootPath
				f := hardwarescraper.NewFactory()
				hCfg := f.CreateDefaultConfig().(*hardwarescraper.Config)
				hCfg.Temperature.Include.Sensors = []string{".*"}
				hCfg.MetricsBuilderConfig.Metrics.HwTemperatureLimit.Enabled = true
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): hCfg,
				}
			},
		),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}

func Test_HardwareScrapeWithSensorFiltering(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("hardwarescraper only supported on linux")
	}

	expectedFile := filepath.Join("testdata", "e2e", "expected_hardware_filtered_sensors.yaml")
	rootPath := filepath.Join("testdata", "e2e")

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.ControllerConfig.CollectionInterval = time.Second
				rCfg.RootPath = rootPath
				f := hardwarescraper.NewFactory()
				hCfg := f.CreateDefaultConfig().(*hardwarescraper.Config)
				hCfg.Temperature.Include.Sensors = []string{"Composite"}
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): hCfg,
				}
			},
		),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
