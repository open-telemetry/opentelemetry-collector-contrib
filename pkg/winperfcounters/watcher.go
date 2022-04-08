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

package winperfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"

import (
	"fmt"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/pdh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/third_party/telegraf/win_perf_counters"
)

var _ PerfCounterWatcher = (*Watcher)(nil)

// PerfCounterWatcher represents how to scrape data
type PerfCounterWatcher interface {
	// Path returns the counter path
	Path() string
	// ScrapeData collects a measurement and returns the value(s).
	ScrapeData() ([]win_perf_counters.CounterValue, error)
	// Close all counters/handles related to the query and free all associated memory.
	Close() error

	GetMetricRep() MetricRep
}

const instanceLabelName = "instance"

type Watcher struct {
	Counter *pdh.PerfCounter
	MetricRep
}

func (w Watcher) Path() string {
	return w.Counter.Path()
}

func (w Watcher) ScrapeData() ([]win_perf_counters.CounterValue, error) {
	return w.Counter.ScrapeData()
}

func (w Watcher) Close() error {
	return w.Counter.Close()
}

func (w Watcher) GetMetricRep() MetricRep {
	return w.MetricRep
}

// BuildPaths creates watchers and their paths from configs.
func BuildPaths(objs []ObjectConfig) ([]PerfCounterWatcher, error) {
	var errs error
	var watchers []PerfCounterWatcher

	for _, objCfg := range objs {
		for _, instance := range objCfg.instances() {
			for _, counterCfg := range objCfg.Counters {
				counterPath := counterPath(objCfg.Object, instance, counterCfg.Name)

				c, err := pdh.NewPerfCounter(counterPath, true)
				if err != nil {
					errs = multierr.Append(errs, fmt.Errorf("counter %v: %w", counterPath, err))
				} else {
					newWatcher := Watcher{Counter: c}

					if counterCfg.MetricRep.Name != "" {
						metricCfg := MetricRep{Name: counterCfg.MetricRep.Name}
						if counterCfg.Attributes != nil {
							metricCfg.Attributes = counterCfg.Attributes
						}
						newWatcher.MetricRep = metricCfg
					} else {
						newWatcher.MetricRep.Name = c.Path()
					}

					watchers = append(watchers, newWatcher)
				}
			}
		}
	}

	return watchers, errs
}

func counterPath(object, instance, counterName string) string {
	if instance != "" {
		instance = fmt.Sprintf("(%s)", instance)
	}

	return fmt.Sprintf("\\%s%s\\%s", object, instance, counterName)
}

type CounterValue struct {
	MetricRep
	Value int64
}

// WatchCounters pulls values given the passed in watchers
func WatchCounters(watchers []PerfCounterWatcher) (metrics []CounterValue, errs error) {
	for _, watcher := range watchers {
		counterValues, err := watcher.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		metric := watcher.GetMetricRep()

		if len(counterValues) != 1 {
			continue
		}
		counterValue := counterValues[0]

		if counterValue.InstanceName != "" {
			if metric.Attributes == nil {
				metric.Attributes = map[string]string{instanceLabelName: counterValue.InstanceName}
			}
			metric.Attributes[instanceLabelName] = counterValue.InstanceName
		}

		metrics = append(metrics, CounterValue{MetricRep: metric, Value: int64(counterValue.Value)})

	}
	return metrics, errs
}

// CloseCounters closes the passed in counters.
// This should be called in the shutdown function of receivers using this package
func CloseCounters(watchers []PerfCounterWatcher) error {
	var errs error

	for _, watcher := range watchers {
		errs = multierr.Append(errs, watcher.Close())
	}

	return errs
}
