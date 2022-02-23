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

//go:build linux
// +build linux

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

func (s *scraper) recordMemoryUsageMetric(now pdata.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Used), metadata.AttributeState.Used)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Free), metadata.AttributeState.Free)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Buffers), metadata.AttributeState.Buffered)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Cached), metadata.AttributeState.Cached)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Sreclaimable), metadata.AttributeState.SlabReclaimable)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Sunreclaim), metadata.AttributeState.SlabUnreclaimable)
}

func (s *scraper) recordMemoryUtilizationMetric(now pdata.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Used)/float64(memInfo.Total), metadata.AttributeState.Used)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Free)/float64(memInfo.Total), metadata.AttributeState.Free)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Buffers)/float64(memInfo.Total), metadata.AttributeState.Buffered)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Cached)/float64(memInfo.Total), metadata.AttributeState.Cached)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Sreclaimable)/float64(memInfo.Total), metadata.AttributeState.SlabReclaimable)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Sunreclaim)/float64(memInfo.Total), metadata.AttributeState.SlabUnreclaimable)
}
