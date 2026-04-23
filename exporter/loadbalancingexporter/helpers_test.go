// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestFailedTracesFromError(t *testing.T) {
	input := twoServicesWithSameTraceID()
	partial := ptrace.NewTraces()
	input.ResourceSpans().At(0).CopyTo(partial.ResourceSpans().AppendEmpty())

	failed := failedTracesFromError(consumererror.NewTraces(errors.New("partial failure"), partial), input)
	require.Equal(t, partial.SpanCount(), failed.SpanCount())

	failed = failedTracesFromError(errors.New("total failure"), input)
	require.Equal(t, input.SpanCount(), failed.SpanCount())
}

func TestFailedLogsFromError(t *testing.T) {
	input := logsWithTwoRecords()
	partial := logsWithOneRecord()

	failed := failedLogsFromError(consumererror.NewLogs(errors.New("partial failure"), partial), input)
	require.Equal(t, partial.LogRecordCount(), failed.LogRecordCount())

	failed = failedLogsFromError(errors.New("total failure"), input)
	require.Equal(t, input.LogRecordCount(), failed.LogRecordCount())
}

func TestFailedMetricsFromError(t *testing.T) {
	input := metricsWithTwoDataPoints()
	partial := metricsWithOneDataPoint()

	failed := failedMetricsFromError(consumererror.NewMetrics(errors.New("partial failure"), partial), input)
	require.Equal(t, partial.DataPointCount(), failed.DataPointCount())

	failed = failedMetricsFromError(errors.New("total failure"), input)
	require.Equal(t, input.DataPointCount(), failed.DataPointCount())
}

func logsWithTwoRecords() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()
	sl.LogRecords().AppendEmpty()

	return logs
}

func logsWithOneRecord() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()

	return logs
}

func metricsWithTwoDataPoints() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sum := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptySum()
	sum.DataPoints().AppendEmpty().SetIntValue(1)
	sum.DataPoints().AppendEmpty().SetIntValue(2)

	return metrics
}

func metricsWithOneDataPoint() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sum := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptySum()
	sum.DataPoints().AppendEmpty().SetIntValue(1)

	return metrics
}
