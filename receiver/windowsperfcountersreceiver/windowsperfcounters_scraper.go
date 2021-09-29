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

package windowsperfcountersreceiver

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
	logger   *zap.Logger
	counters []PerfCounterScraper
}

func newScraper(cfg *Config, logger *zap.Logger) (*scraper, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	s := &scraper{cfg: cfg, logger: logger}
	return s, nil
}

func (s *scraper) start(context.Context, component.Host) error {
	var errs error

	for _, perfCounterCfg := range s.cfg.PerfCounters {
		for _, instance := range perfCounterCfg.instances() {
			for _, counterName := range perfCounterCfg.Counters {
				counterPath := counterPath(perfCounterCfg.Object, instance, counterName)

				c, err := pdh.NewPerfCounter(counterPath, true)
				if err != nil {
					errs = multierr.Append(errs, fmt.Errorf("counter %v: %w", counterPath, err))
				} else {
					s.counters = append(s.counters, c)
				}
			}
		}
	}

	// log a warning if some counters cannot be loaded, but do not crash the app
	if errs != nil {
		s.logger.Warn("some performance counters could not be initialized", zap.Error(errs))
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
		errs = multierr.Append(errs, counter.Close())
	}

	return errs
}

func (s *scraper) scrape(context.Context) (pdata.MetricSlice, error) {
	metrics := pdata.NewMetricSlice()

	now := pdata.NewTimestampFromTime(time.Now())

	var errs error

	metrics.EnsureCapacity(len(s.counters))
	for _, counter := range s.counters {
		counterValues, err := counter.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		initializeDoubleGaugeMetric(metrics.AppendEmpty(), now, counter.Path(), counterValues)
	}

	return metrics, errs
}

func initializeDoubleGaugeMetric(metric pdata.Metric, now pdata.Timestamp, name string, counterValues []win_perf_counters.CounterValue) {
	metric.SetName(name)
	metric.SetDataType(pdata.MetricDataTypeGauge)

	dg := metric.Gauge()
	ddps := dg.DataPoints()
	ddps.EnsureCapacity(len(counterValues))
	for _, counterValue := range counterValues {
		initializeNumberDataPointAsDouble(ddps.AppendEmpty(), now, counterValue.InstanceName, counterValue.Value)
	}
}

func initializeNumberDataPointAsDouble(dataPoint pdata.NumberDataPoint, now pdata.Timestamp, instanceLabel string, value float64) {
	if instanceLabel != "" {
		dataPoint.Attributes().InsertString(instanceLabelName, instanceLabel)
	}

	dataPoint.SetTimestamp(now)
	dataPoint.SetDoubleVal(value)
}
