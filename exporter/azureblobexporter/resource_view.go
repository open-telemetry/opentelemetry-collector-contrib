// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// With template_scope set to resource, blob name templates are evaluated
// against these views instead of a copy of each resource entry. A view presents
// one resource entry of the original payload as a payload holding only that
// entry, so templates written for the request scope keep working.

// resourceSliceView is a one-entry resource slice.
type resourceSliceView[R any] struct{ resource R }

func (resourceSliceView[R]) Len() int { return 1 }

func (s resourceSliceView[R]) At(i int) (R, error) {
	if i != 0 {
		var zero R
		return zero, fmt.Errorf("resource index %d out of range: with template_scope resource, only index 0 exists", i)
	}
	return s.resource, nil
}

type logsView struct{ resource plog.ResourceLogs }

func (v logsView) ResourceLogs() resourceSliceView[plog.ResourceLogs] {
	return resourceSliceView[plog.ResourceLogs](v)
}

func (v logsView) LogRecordCount() int {
	count := 0
	for _, sl := range v.resource.ScopeLogs().All() {
		count += sl.LogRecords().Len()
	}
	return count
}

type metricsView struct{ resource pmetric.ResourceMetrics }

func (v metricsView) ResourceMetrics() resourceSliceView[pmetric.ResourceMetrics] {
	return resourceSliceView[pmetric.ResourceMetrics](v)
}

func (v metricsView) MetricCount() int {
	count := 0
	for _, sm := range v.resource.ScopeMetrics().All() {
		count += sm.Metrics().Len()
	}
	return count
}

func (v metricsView) DataPointCount() int {
	count := 0
	for _, sm := range v.resource.ScopeMetrics().All() {
		for _, m := range sm.Metrics().All() {
			switch m.Type() {
			case pmetric.MetricTypeGauge:
				count += m.Gauge().DataPoints().Len()
			case pmetric.MetricTypeSum:
				count += m.Sum().DataPoints().Len()
			case pmetric.MetricTypeHistogram:
				count += m.Histogram().DataPoints().Len()
			case pmetric.MetricTypeExponentialHistogram:
				count += m.ExponentialHistogram().DataPoints().Len()
			case pmetric.MetricTypeSummary:
				count += m.Summary().DataPoints().Len()
			}
		}
	}
	return count
}

type tracesView struct{ resource ptrace.ResourceSpans }

func (v tracesView) ResourceSpans() resourceSliceView[ptrace.ResourceSpans] {
	return resourceSliceView[ptrace.ResourceSpans](v)
}

func (v tracesView) SpanCount() int {
	count := 0
	for _, ss := range v.resource.ScopeSpans().All() {
		count += ss.Spans().Len()
	}
	return count
}

// The following functions return resource entry i of a template's root object,
// which is either the whole payload (request scope) or a view (resource scope).
// ok is false when the payload has no resource entries.

func resourceLogsAt(logs any, i int) (rl plog.ResourceLogs, ok bool, err error) {
	switch l := logs.(type) {
	case plog.Logs:
		if l.ResourceLogs().Len() == 0 {
			return rl, false, nil
		}
		return l.ResourceLogs().At(i), true, nil
	case logsView:
		rl, err = l.ResourceLogs().At(i)
		return rl, err == nil, err
	default:
		return rl, false, fmt.Errorf("expected logs, got %T", logs)
	}
}

func resourceMetricsAt(metrics any, i int) (rm pmetric.ResourceMetrics, ok bool, err error) {
	switch m := metrics.(type) {
	case pmetric.Metrics:
		if m.ResourceMetrics().Len() == 0 {
			return rm, false, nil
		}
		return m.ResourceMetrics().At(i), true, nil
	case metricsView:
		rm, err = m.ResourceMetrics().At(i)
		return rm, err == nil, err
	default:
		return rm, false, fmt.Errorf("expected metrics, got %T", metrics)
	}
}

func resourceSpansAt(traces any, i int) (rs ptrace.ResourceSpans, ok bool, err error) {
	switch t := traces.(type) {
	case ptrace.Traces:
		if t.ResourceSpans().Len() == 0 {
			return rs, false, nil
		}
		return t.ResourceSpans().At(i), true, nil
	case tracesView:
		rs, err = t.ResourceSpans().At(i)
		return rs, err == nil, err
	default:
		return rs, false, fmt.Errorf("expected traces, got %T", traces)
	}
}
