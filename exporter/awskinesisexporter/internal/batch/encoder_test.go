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
