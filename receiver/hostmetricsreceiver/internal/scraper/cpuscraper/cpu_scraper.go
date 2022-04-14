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

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
)

const metricsLen = 2

// scraper for CPU Metrics
type scraper struct {
	config *Config
	mb     *metadata.MetricsBuilder
	ucal   *ucal.CPUUtilizationCalculator

	// for mocking
	bootTime func() (uint64, error)
	times    func(bool) ([]cpu.TimesStat, error)
	now      func() time.Time
}

// newCPUScraper creates a set of CPU related metrics
func newCPUScraper(_ context.Context, cfg *Config) *scraper {
	return &scraper{config: cfg, bootTime: host.BootTime, times: cpu.Times, ucal: &ucal.CPUUtilizationCalculator{}, now: time.Now}
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
	now := pcommon.NewTimestampFromTime(s.now())
	cpuTimes, err := s.times( /*percpu=*/ true)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	for _, cpuTime := range cpuTimes {
		s.recordCPUTimeStateDataPoints(now, cpuTime)
	}

	err = s.ucal.CalculateAndRecord(now, cpuTimes, s.recordCPUUtilization)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	return s.mb.Emit(), nil
}
