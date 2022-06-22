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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/pdh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/third_party/telegraf/win_perf_counters"
)

// TODO: This package became redundant as a wrapper. pkg/winperfcounters/internal/pdh can be moved here.

var _ PerfCounterWatcher = (*Watcher)(nil)

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

type Watcher = pdh.PerfCounter

// NewWatcher creates new PerfCounterWatcher by provided parts of its path.
func NewWatcher(object, instance, counterName string) (PerfCounterWatcher, error) {
	path := counterPath(object, instance, counterName)
	counter, err := pdh.NewPerfCounter(path, true)
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
