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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/third_party/telegraf/win_perf_counters"
)

const totalInstanceName = "_Total"

var _ PerfCounterWatcher = (*perfCounter)(nil)

// PerfCounterWatcher represents how to scrape data
type PerfCounterWatcher interface {
	// Path returns the counter path
	Path() string
	// ScrapeData collects a measurement and returns the value(s).
	ScrapeData() ([]CounterValue, error)
	// Close all counters/handles related to the query and free all associated memory.
	Close() error
}

type CounterValue = win_perf_counters.CounterValue

type perfCounter struct {
	path   string
	query  win_perf_counters.PerformanceQuery
	handle win_perf_counters.PDH_HCOUNTER
}

// NewWatcher creates new PerfCounterWatcher by provided parts of its path.
func NewWatcher(object, instance, counterName string) (PerfCounterWatcher, error) {
	path := counterPath(object, instance, counterName)
	counter, err := newPerfCounter(path, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create perf counter with path %v: %w", path, err)
	}
	return counter, nil
}

func counterPath(object, instance, counterName string) string {
	if instance != "" {
		instance = fmt.Sprintf("(%s)", instance)
	}

	return fmt.Sprintf("\\%s%s\\%s", object, instance, counterName)
}

// newPerfCounter returns a new performance counter for the specified descriptor.
func newPerfCounter(counterPath string, collectOnStartup bool) (*perfCounter, error) {
	query := &win_perf_counters.PerformanceQueryImpl{}
	err := query.Open()
	if err != nil {
		return nil, err
	}

	var handle win_perf_counters.PDH_HCOUNTER
	handle, err = query.AddEnglishCounterToQuery(counterPath)
	if err != nil {
		return nil, err
	}

	// Some perf counters (e.g. cpu) return the usage stats since the last measure.
	// We collect data on startup to avoid an invalid initial reading
	if collectOnStartup {
		err = query.CollectData()
		if err != nil {
			return nil, err
		}
	}

	counter := &perfCounter{
		path:   counterPath,
		query:  query,
		handle: handle,
	}

	return counter, nil
}

func (pc *perfCounter) Close() error {
	return pc.query.Close()
}

func (pc *perfCounter) Path() string {
	return pc.path
}

func (pc *perfCounter) ScrapeData() ([]CounterValue, error) {
	err := pc.query.CollectData()
	if err != nil {
		return nil, err
	}

	vals, err := pc.query.GetFormattedCounterArrayDouble(pc.handle)
	if err != nil {
		return nil, err
	}

	vals = removeTotalIfMultipleValues(vals)
	return vals, nil
}

func removeTotalIfMultipleValues(vals []CounterValue) []CounterValue {
	if len(vals) == 0 {
		return vals
	}

	if len(vals) == 1 {
		// if there is only one item & the instance name is "_Total", clear the instance name
		if vals[0].InstanceName == totalInstanceName {
			vals[0].InstanceName = ""
		}
		return vals
	}

	// if there is more than one item, remove an item that has the instance name "_Total"
	for i, val := range vals {
		if val.InstanceName == totalInstanceName {
			return removeItemAt(vals, i)
		}
	}

	return vals
}

func removeItemAt(vals []CounterValue, idx int) []CounterValue {
	vals[idx] = vals[len(vals)-1]
	vals[len(vals)-1] = CounterValue{}
	return vals[:len(vals)-1]
}
