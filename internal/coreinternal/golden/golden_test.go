// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestWriteMetrics(t *testing.T) {
	metricslice := testMetrics()
	metrics := pmetric.NewMetrics()
	metricslice.CopyTo(metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

	actualFile := filepath.Join(t.TempDir(), "metrics.yaml")
	require.NoError(t, writeMetrics(actualFile, metrics))

	actualBytes, err := os.ReadFile(actualFile)
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.yaml")
	expectedBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		// os.ReadFile adds a '\r' that we don't actually expect
		expectedBytes = bytes.ReplaceAll(expectedBytes, []byte("\r\n"), []byte("\n"))
	}

	require.Equal(t, expectedBytes, actualBytes)
}

func TestReadMetrics(t *testing.T) {
	metricslice := testMetrics()
	expectedMetrics := pmetric.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

	expectedFile := filepath.Join("testdata", "roundtrip", "expected.yaml")
	actualMetrics, err := ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}

func TestRoundTrip(t *testing.T) {
	metricslice := testMetrics()
	expectedMetrics := pmetric.NewMetrics()
	metricslice.CopyTo(expectedMetrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics())

	tempDir := filepath.Join(t.TempDir(), "metrics.yaml")
	require.NoError(t, writeMetrics(tempDir, expectedMetrics))

	actualMetrics, err := ReadMetrics(tempDir)
	require.NoError(t, err)
	require.Equal(t, expectedMetrics, actualMetrics)
}

func testMetrics() pmetric.MetricSlice {
	slice := pmetric.NewMetricSlice()

	// Gauge with two double dps
	metric := slice.AppendEmpty()
	initGauge(metric, "test gauge multi", "multi gauge", "1")
	dps := metric.Gauge().DataPoints()

	dp := dps.AppendEmpty()
	attributes := pcommon.NewMap()
	attributes.PutStr("testKey1", "teststringvalue1")
	attributes.PutStr("testKey2", "testvalue1")
	setDPDoubleVal(dp, 2, attributes, time.Time{})

	dp = dps.AppendEmpty()
	attributes = pcommon.NewMap()
	attributes.PutStr("testKey1", "teststringvalue2")
	attributes.PutStr("testKey2", "testvalue2")
	setDPDoubleVal(dp, 2, attributes, time.Time{})

	// Gauge with one int dp
	metric = slice.AppendEmpty()
	initGauge(metric, "test gauge single", "single gauge", "By")
	dps = metric.Gauge().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pcommon.NewMap()
	attributes.PutStr("testKey2", "teststringvalue2")
	setDPIntVal(dp, 2, attributes, time.Time{})

	// Delta Sum with two int dps
	metric = slice.AppendEmpty()
	initSum(metric, "test delta sum multi", "multi sum", "s", pmetric.AggregationTemporalityDelta, false)
	dps = metric.Sum().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pcommon.NewMap()
	attributes.PutStr("testKey2", "teststringvalue2")
	setDPIntVal(dp, 2, attributes, time.Time{})

	dp = dps.AppendEmpty()
	attributes = pcommon.NewMap()
	attributes.PutStr("testKey2", "teststringvalue2")
	setDPIntVal(dp, 2, attributes, time.Time{})

	// Cumulative Sum with one double dp
	metric = slice.AppendEmpty()
	initSum(metric, "test cumulative sum single", "single sum", "1/s", pmetric.AggregationTemporalityCumulative, true)
	dps = metric.Sum().DataPoints()

	dp = dps.AppendEmpty()
	attributes = pcommon.NewMap()
	setDPDoubleVal(dp, 2, attributes, time.Date(1997, 07, 27, 1, 1, 1, 1, &time.Location{}))
	return slice
}

func setDPDoubleVal(dp pmetric.NumberDataPoint, value float64, attributes pcommon.Map, timeStamp time.Time) {
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeStamp))
	attributes.CopyTo(dp.Attributes())
}

func setDPIntVal(dp pmetric.NumberDataPoint, value int64, attributes pcommon.Map, timeStamp time.Time) {
	dp.SetIntValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeStamp))
	attributes.CopyTo(dp.Attributes())
}

func initGauge(metric pmetric.Metric, name, desc, unit string) {
	metric.SetDescription(desc)
	metric.SetName(name)
	metric.SetUnit(unit)
	metric.SetEmptyGauge()
}

func initSum(metric pmetric.Metric, name, desc, unit string, aggr pmetric.AggregationTemporality, isMonotonic bool) {
	metric.SetDescription(desc)
	metric.SetName(name)
	metric.SetUnit(unit)
	metric.SetEmptySum().SetIsMonotonic(isMonotonic)
	metric.Sum().SetAggregationTemporality(aggr)
}

