// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

// MetricsData generates OTLP metrics from endpoint raw data
func MetricsData(containerStatsMap map[string]*ContainerStats, metadata ecsutil.TaskMetadata, logger *zap.Logger, trackers ...*TaskResetTracker) []pmetric.Metrics {
	var tracker *TaskResetTracker
	if len(trackers) > 0 {
		tracker = trackers[0]
	}
	acc := &metricDataAccumulator{tracker: tracker}
	acc.getMetricsData(containerStatsMap, metadata, logger)

	return acc.mds
}
