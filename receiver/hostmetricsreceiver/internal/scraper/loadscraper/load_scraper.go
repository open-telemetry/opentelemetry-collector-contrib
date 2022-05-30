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

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

const metricsLen = 3

// scraper for Load Metrics
type scraper struct {
	settings component.ReceiverCreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking
	bootTime func() (uint64, error)
	load     func() (*load.AvgStat, error)
}

// newLoadScraper creates a set of Load related metrics
func newLoadScraper(_ context.Context, settings component.ReceiverCreateSettings, cfg *Config) *scraper {
	return &scraper{settings: settings, config: cfg, bootTime: host.BootTime, load: getSampledLoadAverages}
}

// start
func (s *scraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, s.settings.BuildInfo, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return startSampling(ctx, s.settings.Logger)
}

// shutdown
func (s *scraper) shutdown(ctx context.Context) error {
	return stopSampling(ctx)
}

// scrape
func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	avgLoadValues, err := s.load()
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if s.config.CPUAverage {
		divisor := float64(runtime.NumCPU())
		avgLoadValues.Load1 = avgLoadValues.Load1 / divisor
		avgLoadValues.Load5 = avgLoadValues.Load1 / divisor
		avgLoadValues.Load15 = avgLoadValues.Load1 / divisor
	}

	s.mb.RecordSystemCPULoadAverage1mDataPoint(now, avgLoadValues.Load1)
	s.mb.RecordSystemCPULoadAverage5mDataPoint(now, avgLoadValues.Load5)
	s.mb.RecordSystemCPULoadAverage15mDataPoint(now, avgLoadValues.Load15)

	return s.mb.Emit(), nil
}
