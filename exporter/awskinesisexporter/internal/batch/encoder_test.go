// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch_test

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func NewTestTraces(spanCount int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for range spanCount {
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("foo")
		span.SetStartTimestamp(pcommon.Timestamp(10))
		span.SetEndTimestamp(pcommon.Timestamp(20))
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	}

	return traces
}

func NewTestMetrics(metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	for i := range metricCount {
		metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foo")
		metric.SetUnit("bar")
		metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(int64(i))
	}

	return metrics
}

func NewTestLogs(logCount int) plog.Logs {
	logs := plog.NewLogs()

	for range logCount {
		log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		log.SetSeverityText("bar")
	}

	return logs
}