func TestReadLogs(t *testing.T) {
	expectedLogs := CreateTestLogs()

	expectedFile := filepath.Join("testdata", "logs-roundtrip", "expected.yaml")
	actualLogs, err := ReadLogs(expectedFile)
	require.NoError(t, err)
	require.Equal(t, expectedLogs, actualLogs)
}

func TestWriteLogs(t *testing.T) {
	logs := CreateTestLogs()

	actualFile := filepath.Join(t.TempDir(), "logs.yaml")
	require.NoError(t, writeLogs(actualFile, logs))

	actualBytes, err := os.ReadFile(actualFile)
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "logs-roundtrip", "expected.yaml")
	expectedBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		// os.ReadFile adds a '\r' that we don't actually expect
		expectedBytes = bytes.ReplaceAll(expectedBytes, []byte("\r\n"), []byte("\n"))
	}

	require.Equal(t, expectedBytes, actualBytes)
}

func TestLogsRoundTrip(t *testing.T) {
	expectedLogs := CreateTestLogs()

	tempDir := filepath.Join(t.TempDir(), "logs.yaml")
	require.NoError(t, writeLogs(tempDir, expectedLogs))

	actualLogs, err := ReadLogs(tempDir)
	require.NoError(t, err)
	require.Equal(t, expectedLogs, actualLogs)
}

func CreateTestLogs() plog.Logs {
	logs := plog.NewLogs()

	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("testKey1", "teststringvalue1")
	rl.Resource().Attributes().PutStr("testKey2", "teststringvalue2")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("collector")
	sl.Scope().SetVersion("v0.1.0")

	logRecord := sl.LogRecords().AppendEmpty()
	timestamp := pcommon.NewTimestampFromTime(time.Time{})
	logRecord.SetTimestamp(timestamp)
	logRecord.SetObservedTimestamp(timestamp)
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("TEST")
	logRecord.Body().SetStr("testscopevalue1")
	logRecord.Attributes().PutStr("testKey1", "teststringvalue1")
	logRecord.Attributes().PutStr("testKey2", "teststringvalue2")

	return logs
}

func TestReadTraces(t *testing.T) {
	expectedTraces := CreateTestTraces()

	expectedFile := filepath.Join("testdata", "traces-roundtrip", "expected.yaml")
	actualTraces, err := ReadTraces(expectedFile)
	require.NoError(t, err)
	require.Equal(t, expectedTraces, actualTraces)
}

func TestWriteTraces(t *testing.T) {
	traces := CreateTestTraces()

	actualFile := filepath.Join("./testdata/traces-roundtrip", "traces.yaml")
	require.NoError(t, writeTraces(actualFile, traces))

	actualBytes, err := os.ReadFile(actualFile)
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "traces-roundtrip", "expected.yaml")
	expectedBytes, err := os.ReadFile(expectedFile)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		// os.ReadFile adds a '\r' that we don't actually expect
		expectedBytes = bytes.ReplaceAll(expectedBytes, []byte("\r\n"), []byte("\n"))
	}

	require.Equal(t, expectedBytes, actualBytes)
}

func CreateTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()

	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("testKey1", "teststringvalue1")
	rs.Resource().Attributes().PutStr("testKey2", "teststringvalue2")
	rs.Resource().SetDroppedAttributesCount(1)

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("collector")
	ss.Scope().SetVersion("v0.1.0")

	span := ss.Spans().AppendEmpty()
	span.Attributes().PutStr("testKey1", "teststringvalue1")
	span.Attributes().PutStr("testKey2", "teststringvalue2")
	span.SetDroppedAttributesCount(1)
	timestamp := pcommon.NewTimestampFromTime(time.Time{})
	span.SetStartTimestamp(timestamp)
	span.SetEndTimestamp(timestamp)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.SetName("span1")
	span.SetKind(ptrace.SpanKindServer)
	var parentSpanID [8]byte
	byteSlice, _ := hex.DecodeString("bcff497b5a47310f")
	copy(parentSpanID[:], byteSlice)
	span.SetParentSpanID(pcommon.SpanID(parentSpanID))

	var spanID [8]byte
	byteSlice, _ = hex.DecodeString("fd0da883bb27cd6b")
	copy(spanID[:], byteSlice)
	span.SetSpanID(pcommon.SpanID(spanID))

	var traceID [16]byte
	byteSlice, _ = hex.DecodeString("8c8b1765a7b0acf0b66aa4623fcb7bd5")
	copy(traceID[:], byteSlice)
	span.SetTraceID(pcommon.TraceID(traceID))

	event := span.Events().AppendEmpty()
	event.SetName("Sub span event")

	return traces
}
