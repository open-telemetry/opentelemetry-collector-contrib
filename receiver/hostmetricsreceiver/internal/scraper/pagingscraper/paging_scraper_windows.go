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

//go:build windows
// +build windows

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

const (
	pagingUsageMetricsLen = 1
	pagingMetricsLen      = 1

	memory = "Memory"

	pageReadsPerSec  = "Page Reads/sec"
	pageWritesPerSec = "Page Writes/sec"
)

// scraper for Paging Metrics
type scraper struct {
	config *Config
	mb     *metadata.MetricsBuilder

	perfCounterScraper perfcounters.PerfCounterScraper

	// for mocking
	bootTime      func() (uint64, error)
	pageFileStats func() ([]*pageFileStats, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, cfg *Config) *scraper {
	return &scraper{config: cfg, perfCounterScraper: &perfcounters.PerfLibScraper{}, bootTime: host.BootTime, pageFileStats: getPageFileStats}
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, metadata.WithStartTime(pdata.Timestamp(bootTime*1e9)))

	return s.perfCounterScraper.Initialize(memory)
}

func (s *scraper) scrape(context.Context) (pdata.Metrics, error) {
	var errors scrapererror.ScrapeErrors

	err := s.scrapePagingUsageMetric()
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapePagingOperationsMetric()
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	return s.mb.Emit(), errors.Combine()
}

func (s *scraper) scrapePagingUsageMetric() error {
	now := pdata.NewTimestampFromTime(time.Now())
	pageFiles, err := s.pageFileStats()
	if err != nil {
		return fmt.Errorf("failed to read page file stats: %w", err)
	}

	s.recordPagingUsageDataPoints(now, pageFiles)
	s.recordPagingUtilizationDataPoints(now, pageFiles)

	return nil
}

func (s *scraper) recordPagingUsageDataPoints(now pdata.Timestamp, pageFiles []*pageFileStats) {
	for _, pageFile := range pageFiles {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeState.Used)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeState.Free)
	}
}

func (s *scraper) recordPagingUtilizationDataPoints(now pdata.Timestamp, pageFiles []*pageFileStats) {
	for _, pageFile := range pageFiles {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeState.Used)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeState.Free)
	}
}

func (s *scraper) scrapePagingOperationsMetric() error {
	now := pdata.NewTimestampFromTime(time.Now())

	counters, err := s.perfCounterScraper.Scrape()
	if err != nil {
		return err
	}

	memoryObject, err := counters.GetObject(memory)
	if err != nil {
		return err
	}

	memoryCounterValues, err := memoryObject.GetValues(pageReadsPerSec, pageWritesPerSec)
	if err != nil {
		return err
	}

	if len(memoryCounterValues) > 0 {
		s.recordPagingOperationsDataPoints(now, memoryCounterValues[0])
	}
	return nil
}

func (s *scraper) recordPagingOperationsDataPoints(now pdata.Timestamp, memoryCounterValues *perfcounters.CounterValues) {
	s.mb.RecordSystemPagingOperationsDataPoint(now, memoryCounterValues.Values[pageReadsPerSec], metadata.AttributeDirection.PageIn, metadata.AttributeType.Major)
	s.mb.RecordSystemPagingOperationsDataPoint(now, memoryCounterValues.Values[pageWritesPerSec], metadata.AttributeDirection.PageOut, metadata.AttributeType.Major)
}
