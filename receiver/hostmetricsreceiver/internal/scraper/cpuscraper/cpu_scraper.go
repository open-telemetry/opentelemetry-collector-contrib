// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

const metricsLen = 2

// scraper for CPU Metrics
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder
	ucal     *ucal.CPUUtilizationCalculator

	// for mocking
	bootTime func() (uint64, error)
	times    func(bool) ([]cpu.TimesStat, error)
	now      func() time.Time
}

// newCPUScraper creates a set of CPU related metrics
func newCPUScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config) *scraper {
	return &scraper{settings: settings, config: cfg, bootTime: host.BootTime, times: cpu.Times, ucal: &ucal.CPUUtilizationCalculator{}, now: time.Now}
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(s.now())
	cpuTimes, err := s.times( /*percpu=*/ true)
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

	return s.mb.Emit(), nil
}
