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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

const instanceLabelName = "instance"

// scraper is the type that scrapes various host metrics.
type scraper struct {
	cfg      *Config
	settings component.TelemetrySettings
	watchers []winperfcounters.PerfCounterWatcher
}

func newScraper(cfg *Config, settings component.TelemetrySettings) *scraper {
	return &scraper{cfg: cfg, settings: settings}
}

func (s *scraper) start(context.Context, component.Host) error {
	watchers := []winperfcounters.PerfCounterWatcher{}
	for _, objCfg := range s.cfg.PerfCounters {
		objWatchers, err := objCfg.BuildPaths()
		if err != nil {
			s.settings.Logger.Warn("some performance counters could not be initialized", zap.Error(err))
			continue
		}
		for _, objWatcher := range objWatchers {
			watchers = append(watchers, objWatcher)
		}
	}
	s.watchers = watchers

	return nil
}

func (s *scraper) shutdown(context.Context) error {
	var errs error
	for _, watcher := range s.watchers {
		err := watcher.Close()
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return errs
}

func (s *scraper) scrape(context.Context) (pmetric.Metrics, error) {
	md := pmetric.NewMetrics()
	metricSlice := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
	now := pcommon.NewTimestampFromTime(time.Now())
	var errs error

	metricSlice.EnsureCapacity(len(s.watchers))
	metrics := map[string]pmetric.Metric{}
	for name, metricCfg := range s.cfg.MetricMetaData {
		builtMetric := metricSlice.AppendEmpty()

		builtMetric.SetName(name)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		if (metricCfg.Sum != SumMetric{}) {
			builtMetric.SetDataType(pmetric.MetricDataTypeSum)
			builtMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
			}
		} else {
			builtMetric.SetDataType(pmetric.MetricDataTypeGauge)
		}

		metrics[name] = builtMetric
	}

	counterVals := []winperfcounters.CounterValue{}
	for _, watcher := range s.watchers {
		scrapedCounterValues, err := watcher.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		counterVals = append(counterVals, scrapedCounterValues...)
	}

	for _, scrapedValue := range counterVals {
		var metric pmetric.Metric
		metricRep := scrapedValue.MetricRep
		if builtmetric, ok := metrics[metricRep.Name]; ok {
			metric = builtmetric
		} else {
			metric = metricSlice.AppendEmpty()
			metric.SetDataType(pmetric.MetricDataTypeGauge)
			metric.SetName(metricRep.Name)
			metric.SetUnit("1")
		}

		initializeMetricDps(metric, now, scrapedValue.Value, metricRep.Attributes)
	}

	return md, errs
}

func initializeMetricDps(metric pmetric.Metric, now pcommon.Timestamp, counterValue float64, attributes map[string]string) {
	var dps pmetric.NumberDataPointSlice

	if metric.DataType() == pmetric.MetricDataTypeGauge {
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
	dp.SetDoubleVal(counterValue)
}
