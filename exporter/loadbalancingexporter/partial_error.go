// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func failedTracesFromError(err error, fallback ptrace.Traces) ptrace.Traces {
	var tracesErr consumererror.Traces
	if errors.As(err, &tracesErr) {
		return tracesErr.Data()
	}

	return fallback
}

func failedLogsFromError(err error, fallback plog.Logs) plog.Logs {
	var logsErr consumererror.Logs
	if errors.As(err, &logsErr) {
		return logsErr.Data()
	}

	return fallback
}

func failedMetricsFromError(err error, fallback pmetric.Metrics) pmetric.Metrics {
	var metricsErr consumererror.Metrics
	if errors.As(err, &metricsErr) {
		return metricsErr.Data()
	}

	return fallback
}
