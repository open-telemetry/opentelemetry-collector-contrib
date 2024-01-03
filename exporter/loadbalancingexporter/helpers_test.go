// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestMergeTracesTwoEmpty(t *testing.T) {
	expectedEmpty := ptrace.NewTraces()
	trace1 := ptrace.NewTraces()
	trace2 := ptrace.NewTraces()

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedEmpty, mergedTraces)
}

func TestMergeTracesSingleEmpty(t *testing.T) {
	expectedTraces := simpleTraces()

	trace1 := ptrace.NewTraces()
	trace2 := simpleTraces()

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedTraces, mergedTraces)
}

func TestMergeTraces(t *testing.T) {
	expectedTraces := ptrace.NewTraces()
	expectedTraces.ResourceSpans().EnsureCapacity(3)
	aspans := expectedTraces.ResourceSpans().AppendEmpty()
	aspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-1")
	aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	bspans := expectedTraces.ResourceSpans().AppendEmpty()
	bspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-2")
	bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 2})
	cspans := expectedTraces.ResourceSpans().AppendEmpty()
	cspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-3")
	cspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 3})

	trace1 := ptrace.NewTraces()
	trace1.ResourceSpans().EnsureCapacity(2)
	t1aspans := trace1.ResourceSpans().AppendEmpty()
	t1aspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-1")
	t1aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	t1bspans := trace1.ResourceSpans().AppendEmpty()
	t1bspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-2")
	t1bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 2})

	trace2 := ptrace.NewTraces()
	trace2.ResourceSpans().EnsureCapacity(1)
	t2cspans := trace2.ResourceSpans().AppendEmpty()
	t2cspans.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-3")
	t2cspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 3})

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedTraces, mergedTraces)
}

func TestMergeMetricsTwoEmpty(t *testing.T) {
	expectedEmpty := pmetric.NewMetrics()
	metric1 := pmetric.NewMetrics()
	metric2 := pmetric.NewMetrics()

	mergedMetrics := mergeMetrics(metric1, metric2)

	require.Equal(t, expectedEmpty, mergedMetrics)
}

func TestMergeMetricsSingleEmpty(t *testing.T) {
	expectedMetrics := simpleMetricsWithResource()

	metric1 := pmetric.NewMetrics()
	metric2 := simpleMetricsWithResource()

	mergedMetrics := mergeMetrics(metric1, metric2)

	require.Equal(t, expectedMetrics, mergedMetrics)
}

func TestMergeMetrics(t *testing.T) {
	expectedMetrics := pmetric.NewMetrics()
	expectedMetrics.ResourceMetrics().EnsureCapacity(3)
	ametrics := expectedMetrics.ResourceMetrics().AppendEmpty()
	ametrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-1")
	ametrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m1")
	bmetrics := expectedMetrics.ResourceMetrics().AppendEmpty()
	bmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-2")
	bmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m1")
	cmetrics := expectedMetrics.ResourceMetrics().AppendEmpty()
	cmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-3")
	cmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m2")

	metric1 := pmetric.NewMetrics()
	metric1.ResourceMetrics().EnsureCapacity(2)
	m1ametrics := metric1.ResourceMetrics().AppendEmpty()
	m1ametrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-1")
	m1ametrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m1")
	m1bmetrics := metric1.ResourceMetrics().AppendEmpty()
	m1bmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-2")
	m1bmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m1")

	metric2 := pmetric.NewMetrics()
	metric2.ResourceMetrics().EnsureCapacity(1)
	m2cmetrics := metric2.ResourceMetrics().AppendEmpty()
	m2cmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, "service-name-3")
	m2cmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m2")

	mergedMetrics := mergeMetrics(metric1, metric2)

	require.Equal(t, expectedMetrics, mergedMetrics)
}
