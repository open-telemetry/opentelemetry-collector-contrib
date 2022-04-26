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

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

const metricsLen = 2

var ErrInvalidTotalMem = errors.New("invalid total memory")

// scraper for Memory Metrics
type scraper struct {
	config *Config
	mb     *metadata.MetricsBuilder

	// for mocking gopsutil mem.VirtualMemory
	bootTime      func() (uint64, error)
	virtualMemory func() (*mem.VirtualMemoryStat, error)
}

// newMemoryScraper creates a Memory Scraper
func newMemoryScraper(_ context.Context, cfg *Config) *scraper {
	return &scraper{config: cfg, bootTime: host.BootTime, virtualMemory: mem.VirtualMemory}
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	memInfo, err := s.virtualMemory()
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
