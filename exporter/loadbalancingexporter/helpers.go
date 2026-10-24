// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
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
	if tracesErr, ok := errors.AsType[consumererror.Traces](err); ok {
		return tracesErr.Data()
	}
	return fallback
}

func failedLogsFromError(err error, fallback plog.Logs) plog.Logs {
	if logsErr, ok := errors.AsType[consumererror.Logs](err); ok {
		return logsErr.Data()
	}
	return fallback
}

func failedMetricsFromError(err error, fallback pmetric.Metrics) pmetric.Metrics {
	if metricsErr, ok := errors.AsType[consumererror.Metrics](err); ok {
		return metricsErr.Data()
	}
	return fallback
}

// The copyFailed* helpers combine failure payloads into newly allocated data. The payloads
// are borrowed from backend batches and errors, so they are copied rather than moved and
// none of them is modified.

func copyFailedTraces(failed []ptrace.Traces) ptrace.Traces {
	out := ptrace.NewTraces()
	for _, td := range failed {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			rss.At(i).CopyTo(out.ResourceSpans().AppendEmpty())
		}
	}
	return out
}

func copyFailedLogs(failed []plog.Logs) plog.Logs {
	out := plog.NewLogs()
	for _, ld := range failed {
		rls := ld.ResourceLogs()
		for i := 0; i < rls.Len(); i++ {
			rls.At(i).CopyTo(out.ResourceLogs().AppendEmpty())
		}
	}
	return out
}

func copyFailedMetrics(failed []pmetric.Metrics) pmetric.Metrics {
	merger := metrics.NewMerger(pmetric.NewMetrics())
	for _, md := range failed {
		merger.Merge(md)
	}
	return merger.Metrics()
}
