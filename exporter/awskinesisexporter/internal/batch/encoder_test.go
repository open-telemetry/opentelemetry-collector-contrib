// Copyright  OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/model/pdata"
)

func NewTestTraces(spanCount int) pdata.Traces {
	traces := pdata.NewTraces()

	for i := 0; i < spanCount; i++ {
		span := traces.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("foo")
		span.SetStartTimestamp(pdata.Timestamp(10))
		span.SetEndTimestamp(pdata.Timestamp(20))
		span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
		span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	}

	return traces
}

func NewTestMetrics(metricCount int) pdata.Metrics {
	metrics := pdata.NewMetrics()

	for i := 0; i < metricCount; i++ {
		metric := metrics.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("foo")
		metric.SetUnit("bar")
		metric.SetDataType(pdata.MetricDataTypeGauge)
		metric.Gauge().DataPoints().AppendEmpty().SetIntVal(int64(i))
	}

	return metrics
}

func NewTestLogs(logCount int) pdata.Logs {
	logs := pdata.NewLogs()

	for i := 0; i < logCount; i++ {
		log := logs.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty().Logs().AppendEmpty()
		log.SetName("foo")
		log.SetSeverityText("bar")
	}

	return logs
}
