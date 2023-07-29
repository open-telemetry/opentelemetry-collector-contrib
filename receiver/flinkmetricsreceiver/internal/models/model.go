// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/models"

// These are processed metrics that are used to unique identify metrics from each scope source.
// See https://nightlies.apache.org/flink/flink-docs-release-1.14/docs/ops/metrics/#system-scope

// JobmanagerMetrics store metrics with associated identifier attributes.
type JobmanagerMetrics struct {
	Host    string
	Metrics MetricsResponse
}

// TaskmanagerMetrics store metrics with associated identifier attributes.
type TaskmanagerMetrics struct {
	Host          string
	TaskmanagerID string
	Metrics       MetricsResponse
}

// JobMetrics store metrics with associated identifier attributes.
type JobMetrics struct {
	Host    string
	JobName string
	Metrics MetricsResponse
}

// SubtaskMetrics store metrics with associated identifier attributes.
type SubtaskMetrics struct {
	Host          string
	TaskmanagerID string
	JobName       string
	TaskName      string
	SubtaskIndex  string
	Metrics       MetricsResponse
}
