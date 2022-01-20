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
	"time"

	"github.com/shirou/gopsutil/v3/load"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

const metricsLen = 3

// scraper for Load Metrics
type scraper struct {
	logger *zap.Logger
	config *Config

	// for mocking
	load func() (*load.AvgStat, error)
}

// newLoadScraper creates a set of Load related metrics
func newLoadScraper(_ context.Context, logger *zap.Logger, cfg *Config) *scraper {
	return &scraper{logger: logger, config: cfg, load: getSampledLoadAverages}
}

// start
func (s *scraper) start(ctx context.Context, _ component.Host) error {
	return startSampling(ctx, s.logger)
}

// shutdown
func (s *scraper) shutdown(ctx context.Context) error {
	return stopSampling(ctx)
}

// scrape
func (s *scraper) scrape(_ context.Context) (pdata.Metrics, error) {
	md := pdata.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics()

	now := pdata.NewTimestampFromTime(time.Now())
	avgLoadValues, err := s.load()
	if err != nil {
		return md, scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	metrics.EnsureCapacity(metricsLen)

	initializeLoadMetric(metrics.AppendEmpty(), metadata.Metrics.SystemCPULoadAverage1m, now, avgLoadValues.Load1)
	initializeLoadMetric(metrics.AppendEmpty(), metadata.Metrics.SystemCPULoadAverage5m, now, avgLoadValues.Load5)
	initializeLoadMetric(metrics.AppendEmpty(), metadata.Metrics.SystemCPULoadAverage15m, now, avgLoadValues.Load15)
	return md, nil
}

func initializeLoadMetric(metric pdata.Metric, metricDescriptor metadata.MetricIntf, now pdata.Timestamp, value float64) {
	metricDescriptor.Init(metric)
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetDoubleVal(value)
}
