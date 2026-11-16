// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pressurescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper"

import (
	"context"
	"errors"
	"io/fs"
	"syscall"
	"time"

	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v4/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

type pressureScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder
	psiStats func(ctx context.Context, resource string) (procfs.PSIStats, error)
}

func newPressureScraper(settings scraper.Settings, cfg *Config) *pressureScraper {
	return &pressureScraper{
		settings: settings,
		config:   cfg,
		psiStats: psiStatsWithContext,
	}
}

func (s *pressureScraper) start(context.Context, component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *pressureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs scrapererror.ScrapeErrors
	now := pcommon.NewTimestampFromTime(time.Now())

	if psi, err := s.psiStats(ctx, "cpu"); err != nil {
		errs.AddPartial(1, err)
	} else if psi.Some != nil {
		s.mb.RecordSystemCPULinuxPressureSomeTimeDataPoint(now, toSeconds(psi.Some))
	}

	if psi, err := s.psiStats(ctx, "memory"); err != nil {
		errs.AddPartial(2, err)
	} else {
		if psi.Some != nil {
			s.mb.RecordSystemMemoryLinuxPressureSomeTimeDataPoint(now, toSeconds(psi.Some))
		}
		if psi.Full != nil {
			s.mb.RecordSystemMemoryLinuxPressureFullTimeDataPoint(now, toSeconds(psi.Full))
		}
	}

	if psi, err := s.psiStats(ctx, "io"); err != nil {
		errs.AddPartial(2, err)
	} else {
		if psi.Some != nil {
			s.mb.RecordSystemIoLinuxPressureSomeTimeDataPoint(now, toSeconds(psi.Some))
		}
		if psi.Full != nil {
			s.mb.RecordSystemIoLinuxPressureFullTimeDataPoint(now, toSeconds(psi.Full))
		}
	}

	if psi, err := s.psiStats(ctx, "irq"); err != nil {
		if !irqUnavailable(err) {
			errs.AddPartial(1, err)
		}
	} else if psi.Full != nil {
		s.mb.RecordSystemIrqLinuxPressureFullTimeDataPoint(now, toSeconds(psi.Full))
	}

	return s.mb.Emit(), errs.Combine()
}

func psiStatsWithContext(ctx context.Context, resource string) (procfs.PSIStats, error) {
	procFS, err := procfs.NewFS(gopsutilenv.GetEnvWithContext(ctx, string(common.HostProcEnvKey), "/proc"))
	if err != nil {
		return procfs.PSIStats{}, err
	}
	return procFS.PSIStatsForResource(resource)
}

// irqUnavailable reports whether the kernel does not provide IRQ pressure: the file is missing
// without CONFIG_IRQ_TIME_ACCOUNTING, and since Linux 6.14 reading it returns EOPNOTSUPP while
// IRQ time accounting is disabled at runtime.
func irqUnavailable(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.EOPNOTSUPP)
}

func toSeconds(line *procfs.PSILine) float64 {
	return float64(line.Total) / 1e6
}
