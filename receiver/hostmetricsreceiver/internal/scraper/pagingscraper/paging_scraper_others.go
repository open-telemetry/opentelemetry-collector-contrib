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
	pagingUsageMetricsLen = 1
	pagingMetricsLen      = 2
)

// scraper for Paging Metrics
type scraper struct {
	config    *Config
	startTime pdata.Timestamp

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

	s.startTime = pdata.Timestamp(bootTime * 1e9)
	return nil
}

func (s *scraper) scrape(_ context.Context) (pdata.Metrics, error) {
	md := pdata.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics()

	var errors scrapererror.ScrapeErrors

	err := s.scrapeAndAppendPagingUsageMetric(metrics)
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapeAndAppendPagingMetrics(metrics)
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	return md, errors.Combine()
}

func (s *scraper) scrapeAndAppendPagingUsageMetric(metrics pdata.MetricSlice) error {
	now := pdata.NewTimestampFromTime(time.Now())
	pageFileStats, err := s.getPageFileStats()
	if err != nil {
		return err
	}

	idx := metrics.Len()
	metrics.EnsureCapacity(idx + pagingUsageMetricsLen)
	initializePagingUsageMetric(metrics.AppendEmpty(), now, pageFileStats)
	return nil
}

func initializePagingUsageMetric(metric pdata.Metric, now pdata.Timestamp, pageFileStats []*pageFileStats) {
	metadata.Metrics.SystemPagingUsage.Init(metric)

	idps := metric.Sum().DataPoints()
	idps.EnsureCapacity(3)
	for _, pageFile := range pageFileStats {
		initializePagingUsageDataPoint(idps.AppendEmpty(), now, pageFile.deviceName, metadata.AttributeState.Used, int64(pageFile.usedBytes))
		initializePagingUsageDataPoint(idps.AppendEmpty(), now, pageFile.deviceName, metadata.AttributeState.Free, int64(pageFile.freeBytes))
		if pageFile.cachedBytes != nil {
			initializePagingUsageDataPoint(idps.AppendEmpty(), now, pageFile.deviceName, metadata.AttributeState.Cached, int64(*pageFile.cachedBytes))
		}
	}
}

func initializePagingUsageDataPoint(dataPoint pdata.NumberDataPoint, now pdata.Timestamp, deviceLabel, stateLabel string, value int64) {
	if deviceLabel != "" {
		dataPoint.Attributes().InsertString(metadata.Attributes.Device, deviceLabel)
	}
	dataPoint.Attributes().InsertString(metadata.Attributes.State, stateLabel)
	dataPoint.SetTimestamp(now)
	dataPoint.SetIntVal(value)
}

func (s *scraper) scrapeAndAppendPagingMetrics(metrics pdata.MetricSlice) error {
	now := pdata.NewTimestampFromTime(time.Now())
	swap, err := s.swapMemory()
	if err != nil {
		return err
	}

	idx := metrics.Len()
	metrics.EnsureCapacity(idx + pagingMetricsLen)
	initializePagingOperationsMetric(metrics.AppendEmpty(), s.startTime, now, swap)
	initializePageFaultsMetric(metrics.AppendEmpty(), s.startTime, now, swap)
	return nil
}

func initializePagingOperationsMetric(metric pdata.Metric, startTime, now pdata.Timestamp, swap *mem.SwapMemoryStat) {
	metadata.Metrics.SystemPagingOperations.Init(metric)

	idps := metric.Sum().DataPoints()
	idps.EnsureCapacity(4)
	initializePagingOperationsDataPoint(idps.AppendEmpty(), startTime, now, metadata.AttributeType.Major, metadata.AttributeDirection.PageIn, int64(swap.Sin))
	initializePagingOperationsDataPoint(idps.AppendEmpty(), startTime, now, metadata.AttributeType.Major, metadata.AttributeDirection.PageOut, int64(swap.Sout))
	initializePagingOperationsDataPoint(idps.AppendEmpty(), startTime, now, metadata.AttributeType.Minor, metadata.AttributeDirection.PageIn, int64(swap.PgIn))
	initializePagingOperationsDataPoint(idps.AppendEmpty(), startTime, now, metadata.AttributeType.Minor, metadata.AttributeDirection.PageOut, int64(swap.PgOut))
}

func initializePagingOperationsDataPoint(dataPoint pdata.NumberDataPoint, startTime, now pdata.Timestamp, typeLabel string, directionLabel string, value int64) {
	attributes := dataPoint.Attributes()
	attributes.InsertString(metadata.Attributes.Type, typeLabel)
	attributes.InsertString(metadata.Attributes.Direction, directionLabel)
	dataPoint.SetStartTimestamp(startTime)
	dataPoint.SetTimestamp(now)
	dataPoint.SetIntVal(value)
}

func initializePageFaultsMetric(metric pdata.Metric, startTime, now pdata.Timestamp, swap *mem.SwapMemoryStat) {
	metadata.Metrics.SystemPagingFaults.Init(metric)

	idps := metric.Sum().DataPoints()
	idps.EnsureCapacity(2)
	initializePageFaultDataPoint(idps.AppendEmpty(), startTime, now, metadata.AttributeType.Major, int64(swap.PgMajFault))
	initializePageFaultDataPoint(idps.AppendEmpty(), startTime, now, metadata.AttributeType.Minor, int64(swap.PgFault-swap.PgMajFault))
}

func initializePageFaultDataPoint(dataPoint pdata.NumberDataPoint, startTime, now pdata.Timestamp, typeLabel string, value int64) {
	dataPoint.Attributes().InsertString(metadata.Attributes.Type, typeLabel)
	dataPoint.SetStartTimestamp(startTime)
	dataPoint.SetTimestamp(now)
	dataPoint.SetIntVal(value)
}
