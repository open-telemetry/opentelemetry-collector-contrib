// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

const metricsLen = 3

// scraper for Load Metrics
type scraper struct {
	settings   receiver.CreateSettings
	config     *Config
	mb         *metadata.MetricsBuilder
	skipScrape bool
	envMap     common.EnvMap

	// for mocking
	bootTime func(context.Context) (uint64, error)
	load     func(context.Context) (*load.AvgStat, error)
}

// newLoadScraper creates a set of Load related metrics
func newLoadScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config, envMap common.EnvMap) *scraper {
	return &scraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, load: getSampledLoadAverages, envMap: envMap}
}

// start
func (s *scraper) start(ctx context.Context, _ component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, s.envMap)
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	err = startSampling(ctx, s.settings.Logger)

	var initErr *perfcounters.PerfCounterInitError
	switch {
	case errors.As(err, &initErr):
		// This indicates, on Windows, that the performance counters can't be scraped.
		// In order to prevent crashing in a fragile manner, we simply skip scraping.
		s.settings.Logger.Error("Failed to init performance counters, load metrics will not be scraped", zap.Error(err))
		s.skipScrape = true
	case err != nil:
		// Unknown error; fail to start if this is the case
		return err
	}

	return nil
}

// shutdown
func (s *scraper) shutdown(ctx context.Context) error {
	if s.skipScrape {
		// We skipped scraping because the sampler failed to start,
		// so it doesn't need to be shut down.
		return nil
	}
	return stopSampling(ctx)
}

// scrape
func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.skipScrape {
		return pmetric.NewMetrics(), nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	ctx = context.WithValue(ctx, common.EnvKey, s.envMap)

	avgLoadValues, err := s.load(ctx)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if s.config.CPUAverage {
		divisor := float64(runtime.NumCPU())
		avgLoadValues.Load1 /= divisor
		avgLoadValues.Load5 /= divisor
		avgLoadValues.Load15 /= divisor
	}

	s.mb.RecordSystemCPULoadAverage1mDataPoint(now, avgLoadValues.Load1)
	s.mb.RecordSystemCPULoadAverage5mDataPoint(now, avgLoadValues.Load5)
	s.mb.RecordSystemCPULoadAverage15mDataPoint(now, avgLoadValues.Load15)

	return s.mb.Emit(), nil
}
