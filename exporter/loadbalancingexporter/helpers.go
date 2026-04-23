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

func mergeLogs(l1, l2 plog.Logs) plog.Logs {
	l2.ResourceLogs().MoveAndAppendTo(l1.ResourceLogs())
	return l1
}

func mergeMetrics(m1, m2 pmetric.Metrics) pmetric.Metrics {
	m2.ResourceMetrics().MoveAndAppendTo(m1.ResourceMetrics())
	return m1
}

func failedTracesFromError(err error, td ptrace.Traces) ptrace.Traces {
	var traceErr consumererror.Traces
	if errors.As(err, &traceErr) {
		return traceErr.Data()
	}

	return td
}

func failedLogsFromError(err error, ld plog.Logs) plog.Logs {
	var logErr consumererror.Logs
	if errors.As(err, &logErr) {
		return logErr.Data()
	}

	return ld
}

func failedMetricsFromError(err error, md pmetric.Metrics) pmetric.Metrics {
	var metricErr consumererror.Metrics
	if errors.As(err, &metricErr) {
		return metricErr.Data()
	}

	return md
}
