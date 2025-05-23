// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

const metricsLen = 3

// errPreventScrape is used to indicate that skip scrape should be set to true
// when the sampler fails to start.
var errPreventScrape = errors.New("cannot scrape load metrics")

// scraper for Load Metrics
type loadScraper struct {
	settings   scraper.Settings
	config     *Config
	mb         *metadata.MetricsBuilder
	skipScrape bool

	// for mocking
	bootTime func(context.Context) (uint64, error)
	load     func(context.Context) (*load.AvgStat, error)
}

// newLoadScraper creates a set of Load related metrics
func newLoadScraper(_ context.Context, settings scraper.Settings, cfg *Config) *loadScraper {
	return &loadScraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, load: getSampledLoadAverages}
}

// start
func (s *loadScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	err = startSampling(ctx, s.settings.Logger)
	if errors.Is(err, errPreventScrape) {
		s.settings.Logger.Error("failed to start load scraper sampler", zap.Error(err))
		s.skipScrape = true
		err = nil
	}

	return err
}

// shutdown
func (s *loadScraper) shutdown(ctx context.Context) error {
	if s.skipScrape {
		// We skipped scraping because the sampler failed to start,
		// so it doesn't need to be shut down.
		return nil
	}
	return stopSampling(ctx)
}

// scrape
func (s *loadScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.skipScrape {
		return pmetric.NewMetrics(), nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())

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
