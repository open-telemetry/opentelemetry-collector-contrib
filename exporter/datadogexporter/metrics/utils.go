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
	"strings"

	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
)

const (
	// Gauge is the Datadog Gauge metric type
	Gauge               string = "gauge"
	Count               string = "count"
	otelNamespacePrefix string = "otel"
)

// newMetric creates a new Datadog metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func newMetric(name string, ts uint64, value float64, tags []string) datadog.Metric {
	// Transform UnixNano timestamp into Unix timestamp
	// 1 second = 1e9 ns
	timestamp := float64(ts / 1e9)

	metric := datadog.Metric{
		Points: []datadog.DataPoint{[2]*float64{&timestamp, &value}},
		Tags:   tags,
	}
	metric.SetMetric(name)
	return metric
}

// NewGauge creates a new Datadog Gauge metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewGauge(name string, ts uint64, value float64, tags []string) datadog.Metric {
	gauge := newMetric(name, ts, value, tags)
	gauge.SetType(Gauge)
	return gauge
}

// NewCount creates a new Datadog count metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewCount(name string, ts uint64, value float64, tags []string) datadog.Metric {
	count := newMetric(name, ts, value, tags)
	count.SetType(Count)
	return count
}

// DefaultMetrics creates built-in metrics to report that an exporter is running
func DefaultMetrics(exporterType string, timestamp uint64) []datadog.Metric {
	return []datadog.Metric{
		NewGauge(fmt.Sprintf("datadog_exporter.%s.running", exporterType), timestamp, 1.0, []string{}),
	}
}

// ProcessMetrics adds the hostname to the metric and prefixes it with the "otel"
// namespace as the Datadog backend expects
func ProcessMetrics(ms []datadog.Metric, logger *zap.Logger, cfg *config.Config) {
	addNamespace(ms, otelNamespacePrefix)
	addHostname(ms, logger, cfg)
}

// addHostname adds an hostname to metrics, either using the hostname given
// in the config, or retrieved from the host metadata
func addHostname(metrics []datadog.Metric, logger *zap.Logger, cfg *config.Config) {
	overrideHostname := cfg.Hostname != ""

	for i := range metrics {
		if overrideHostname || metrics[i].GetHost() == "" {
			metrics[i].Host = metadata.GetHost(logger, cfg)
		}
	}
}

// shouldPrepend decides if a given metric name should be prepended by `otel.`.
// By default, this happens for
// - hostmetrics receiver metrics (since they clash with Datadog Agent system check) and
// - running metrics
func shouldPrepend(name string) bool {
	namespaces := [...]string{"datadog_exporter.", "system.", "process."}
	for _, ns := range namespaces {
		if strings.HasPrefix(name, ns) {
			return true
		}
	}
	return false
}

// addNamespace prepends some metric names with a given namespace.
// This is used to namespace metrics that clash with the Datadog Agent
func addNamespace(metrics []datadog.Metric, namespace string) {
	for i := range metrics {
		if shouldPrepend(*metrics[i].Metric) {
			newName := namespace + "." + *metrics[i].Metric
			metrics[i].Metric = &newName
		}
	}
}
