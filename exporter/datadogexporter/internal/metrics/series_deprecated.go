// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"
)

type MetricType string

const (
	// Gauge is the Datadog Gauge metric type
	Gauge MetricType = "gauge"
	// Count is the Datadog Count metric type
	Count MetricType = "count"
)

// newZorkianMetric creates a new Zorkian Datadog metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func newZorkianMetric(name string, ts uint64, value float64, tags []string) zorkian.Metric {
	// Transform UnixNano timestamp into Unix timestamp
	// 1 second = 1e9 ns
	timestamp := float64(ts / 1e9)

	metric := zorkian.Metric{
		Points: []zorkian.DataPoint{[2]*float64{&timestamp, &value}},
		Tags:   tags,
	}
	metric.SetMetric(name)
	return metric
}

// NewZorkianMetric creates a new Zorkian Datadog metric given a name, a type, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewZorkianMetric(name string, dt MetricType, ts uint64, value float64, tags []string) zorkian.Metric {
	metric := newZorkianMetric(name, ts, value, tags)
	metric.SetType(string(dt))
	return metric
}

// NewZorkianGauge creates a new Datadog Gauge metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewZorkianGauge(name string, ts uint64, value float64, tags []string) zorkian.Metric {
	return NewZorkianMetric(name, Gauge, ts, value, tags)
}

// NewZorkianCount creates a new Datadog count metric given a name, a Unix nanoseconds timestamp
// a value and a slice of tags
func NewZorkianCount(name string, ts uint64, value float64, tags []string) zorkian.Metric {
	return NewZorkianMetric(name, Count, ts, value, tags)
}

// DefaultZorkianMetrics creates built-in metrics to report that an exporter is running
func DefaultZorkianMetrics(exporterType string, hostname string, timestamp uint64, buildInfo component.BuildInfo) []zorkian.Metric {
	var tags []string

	if buildInfo.Version != "" {
		tags = append(tags, "version:"+buildInfo.Version)
	}

	if buildInfo.Command != "" {
		tags = append(tags, "command:"+buildInfo.Command)
	}

	metrics := []zorkian.Metric{
		NewZorkianGauge(fmt.Sprintf("otel.datadog_exporter.%s.running", exporterType), timestamp, 1.0, tags),
	}

	for i := range metrics {
		metrics[i].SetHost(hostname)
	}

	return metrics
}
