// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

const (
	metricsLen = 2
	hzInAMHz   = 1_000_000
)

// cpuScraper for CPU Metrics
type cpuScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder
	ucal     *ucal.CPUUtilizationCalculator

	// for mocking
	bootTime func(context.Context) (uint64, error)
	times    func(context.Context, bool) ([]cpu.TimesStat, error)
	now      func() time.Time
}

type cpuInfo struct {
	frequency float64
	processor uint
}

// newCPUScraper creates a set of CPU related metrics
func newCPUScraper(_ context.Context, settings scraper.Settings, cfg *Config) *cpuScraper {
	return &cpuScraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, times: cpu.TimesWithContext, ucal: &ucal.CPUUtilizationCalculator{}, now: time.Now}
}

func (s *cpuScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *cpuScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(s.now())
	cpuTimes, err := s.times(ctx, true /*percpu=*/)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	for _, cpuTime := range cpuTimes {
		s.recordCPUTimeStateDataPoints(now, cpuTime)
	}

	err = s.ucal.CalculateAndRecord(now, cpuTimes, s.recordCPUUtilization)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if s.config.Metrics.SystemCPUPhysicalCount.Enabled {
		numCPU, err := cpu.Counts(false)
		if err != nil {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
		}
		s.mb.RecordSystemCPUPhysicalCountDataPoint(now, int64(numCPU))
	}

	if s.config.Metrics.SystemCPULogicalCount.Enabled {
		numCPU, err := cpu.Counts(true)
		if err != nil {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
		}
		s.mb.RecordSystemCPULogicalCountDataPoint(now, int64(numCPU))
	}

	if s.config.Metrics.SystemCPUFrequency.Enabled {
		cpuInfos, err := s.getCPUInfo()
		if err != nil {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
		}
		for _, cInfo := range cpuInfos {
			s.mb.RecordSystemCPUFrequencyDataPoint(now, cInfo.frequency*hzInAMHz, fmt.Sprintf("cpu%d", cInfo.processor))
		}
	}

	return s.mb.Emit(), nil
}
