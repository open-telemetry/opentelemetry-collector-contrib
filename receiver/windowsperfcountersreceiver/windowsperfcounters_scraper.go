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
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/pdh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/third_party/telegraf/win_perf_counters"
)

const instanceLabelName = "instance"

type PerfCounterScraper interface {
	// Path returns the counter path
	Path() string
	// ScrapeData collects a measurement and returns the value(s).
	ScrapeData() ([]win_perf_counters.CounterValue, error)
	// Close all counters/handles related to the query and free all associated memory.
	Close() error
}

// scraper is the type that scrapes various host metrics.
type scraper struct {
	cfg      *Config
	settings component.TelemetrySettings
	counters []PerfCounterMetrics
}

type PerfCounterMetrics struct {
	CounterScraper PerfCounterScraper
	Attributes     map[string]string
	Metric         string
}

func newScraper(cfg *Config, settings component.TelemetrySettings) *scraper {
	s := &scraper{cfg: cfg, settings: settings}
	return s
}

func (s *scraper) start(context.Context, component.Host) error {
	var errs error

	for _, perfCounterCfg := range s.cfg.PerfCounters {
		for _, instance := range perfCounterCfg.instances() {
			for _, counterCfg := range perfCounterCfg.Counters {
				counterPath := counterPath(perfCounterCfg.Object, instance, counterCfg.Name)

				c, err := pdh.NewPerfCounter(counterPath, true)
				if err != nil {
					errs = multierr.Append(errs, fmt.Errorf("counter %v: %w", counterPath, err))
				} else {
					s.counters = append(s.counters, PerfCounterMetrics{CounterScraper: c, Metric: counterCfg.Metric, Attributes: counterCfg.Attributes})
				}
			}
		}
	}

	// log a warning if some counters cannot be loaded, but do not crash the app
	if errs != nil {
		s.settings.Logger.Warn("some performance counters could not be initialized", zap.Error(errs))
	}

	return nil
}

func counterPath(object, instance, counterName string) string {
	if instance != "" {
		instance = fmt.Sprintf("(%s)", instance)
	}

	return fmt.Sprintf("\\%s%s\\%s", object, instance, counterName)
}

func (s *scraper) shutdown(context.Context) error {
	var errs error

	for _, counter := range s.counters {
		errs = multierr.Append(errs, counter.CounterScraper.Close())
	}

	return errs
}

func (s *scraper) scrape(context.Context) (pdata.Metrics, error) {
	md := pdata.NewMetrics()
	metrics := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics()
	now := pdata.NewTimestampFromTime(time.Now())
	var errs error

	metrics.EnsureCapacity(len(s.counters))

	for name, metricCfg := range s.cfg.MetricMetaData {
		builtMetric := metrics.AppendEmpty()

		builtMetric.SetName(name)
		builtMetric.SetDescription(metricCfg.Description)
		builtMetric.SetUnit(metricCfg.Unit)

		if (metricCfg.Gauge != GaugeMetric{}) {
			builtMetric.SetDataType(pdata.MetricDataTypeGauge)
		} else if (metricCfg.Sum != SumMetric{}) {
			builtMetric.SetDataType(pdata.MetricDataTypeSum)
			builtMetric.Sum().SetIsMonotonic(metricCfg.Sum.Monotonic)

			switch metricCfg.Sum.Aggregation {
			case "cumulative":
				builtMetric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
			case "delta":
				builtMetric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
			}
		}

		for _, counter := range s.counters {
			if counter.Metric == builtMetric.Name() {
				counterValues, err := counter.CounterScraper.ScrapeData()
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				initializeMetricDps(metricCfg, builtMetric, now, counterValues, counter.Attributes)
			}
		}
	}

	return md, errs
}

func initializeMetricDps(metricCfg MetricConfig, metric pdata.Metric, now pdata.Timestamp, counterValues []win_perf_counters.CounterValue, attributes map[string]string) {
	var dps pdata.NumberDataPointSlice
	var valueType string

	if metric.DataType() == pdata.MetricDataTypeGauge {
		dps = metric.Gauge().DataPoints()
		valueType = metricCfg.Gauge.ValueType
	} else {
		dps = metric.Sum().DataPoints()
		valueType = metricCfg.Sum.ValueType
	}

	dps.EnsureCapacity(len(counterValues))
	for _, counterValue := range counterValues {
		dp := dps.AppendEmpty()
		for attKey, attVal := range attributes {
			dp.Attributes().InsertString(attKey, attVal)
		}
		if counterValue.InstanceName != "" {
			dp.Attributes().InsertString(instanceLabelName, counterValue.InstanceName)
		}

		dp.SetTimestamp(now)
		if valueType == "int" {
			dp.SetIntVal(int64(counterValue.Value))
		} else if valueType == "double" {
			dp.SetDoubleVal(counterValue.Value)
		}
	}
}
