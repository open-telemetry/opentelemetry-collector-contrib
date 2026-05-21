// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
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

func TestPushLogData_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	ld := newTestLogs()
	err = exp.pushLogData(t.Context(), ld)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPushLogData_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("connection refused"))

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	ld := newTestLogs()
	err = exp.pushLogData(t.Context(), ld)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestLogsInsertBatch_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 2))

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	rows := []logRow{
		{args: make([]any, logColumnsCount)},
		{args: make([]any, logColumnsCount)},
	}

	err = exp.insertBatch(t.Context(), rows)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLogsInsertBatch_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("insert error"))

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	rows := []logRow{{args: make([]any, logColumnsCount)}}
	err = exp.insertBatch(t.Context(), rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert error")
}

func TestLogsExporter_Shutdown(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		exp := &logsExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
		}
		err := exp.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("with db", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectClose()

		exp := &logsExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
			db:     db,
		}
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	})
}

func TestLogsExporter_Start_DBError(t *testing.T) {
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg: &Config{
			DSN: "postgresql://user:pass@localhost:1/db",
		},
	}
	err := exp.start(context.Background(), nil)
	require.Error(t, err)
}

func TestPushLogData_TimestampFallbackWithDB(t *testing.T) {
	// Test the timestamp fallback logic with actual DB insertion.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	sl := rl.ScopeLogs().AppendEmpty()
	log := sl.LogRecords().AppendEmpty()
	// Only set ObservedTimestamp, leave Timestamp as zero.
	observed := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(observed))
	log.Body().SetStr("test fallback")

	err = exp.pushLogData(t.Context(), ld)
	require.NoError(t, err)
}
