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
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/service/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

const (
	pagingUsageMetricsLen = 2
	pagingMetricsLen      = 2
)

// scraper for Paging Metrics
type scraper struct {
	settings component.ReceiverCreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking
	bootTime         func() (uint64, error)
	getPageFileStats func() ([]*pageFileStats, error)
	swapMemory       func() (*mem.SwapMemoryStat, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, settings component.ReceiverCreateSettings, cfg *Config) *scraper {
	return &scraper{settings: settings, config: cfg, bootTime: host.BootTime, getPageFileStats: getPageFileStats, swapMemory: mem.SwapMemory}
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, s.settings.BuildInfo, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errors scrapererror.ScrapeErrors

	err := s.scrapePagingUsageMetric()
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapePagingMetrics()
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	return s.mb.Emit(), errors.Combine()
}

func (s *scraper) scrapePagingUsageMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())
	pageFileStats, err := s.getPageFileStats()
	if err != nil {
		return fmt.Errorf("failed to read page file stats: %w", err)
	}

	s.recordPagingUsageDataPoints(now, pageFileStats)
	s.recordPagingUtilizationDataPoints(now, pageFileStats)
	return nil
}

func (s *scraper) recordPagingUsageDataPoints(now pcommon.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeStateFree)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUsageDataPoint(now, int64(*pageFile.cachedBytes), pageFile.deviceName, metadata.AttributeStateCached)
		}
	}
}

func (s *scraper) recordPagingUtilizationDataPoints(now pcommon.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateFree)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(*pageFile.cachedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateCached)
		}
	}
}

func (s *scraper) scrapePagingMetrics() error {
	now := pcommon.NewTimestampFromTime(time.Now())
	swap, err := s.swapMemory()
	if err != nil {
		return fmt.Errorf("failed to read swap info: %w", err)
	}

	s.recordPagingOperationsDataPoints(now, swap)
	s.recordPageFaultsDataPoints(now, swap)
	return nil
}

func (s *scraper) recordPagingOperationsDataPoints(now pcommon.Timestamp, swap *mem.SwapMemoryStat) {
	if featuregate.GetRegistry().IsEnabled(removeDirectionAttributeFeatureGateID) {
		s.mb.RecordSystemPagingOperationsPageInDataPoint(now, int64(swap.Sin), metadata.AttributeTypeMajor)
		s.mb.RecordSystemPagingOperationsPageOutDataPoint(now, int64(swap.Sout), metadata.AttributeTypeMajor)
		s.mb.RecordSystemPagingOperationsPageInDataPoint(now, int64(swap.PgIn), metadata.AttributeTypeMinor)
		s.mb.RecordSystemPagingOperationsPageOutDataPoint(now, int64(swap.PgOut), metadata.AttributeTypeMinor)
	} else {

		s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sin), metadata.AttributeDirectionPageIn, metadata.AttributeTypeMajor)
		s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sout), metadata.AttributeDirectionPageOut, metadata.AttributeTypeMajor)
		s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgIn), metadata.AttributeDirectionPageIn, metadata.AttributeTypeMinor)
		s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgOut), metadata.AttributeDirectionPageOut, metadata.AttributeTypeMinor)
	}
}

func (s *scraper) recordPageFaultsDataPoints(now pcommon.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgMajFault), metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgFault-swap.PgMajFault), metadata.AttributeTypeMinor)
}
