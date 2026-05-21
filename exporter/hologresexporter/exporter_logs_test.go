// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

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

func TestPushLogData_WithDB(t *testing.T) {
	db := newMockPgxDB()

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	ld := newTestLogs()
	err := exp.pushLogData(t.Context(), ld)
	require.NoError(t, err)

	require.Len(t, db.copyFromCalls, 1)
	call := db.copyFromCalls[0]
	assert.Equal(t, "test_logs", call.table[0])
	assert.Equal(t, logColumns, call.columns)
	require.Len(t, call.rows, 1)
	assert.Len(t, call.rows[0], len(logColumns))
}

func TestPushLogData_DBError(t *testing.T) {
	db := newMockPgxDB()
	db.queueCopyFromErr(errors.New("connection refused"))

	cfg := &Config{LogsTableName: "test_logs"}
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	ld := newTestLogs()
	err := exp.pushLogData(t.Context(), ld)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
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
		db := newMockPgxDB()
		exp := &logsExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
			db:     db,
		}
		err := exp.shutdown(context.Background())
		require.NoError(t, err)
		assert.True(t, db.closed)
	})
}

func TestLogsExporter_Start_DBError(t *testing.T) {
	exp := &logsExporter{
		logger: zap.NewNop(),
		cfg: &Config{
			DSN: "postgresql://user:pass@localhost:1/db",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := exp.start(ctx, nil)
	require.Error(t, err)
}

func TestPushLogData_TimestampFallbackWithDB(t *testing.T) {
	db := newMockPgxDB()

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

	err := exp.pushLogData(t.Context(), ld)
	require.NoError(t, err)

	require.Len(t, db.copyFromCalls, 1)
	// First column is timestamp; verify the fallback was applied.
	row := db.copyFromCalls[0].rows[0]
	tsAny := row[0]
	got, ok := tsAny.(time.Time)
	require.True(t, ok, "expected timestamp column to be time.Time")
	assert.True(t, got.Equal(observed), "expected observed timestamp")
}

func TestLogColumnsLength(t *testing.T) {
	// Sanity check: column list matches the historical 13-column DDL.
	assert.Len(t, logColumns, 13)
}
