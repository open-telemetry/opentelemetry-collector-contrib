// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper/internal/metadata"
)

var metricsLength = func() int {
	n := 0
	if enableProcessesCount {
		n++
	}
	if enableProcessesCreated {
		n++
	}
	return n
}()

// scraper for Processes Metrics
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking gopsutil
	getMiscStats func(context.Context) (*load.MiscStat, error)
	getProcesses func() ([]proc, error)
	bootTime     func(context.Context) (uint64, error)
}

// for mocking out gopsutil process.Process
type proc interface {
	Status() ([]string, error)
}

type processesMetadata struct {
	countByStatus    map[metadata.AttributeStatus]int64 // ignored if enableProcessesCount is false
	processesCreated *int64                             // ignored if enableProcessesCreated is false
}

// newProcessesScraper creates a set of Processes related metrics
func newProcessesScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config) *scraper {
	return &scraper{
		settings:     settings,
		config:       cfg,
		getMiscStats: load.MiscWithContext,
		getProcesses: func() ([]proc, error) {
			ctx := context.WithValue(context.Background(), common.EnvKey, cfg.EnvMap)
			ps, err := process.ProcessesWithContext(ctx)
			ret := make([]proc, len(ps))
			for i := range ps {
				ret[i] = ps[i]
			}
			return ret, err
		},
		bootTime: host.BootTimeWithContext,
	}
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, s.config.EnvMap)
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	md := pmetric.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	metrics.EnsureCapacity(metricsLength)

	processMetadata, err := s.getProcessesMetadata()
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLength)
	}

	if enableProcessesCount && processMetadata.countByStatus != nil {
		for status, count := range processMetadata.countByStatus {
			s.mb.RecordSystemProcessesCountDataPoint(now, count, status)
		}
	}

	if enableProcessesCreated && processMetadata.processesCreated != nil {
		s.mb.RecordSystemProcessesCreatedDataPoint(now, *processMetadata.processesCreated)
	}

	return s.mb.Emit(), err
}
