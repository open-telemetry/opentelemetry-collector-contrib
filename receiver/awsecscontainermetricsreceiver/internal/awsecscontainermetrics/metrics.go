// Copyright 2020, OpenTelemetry Authors
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

package awsecscontainermetrics

import (
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// MetricsData generates OTLP metrics from endpoint raw data
func MetricsData(containerStatsMap map[string]*ContainerStats, metadata TaskMetadata, logger *zap.Logger) []pdata.Metrics {
	acc := &metricDataAccumulator{}
	acc.getMetricsData(containerStatsMap, metadata, logger)

	return acc.mds
}
