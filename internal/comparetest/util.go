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

package comparetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"

import (
	"bytes"
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/pdatautil"
)

func metricsByName(metricSlice pmetric.MetricSlice) map[string]pmetric.Metric {
	byName := make(map[string]pmetric.Metric, metricSlice.Len())
	for i := 0; i < metricSlice.Len(); i++ {
		a := metricSlice.At(i)
		byName[a.Name()] = a
	}
	return byName
}

func getDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.Type()))
	}
	return dataPointSlice
}

func sortResourceMetricsSlice(rms pmetric.ResourceMetricsSlice) {
	rms.Sort(func(a, b pmetric.ResourceMetrics) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortScopeMetricsSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		ms.ResourceMetrics().At(i).ScopeMetrics().Sort(func(a, b pmetric.ScopeMetrics) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

func sortMetricSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		for j := 0; j < ms.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Sort(func(a, b pmetric.Metric) bool {
				return a.Name() < b.Name()
			})
		}
	}
}

func sortMetricDataPointSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		for j := 0; j < ms.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					sortNumberDataPointSlice(m.Gauge().DataPoints())
				case pmetric.MetricTypeSum:
					sortNumberDataPointSlice(m.Sum().DataPoints())
				case pmetric.MetricTypeHistogram:
					sortHistogramDataPointSlice(m.Histogram().DataPoints())
				case pmetric.MetricTypeExponentialHistogram:
					sortExponentialHistogramDataPointSlice(m.ExponentialHistogram().DataPoints())
				case pmetric.MetricTypeSummary:
					sortSummaryDataPointSlice(m.Summary().DataPoints())
				}
			}
		}
	}
}

func sortNumberDataPointSlice(ndps pmetric.NumberDataPointSlice) {
	ndps.Sort(func(a, b pmetric.NumberDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortHistogramDataPointSlice(hdps pmetric.HistogramDataPointSlice) {
	hdps.Sort(func(a, b pmetric.HistogramDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortExponentialHistogramDataPointSlice(hdps pmetric.ExponentialHistogramDataPointSlice) {
	hdps.Sort(func(a, b pmetric.ExponentialHistogramDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortSummaryDataPointSlice(sds pmetric.SummaryDataPointSlice) {
	sds.Sort(func(a, b pmetric.SummaryDataPoint) bool {
		aAttrs := pdatautil.MapHash(a.Attributes())
		bAttrs := pdatautil.MapHash(b.Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortSummaryDataPointValueAtQuantileSlices(ms pmetric.Metrics) {
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		for j := 0; j < ms.ResourceMetrics().At(i).ScopeMetrics().Len(); j++ {
			for k := 0; k < ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().Len(); k++ {
				m := ms.ResourceMetrics().At(i).ScopeMetrics().At(j).Metrics().At(k)
				if m.Type() == pmetric.MetricTypeSummary {
					for l := 0; l < m.Summary().DataPoints().Len(); l++ {
						m.Summary().DataPoints().At(l).QuantileValues().Sort(func(a, b pmetric.SummaryDataPointValueAtQuantile) bool {
							return a.Quantile() < b.Quantile()
						})
					}
				}
			}
		}
	}
}

func sortResourceLogsSlice(rls plog.ResourceLogsSlice) {
	rls.Sort(func(a, b plog.ResourceLogs) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortScopeLogsSlices(ls plog.Logs) {
	for i := 0; i < ls.ResourceLogs().Len(); i++ {
		ls.ResourceLogs().At(i).ScopeLogs().Sort(func(a, b plog.ScopeLogs) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

func sortResourceSpansSlice(rms ptrace.ResourceSpansSlice) {
	rms.Sort(func(a, b ptrace.ResourceSpans) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

func sortScopeSpansSlices(ts ptrace.Traces) {
	for i := 0; i < ts.ResourceSpans().Len(); i++ {
		ts.ResourceSpans().At(i).ScopeSpans().Sort(func(a, b ptrace.ScopeSpans) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

func sortSpanSlices(ts ptrace.Traces) {
	for i := 0; i < ts.ResourceSpans().Len(); i++ {
		for j := 0; j < ts.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
			ts.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Sort(func(a, b ptrace.Span) bool {
				if a.Kind() != b.Kind() {
					return a.Kind() < b.Kind()
				}
				if a.Name() != b.Name() {
					return a.Name() < b.Name()
				}
				at := a.TraceID()
				bt := b.TraceID()
				if !bytes.Equal(at[:], bt[:]) {
					return bytes.Compare(at[:], bt[:]) < 0
				}
				as := a.SpanID()
				bs := b.SpanID()
				if !bytes.Equal(as[:], bs[:]) {
					return bytes.Compare(as[:], bs[:]) < 0
				}
				aps := a.ParentSpanID()
				bps := b.ParentSpanID()
				if !bytes.Equal(aps[:], bps[:]) {
					return bytes.Compare(aps[:], bps[:]) < 0
				}
				aAttrs := pdatautil.MapHash(a.Attributes())
				bAttrs := pdatautil.MapHash(b.Attributes())
				if !bytes.Equal(aAttrs[:], bAttrs[:]) {
					return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
				}
				if a.StartTimestamp() != b.StartTimestamp() {
					return a.StartTimestamp() < b.StartTimestamp()
				}
				return a.EndTimestamp() < b.EndTimestamp()
			})
		}
	}
}

func sortLogRecordSlices(ls plog.Logs) {
	for i := 0; i < ls.ResourceLogs().Len(); i++ {
		for j := 0; j < ls.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			ls.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Sort(func(a, b plog.LogRecord) bool {
				if a.ObservedTimestamp() != b.ObservedTimestamp() {
					return a.ObservedTimestamp() < b.ObservedTimestamp()
				}
				if a.Timestamp() != b.Timestamp() {
					return a.Timestamp() < b.Timestamp()
				}
				if a.SeverityNumber() != b.SeverityNumber() {
					return a.SeverityNumber() < b.SeverityNumber()
				}
				if a.SeverityText() != b.SeverityText() {
					return a.SeverityText() < b.SeverityText()
				}
				at := a.TraceID()
				bt := b.TraceID()
				if !bytes.Equal(at[:], bt[:]) {
					return bytes.Compare(at[:], bt[:]) < 0
				}
				as := a.SpanID()
				bs := b.SpanID()
				if !bytes.Equal(as[:], bs[:]) {
					return bytes.Compare(as[:], bs[:]) < 0
				}
				aAttrs := pdatautil.MapHash(a.Attributes())
				bAttrs := pdatautil.MapHash(b.Attributes())
				if !bytes.Equal(aAttrs[:], bAttrs[:]) {
					return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
				}
				ab := pdatautil.ValueHash(a.Body())
				bb := pdatautil.ValueHash(b.Body())
				return bytes.Compare(ab[:], bb[:]) < 0
			})
		}
	}
}
