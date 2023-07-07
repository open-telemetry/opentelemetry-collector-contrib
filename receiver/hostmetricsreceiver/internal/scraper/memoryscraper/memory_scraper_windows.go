// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

func (s *scraper) recordMemoryUsageMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Used), metadata.AttributeStateUsed)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Available), metadata.AttributeStateFree)
}

func (s *scraper) recordMemoryUtilizationMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Used)/float64(memInfo.Total), metadata.AttributeStateUsed)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Available)/float64(memInfo.Total), metadata.AttributeStateFree)
}
