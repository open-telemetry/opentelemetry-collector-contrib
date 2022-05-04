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

package processesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	config *Config
	mb     *metadata.MetricsBuilder

	// for mocking gopsutil
	getMiscStats func() (*load.MiscStat, error)
	getProcesses func() ([]proc, error)
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
func newProcessesScraper(_ context.Context, cfg *Config) *scraper {
	return &scraper{
		config:       cfg,
		getMiscStats: load.Misc,
		getProcesses: func() ([]proc, error) {
			ps, err := process.Processes()
			ret := make([]proc, len(ps))
			for i := range ps {
				ret[i] = ps[i]
			}
			return ret, err
		},
	}
}

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := host.BootTime()
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.Metrics, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
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
