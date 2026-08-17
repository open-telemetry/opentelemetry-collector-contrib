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

// mergeTraces concatenates two ptrace.Traces into a single ptrace.Traces.
func mergeTraces(t1, t2 ptrace.Traces) ptrace.Traces {
	t2.ResourceSpans().MoveAndAppendTo(t1.ResourceSpans())
	return t1
}

// mergeLogs concatenates two plog.Logs into a single plog.Logs.
func mergeLogs(l1, l2 plog.Logs) plog.Logs {
	l2.ResourceLogs().MoveAndAppendTo(l1.ResourceLogs())
	return l1
}

// The failed*FromError helpers return the narrower failure payload a sub-exporter reported,
// falling back to the full batch that was sent to that backend when it reported none.

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
