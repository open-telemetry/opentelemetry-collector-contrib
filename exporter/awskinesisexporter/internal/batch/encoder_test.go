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

	for i := 0; i < spanCount; i++ {
		scopeSpans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
		span1 := scopeSpans.AppendEmpty()
		span1.SetName("trace1 spans")
		span1.SetStartTimestamp(pcommon.Timestamp(10))
		span1.SetEndTimestamp(pcommon.Timestamp(20))
		span1.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span1.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

		span2 := scopeSpans.AppendEmpty()
		span2.SetName("trace2 span")
		span2.SetStartTimestamp(pcommon.Timestamp(10))
		span2.SetEndTimestamp(pcommon.Timestamp(20))
		span2.SetTraceID([16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
		span2.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
	}

	return traces
}

func NewTestMetrics(metricCount int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	for i := 0; i < metricCount; i++ {
		metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foo")
		metric.SetUnit("bar")
		metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(int64(i))
	}

	return metrics
}

func NewTestLogs(logCount int) plog.Logs {
	logs := plog.NewLogs()

	for i := 0; i < logCount; i++ {
		log := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		log.SetSeverityText("bar")
	}

	return logs
}
