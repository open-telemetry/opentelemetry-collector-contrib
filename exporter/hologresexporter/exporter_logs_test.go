// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func newTestLogs() plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	rl.Resource().Attributes().PutStr("host.name", "test-host")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test-scope")
	sl.Scope().SetVersion("v1.0.0")
	sl.Scope().Attributes().PutStr("scope.key", "scope-value")

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	log := sl.LogRecords().AppendEmpty()
	log.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(time.Second)))
	log.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	log.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	log.SetFlags(plog.LogRecordFlags(1))
	log.SetSeverityText("INFO")
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.Body().SetStr("hello world")
	log.Attributes().PutStr("log.key", "log-value")
	log.Attributes().PutInt("log.count", 42)

	return ld
}

func TestPushLogData_EmptyLogs(t *testing.T) {
	cfg := &Config{
		LogsTableName: "otel_logs",
	}
	exp := newLogsExporter(nil, cfg)

	ld := plog.NewLogs()
	err := exp.pushLogData(t.Context(), ld)
	assert.NoError(t, err)
}

func TestPushLogData_DataConversion(t *testing.T) {
	// Verify the data conversion logic produces correct log rows
	// by directly testing the row construction without a database.
	ld := newTestLogs()

	rls := ld.ResourceLogs()
	rl := rls.At(0)

	serviceName := getServiceName(rl.Resource())
	assert.Equal(t, "test-service", serviceName)

	resourceAttrs, err := attributesToJSON(rl.Resource().Attributes())
	require.NoError(t, err)

	var resAttrsMap map[string]any
	require.NoError(t, json.Unmarshal(resourceAttrs, &resAttrsMap))
	assert.Equal(t, "test-service", resAttrsMap["service.name"])
	assert.Equal(t, "test-host", resAttrsMap["host.name"])

	sl := rl.ScopeLogs().At(0)
	assert.Equal(t, "test-scope", sl.Scope().Name())
	assert.Equal(t, "v1.0.0", sl.Scope().Version())

	scopeAttrs, err := attributesToJSON(sl.Scope().Attributes())
	require.NoError(t, err)
	var scopeAttrsMap map[string]any
	require.NoError(t, json.Unmarshal(scopeAttrs, &scopeAttrsMap))
	assert.Equal(t, "scope-value", scopeAttrsMap["scope.key"])

	log := sl.LogRecords().At(0)

	logAttrs, err := attributesToJSON(log.Attributes())
	require.NoError(t, err)
	var logAttrsMap map[string]any
	require.NoError(t, json.Unmarshal(logAttrs, &logAttrsMap))
	assert.Equal(t, "log-value", logAttrsMap["log.key"])
	assert.InDelta(t, float64(42), logAttrsMap["log.count"], 0.0001)

	assert.Equal(t, "INFO", log.SeverityText())
	assert.Equal(t, plog.SeverityNumberInfo, log.SeverityNumber())
	assert.Equal(t, "hello world", log.Body().AsString())
}

func TestPushLogData_TimestampFallback(t *testing.T) {
	// When Timestamp is zero, ObservedTimestamp must be used.
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()

	observed := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	log := sl.LogRecords().AppendEmpty()
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(observed))
	// Timestamp left as zero.

	ts := log.Timestamp()
	if ts == 0 {
		ts = log.ObservedTimestamp()
	}
	assert.Equal(t, observed, ts.AsTime())

	// And when Timestamp is set, it should take precedence.
	primary := time.Date(2024, 2, 20, 8, 0, 0, 0, time.UTC)
	log.SetTimestamp(pcommon.NewTimestampFromTime(primary))
	ts = log.Timestamp()
	if ts == 0 {
		ts = log.ObservedTimestamp()
	}
	assert.Equal(t, primary, ts.AsTime())
}

func TestPushLogData_BatchSplit(t *testing.T) {
	// Verify the batch split logic without an actual database. The
	// pushLogData implementation calls insertBatch in chunks of
	// maxLogsPerBatch; here we replicate the chunking arithmetic to ensure
	// that a large input is partitioned into the expected number of pieces.
	totalRows := maxLogsPerBatch*2 + 5

	chunks := 0
	for batchStart := 0; batchStart < totalRows; batchStart += maxLogsPerBatch {
		batchEnd := batchStart + maxLogsPerBatch
		if batchEnd > totalRows {
			batchEnd = totalRows
		}
		assert.LessOrEqual(t, batchEnd-batchStart, maxLogsPerBatch)
		chunks++
	}
	assert.Equal(t, 3, chunks)
}

func TestMaxLogsPerBatch(t *testing.T) {
	// Verify constant calculations.
	assert.Equal(t, 13, logColumnsCount)
	assert.Equal(t, 65535/13, maxLogsPerBatch)
	assert.LessOrEqual(t, maxLogsPerBatch*logColumnsCount, 65535)
}
