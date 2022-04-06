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

package windowsperfcountercommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon/internal/pdh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon/internal/third_party/telegraf/win_perf_counters"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type PerfCounterScraper interface {
	// Path returns the counter path
	Path() string
	// ScrapeData collects a measurement and returns the value(s).
	ScrapeData() ([]win_perf_counters.CounterValue, error)
	// Close all counters/handles related to the query and free all associated memory.
	Close() error
}

const instanceLabelName = "instance"

type ScraperCfg struct {
	CounterCfg PerfCounterConfig
	Metric     MetricRep
}

type MetricRep struct {
	Name       string
	Attributes map[string]string
}

type Scraper struct {
	Counter PerfCounterScraper
	Metric  MetricRep
}

func BuildPaths(scraperCfgs []ScraperCfg, logger *zap.Logger) []Scraper {
	var errs error
	var scrapers []Scraper

	for _, scraperCfg := range scraperCfgs {
		for _, instance := range scraperCfg.CounterCfg.instances() {
			for _, counterCfg := range scraperCfg.CounterCfg.Counters {
				counterPath := counterPath(scraperCfg.CounterCfg.Object, instance, counterCfg.Name)

				c, err := pdh.NewPerfCounter(counterPath, true)
				if err != nil {
					errs = multierr.Append(errs, fmt.Errorf("counter %v: %w", counterPath, err))
				} else {
					newScraper := Scraper{Counter: c}

					if scraperCfg.Metric.Name != "" {
						newScraper.Metric = scraperCfg.Metric
					} else if counterCfg.Metric != "" {
						metricCfg := MetricRep{Name: counterCfg.Metric}
						if counterCfg.Attributes != nil {
							metricCfg.Attributes = counterCfg.Attributes
						}
						newScraper.Metric.Name = counterCfg.Metric
					} else {
						newScraper.Metric.Name = c.Path()
					}

					scrapers = append(scrapers, newScraper)
				}
			}
		}
	}

	// log a warning if some counters cannot be loaded, but do not crash the app
	if errs != nil {
		logger.Warn("some performance counters could not be initialized", zap.Error(errs))
	}

	return scrapers
}

func counterPath(object, instance, counterName string) string {
	if instance != "" {
		instance = fmt.Sprintf("(%s)", instance)
	}

	return fmt.Sprintf("\\%s%s\\%s", object, instance, counterName)
}

type ScrapedValues struct {
	Metric MetricRep
	Value  int64
}

func ScrapeCounters(scrapers []Scraper) (metrics []ScrapedValues, errs error) {
	for _, scraper := range scrapers {
		counterValues, err := scraper.Counter.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		metric := scraper.Metric

		for _, counterValue := range counterValues {
			if counterValue.InstanceName != "" {
				if metric.Attributes == nil {
					metric.Attributes = map[string]string{instanceLabelName: counterValue.InstanceName}
				}
				metric.Attributes[instanceLabelName] = counterValue.InstanceName
			}

			metrics = append(metrics, ScrapedValues{Metric: scraper.Metric, Value: int64(counterValue.Value)})
		}
	}
	return metrics, errs
}

func CloseCounters(scrapers []Scraper) error {
	var errs error

	for _, scraper := range scrapers {
		errs = multierr.Append(errs, scraper.Counter.Close())
	}

	return errs
}
