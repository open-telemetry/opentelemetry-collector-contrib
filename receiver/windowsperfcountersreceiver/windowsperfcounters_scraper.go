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

package windowsperfcountersreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"

	windowsapi "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon"
)

const instanceLabelName = "instance"

// scraper is the type that scrapes various host metrics.
type scraper struct {
	cfg             *Config
	settings        component.TelemetrySettings
	counterScrapers []windowsapi.Scraper
}

func newScraper(cfg *Config, settings component.TelemetrySettings) *scraper {
	return &scraper{cfg: cfg, settings: settings}
}

func (s *scraper) start(context.Context, component.Host) error {
	scrapercfg := []windowsapi.ScraperCfg{}
	for _, perfCounter := range s.cfg.PerfCounters {
		scrapercfg = append(scrapercfg, windowsapi.ScraperCfg{CounterCfg: perfCounter})
	}

	s.counterScrapers = windowsapi.BuildPaths(scrapercfg, s.settings.Logger)
	return nil
}

func (s *scraper) shutdown(context.Context) error {
	return windowsapi.CloseCounters(s.counterScrapers)
}

func (s *scraper) scrape(context.Context) (pdata.Metrics, error) {
	md := pdata.NewMetrics()
	metricSlice := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	now := pdata.NewTimestampFromTime(time.Now())
	var errs error

	metricSlice.EnsureCapacity(len(s.counterScrapers))
	metrics := map[string]pdata.Metric{}
	for name, metricCfg := range s.cfg.MetricMetaData {
		builtMetric := metricSlice.AppendEmpty()

		builtMetric.SetName(name)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		if (metricCfg.Sum != SumMetric{}) {
			builtMetric.SetDataType(pdata.MetricDataTypeSum)
			builtMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
			}
		} else {
			builtMetric.SetDataType(pdata.MetricDataTypeGauge)
		}

		metrics[name] = builtMetric
	}

	scrapedMetrics, errs := windowsapi.ScrapeCounters(s.counterScrapers)

	for _, scrapedValue := range scrapedMetrics {
		var metric pdata.Metric
		if builtmetric, ok := metrics[scrapedValue.Metric.Name]; ok {
			metric = builtmetric
		} else {
			metric = metricSlice.AppendEmpty()
			metric.SetDataType(pdata.MetricDataTypeGauge)
			metric.SetName(scrapedValue.Metric.Name)
			metric.SetUnit("1")
		}

		initializeMetricDps(metric, now, scrapedValue.Value, scrapedValue.Metric.Attributes)
	}

	return md, errs
}

func initializeMetricDps(metric pdata.Metric, now pdata.Timestamp, counterValue int64, attributes map[string]string) {
	var dps pdata.NumberDataPointSlice

	if metric.DataType() == pdata.MetricDataTypeGauge {
		dps = metric.Gauge().DataPoints()
	} else {
		dps = metric.Sum().DataPoints()
	}

	dp := dps.AppendEmpty()
	if attributes != nil {
		for attKey, attVal := range attributes {
			dp.Attributes().InsertString(attKey, attVal)
		}
	}

	dp.SetTimestamp(now)
	dp.SetIntVal(counterValue)
}
