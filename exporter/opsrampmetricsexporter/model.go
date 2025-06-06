// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opsrampmetricsexporter"

// OpsRampMetric represents a simplified metric structure.
// Each OTLP metric data point will be converted to this format.
type OpsRampMetric struct {
	MetricName string            `json:"metricName"`
	Value      float64           `json:"value"`
	Timestamp  int64             `json:"timestamp"`
	Labels     map[string]string `json:"labels"`
}
