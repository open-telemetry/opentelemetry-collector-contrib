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

package windowsperfcountersreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver"

import (
	"fmt"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

// Config defines configuration for WindowsPerfCounters receiver.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	MetricMetaData map[string]MetricConfig `mapstructure:"metrics"`
	PerfCounters   []PerfCounterConfig     `mapstructure:"perfcounters"`
}

// PerfCounterConfig defines configuration for a perf counter object.
type PerfCounterConfig struct {
	Object    string          `mapstructure:"object"`
	Instances []string        `mapstructure:"instances"`
	Counters  []CounterConfig `mapstructure:"counters"`
}

// MetricsConfig defines the configuration for a metric to be created.
type MetricConfig struct {
	Unit        string      `mapstructure:"unit"`
	Description string      `mapstructure:"description"`
	Gauge       GaugeMetric `mapstructure:"gauge"`
	Sum         SumMetric   `mapstructure:"sum"`
}

type GaugeMetric struct {
}

type SumMetric struct {
	Aggregation string `mapstructure:"aggregation"`
	Monotonic   bool   `mapstructure:"monotonic"`
}

type CounterConfig struct {
	Metric     string            `mapstructure:"metric"`
	Name       string            `mapstructure:"name"`
	Attributes map[string]string `mapstructure:"attributes"`
}

func (c *Config) Validate() error {
	var errs error

	if c.CollectionInterval <= 0 {
		errs = multierr.Append(errs, fmt.Errorf("collection_interval must be a positive duration"))
	}

	if len(c.PerfCounters) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("must specify at least one perf counter"))
	}

	if len(c.MetricMetaData) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("must specify at least one metric"))
	}

	for name, metric := range c.MetricMetaData {
		if metric.Unit == "" {
			metric.Unit = "1"
		}

		if (metric.Sum != SumMetric{}) {
			if metric.Sum.Aggregation != "cumulative" && metric.Sum.Aggregation != "delta" {
				errs = multierr.Append(errs, fmt.Errorf("sum metric %q includes an invalid aggregation", name))
			}
		} else if (metric.Sum != SumMetric{}) && (metric.Gauge != GaugeMetric{}) {
			errs = multierr.Append(errs, fmt.Errorf("metric %q provides both a sum config and a gauge config", name))
		}
	}

	var perfCounterMissingObjectName bool
	for _, pc := range c.PerfCounters {
		if pc.Object == "" {
			perfCounterMissingObjectName = true
			continue
		}

		if len(pc.Counters) == 0 {
			errs = multierr.Append(errs, fmt.Errorf("perf counter for object %q does not specify any counters", pc.Object))
		}

		for _, counter := range pc.Counters {
			foundMatchingMetric := false
			for name := range c.MetricMetaData {
				if counter.Metric == name {
					foundMatchingMetric = true
				}
			}
			if !foundMatchingMetric {
				errs = multierr.Append(errs, fmt.Errorf("perf counter for object %q includes an undefined metric", pc.Object))
			}
		}

		for _, instance := range pc.Instances {
			if instance == "" {
				errs = multierr.Append(errs, fmt.Errorf("perf counter for object %q includes an empty instance", pc.Object))
				break
			}
		}
	}

	if perfCounterMissingObjectName {
		errs = multierr.Append(errs, fmt.Errorf("must specify object name for all perf counters"))
	}

	return errs
}
