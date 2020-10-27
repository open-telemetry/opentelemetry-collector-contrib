// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"fmt"

	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
)

const (
	// Gauge is the Datadog Gauge metric type
	Gauge string = "gauge"
)

// NewGauge creates a new Datadog Gauge metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewGauge(name string, ts uint64, value float64, tags []string) datadog.Metric {
	// Transform UnixNano timestamp into Unix timestamp
	// 1 second = 1e9 ns
	timestamp := float64(ts / 1e9)

	gauge := datadog.Metric{
		Points: []datadog.DataPoint{[2]*float64{&timestamp, &value}},
		Tags:   tags,
	}
	gauge.SetMetric(name)
	gauge.SetType(Gauge)
	return gauge
}

// RunningMetric creates a metric to report that an exporter is running
func RunningMetric(exporterType string, timestamp uint64, logger *zap.Logger, cfg *config.Config) []datadog.Metric {
	runningMetric := []datadog.Metric{
		NewGauge(fmt.Sprintf("otel.datadog_exporter.%s.running", exporterType), timestamp, float64(1.0), []string{}),
	}

	AddHostname(runningMetric, logger, cfg)

	return runningMetric
}

// AddHostname adds an hostname to metrics, either using the hostname given
// in the config, or retrieved from the host metadata
func AddHostname(metrics []datadog.Metric, logger *zap.Logger, cfg *config.Config) {
	overrideHostname := cfg.Hostname != ""

	for i := range metrics {
		if overrideHostname || metrics[i].GetHost() == "" {
			metrics[i].Host = metadata.GetHost(logger, cfg)
		}
	}
}

// AddNamespace prepends all metric names with a given namespace
func AddNamespace(metrics []datadog.Metric, namespace string) {
	for i := range metrics {
		newName := namespace + *metrics[i].Metric
		metrics[i].Metric = &newName
	}
}
