// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

const (
	pagingUsageMetricsLen = 2
	pagingMetricsLen      = 2
)

// scraper for Paging Metrics
type scraper struct {
	config *Config
	mb     *metadata.MetricsBuilder

	// for mocking
	bootTime         func() (uint64, error)
	getPageFileStats func() ([]*pageFileStats, error)
	swapMemory       func() (*mem.SwapMemoryStat, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, cfg *Config) *scraper {
	return &scraper{config: cfg, bootTime: host.BootTime, getPageFileStats: getPageFileStats, swapMemory: mem.SwapMemory}
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, metadata.WithStartTime(pdata.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pdata.Metrics, error) {
	md := pdata.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics()

	var errors scrapererror.ScrapeErrors

	err := s.scrapePagingUsageMetric()
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapePagingMetrics()
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	s.mb.Emit(metrics)
	return md, errors.Combine()
}

func (s *scraper) scrapePagingUsageMetric() error {
	now := pdata.NewTimestampFromTime(time.Now())
	pageFileStats, err := s.getPageFileStats()
	if err != nil {
		return err
	}

	s.recordPagingUsageDataPoints(now, pageFileStats)
	s.recordPagingUtilizationDataPoints(now, pageFileStats)
	return nil
}

func (s *scraper) recordPagingUsageDataPoints(now pdata.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeState.Used)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeState.Free)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUsageDataPoint(now, int64(*pageFile.cachedBytes), pageFile.deviceName, metadata.AttributeState.Cached)
		}
	}
}

func (s *scraper) recordPagingUtilizationDataPoints(now pdata.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeState.Used)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeState.Free)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(*pageFile.cachedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeState.Cached)
		}
	}
}

func (s *scraper) scrapePagingMetrics() error {
	now := pdata.NewTimestampFromTime(time.Now())
	swap, err := s.swapMemory()
	if err != nil {
		return err
	}

	s.recordPagingOperationsDataPoints(now, swap)
	s.recordPageFaultsDataPoints(now, swap)
	return nil
}

func (s *scraper) recordPagingOperationsDataPoints(now pdata.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sin), metadata.AttributeDirection.PageIn, metadata.AttributeType.Major)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sout), metadata.AttributeDirection.PageOut, metadata.AttributeType.Major)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgIn), metadata.AttributeDirection.PageIn, metadata.AttributeType.Minor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgOut), metadata.AttributeDirection.PageOut, metadata.AttributeType.Minor)
}

func (s *scraper) recordPageFaultsDataPoints(now pdata.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgMajFault), metadata.AttributeType.Major)
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgFault-swap.PgMajFault), metadata.AttributeType.Minor)
}
