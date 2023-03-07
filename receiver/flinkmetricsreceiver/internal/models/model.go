// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
