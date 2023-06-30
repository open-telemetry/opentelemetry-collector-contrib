// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

const metricsLen = 2

var ErrInvalidTotalMem = errors.New("invalid total memory")

// scraper for Memory Metrics
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder
	envMap   common.EnvMap

	// for mocking gopsutil mem.VirtualMemory
	bootTime      func(context.Context) (uint64, error)
	virtualMemory func(context.Context) (*mem.VirtualMemoryStat, error)
}

// newMemoryScraper creates a Memory Scraper
func newMemoryScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config, envMap common.EnvMap) *scraper {
	return &scraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, virtualMemory: mem.VirtualMemoryWithContext, envMap: envMap}
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, s.envMap)
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	ctx = context.WithValue(ctx, common.EnvKey, s.envMap)

	now := pcommon.NewTimestampFromTime(time.Now())
	memInfo, err := s.virtualMemory(ctx)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if memInfo != nil {
		s.recordMemoryUsageMetric(now, memInfo)
		if memInfo.Total <= 0 {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(fmt.Errorf("%w: %d", ErrInvalidTotalMem,
				memInfo.Total), metricsLen)
		}
		s.recordMemoryUtilizationMetric(now, memInfo)
	}

	return s.mb.Emit(), nil
}
