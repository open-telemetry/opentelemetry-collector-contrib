// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package hostmetricsreceiver

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
				f := processscraper.NewFactory()
				pCfg := f.CreateDefaultConfig().(*processscraper.Config)
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): pCfg,
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
				rCfg.RootPath = badRootPath
				f := processscraper.NewFactory()
				pCfg := f.CreateDefaultConfig().(*processscraper.Config)
				rCfg.Scrapers = map[component.Type]component.Config{
					f.Type(): pCfg,
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

func Test_Windows_ProcessScrapeWMIInformation(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("this integration test is windows exclusive")
	}

	factory := NewFactory()

	rCfg := &Config{}
	rCfg.CollectionInterval = time.Second
	f := processscraper.NewFactory()
	pCfg := f.CreateDefaultConfig().(*processscraper.Config)
	pCfg.Metrics.ProcessHandles.Enabled = true
	pCfg.ResourceAttributes.ProcessParentPid.Enabled = true
	rCfg.Scrapers = map[component.Type]component.Config{
		f.Type(): pCfg,
	}
	cfg := component.Config(rCfg)

	sink := new(consumertest.MetricsSink)
	r, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(factory.Type()), cfg, sink)
	require.NoError(t, err)

	// Start the receiver and give it an extra 250 milliseconds after the
	// collection interval to perform the scrape.
	r.Start(context.Background(), componenttest.NewNopHost())
	time.Sleep(rCfg.CollectionInterval + 250*time.Millisecond)
	r.Shutdown(context.Background())

	// The actual results of the test are non-deterministic, but
	// all we want to know is whether the handles and parent PID
	// metrics are being retrieved successfully on at least some
	// metrics (it doesn't need to work for every process).
	metrics := sink.AllMetrics()
	var foundValidParentPid, foundValidHandles int
	for _, m := range metrics {
		rms := m.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			rm := rms.At(i)

			// Check if the resource attributes has the parent PID.
			ppid, ok := rm.Resource().Attributes().Get("process.parent_pid")
			if ok && ppid.Int() > 0 {
				foundValidParentPid++
			}

			sms := rm.ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				sm := sms.At(j)
				ms := sm.Metrics()
				for k := 0; k < ms.Len(); k++ {
					m := ms.At(k)

					// Check if this is a process.handles metric
					// with a non-zero datapoint.
					if m.Name() == "process.handles" &&
						m.Sum().DataPoints().Len() > 0 &&
						m.Sum().DataPoints().At(0).IntValue() > 0 {
						foundValidHandles++
					}
				}
			}
		}
	}

	require.Positive(t, foundValidHandles)
	require.Positive(t, foundValidParentPid)
}
