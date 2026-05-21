// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestTtlClause(t *testing.T) {
	t.Run("zero ttl returns empty", func(t *testing.T) {
		assert.Equal(t, "", ttlClause(0))
	})

	t.Run("negative ttl returns empty", func(t *testing.T) {
		assert.Equal(t, "", ttlClause(-time.Hour))
	})

	t.Run("positive ttl", func(t *testing.T) {
		result := ttlClause(30 * 24 * time.Hour)
		assert.Contains(t, result, "time_to_live_in_seconds")
		assert.Contains(t, result, "2592000")
	})

	t.Run("small ttl rounds up to 1", func(t *testing.T) {
		result := ttlClause(500 * time.Millisecond)
		assert.Contains(t, result, "time_to_live_in_seconds")
		assert.Contains(t, result, "'1'")
	})

	t.Run("one hour", func(t *testing.T) {
		result := ttlClause(time.Hour)
		assert.Contains(t, result, "3600")
	})
}

func TestCreateTracesTable_Success(t *testing.T) {
	db := newMockPgxDB()

	err := createTracesTable(context.Background(), db, "test_traces", 0)
	require.NoError(t, err)

	// 1 CREATE TABLE + 2 ALTER TABLE.
	assert.Len(t, db.execCalls, 3)
	assert.Contains(t, db.execCalls[0], "CREATE TABLE IF NOT EXISTS")
	assert.Contains(t, db.execCalls[1], "ALTER TABLE")
	assert.Contains(t, db.execCalls[2], "ALTER TABLE")
}

func TestCreateTracesTable_WithTTL(t *testing.T) {
	db := newMockPgxDB()

	err := createTracesTable(context.Background(), db, "test_traces", 7*24*time.Hour)
	require.NoError(t, err)
	assert.Contains(t, db.execCalls[0], "time_to_live_in_seconds")
}

func TestCreateTracesTable_Error(t *testing.T) {
	db := newMockPgxDB()
	db.expectExec("CREATE TABLE IF NOT EXISTS", errors.New("permission denied"))

	err := createTracesTable(context.Background(), db, "test_traces", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCreateLogsTable_Success(t *testing.T) {
	db := newMockPgxDB()

	err := createLogsTable(context.Background(), db, "test_logs", 0)
	require.NoError(t, err)
	// 1 CREATE TABLE + 1 CREATE INDEX + 3 ALTER TABLE.
	assert.Len(t, db.execCalls, 5)
}

func TestCreateLogsTable_WithTTL(t *testing.T) {
	db := newMockPgxDB()
	err := createLogsTable(context.Background(), db, "test_logs", 24*time.Hour)
	require.NoError(t, err)
	assert.Contains(t, db.execCalls[0], "time_to_live_in_seconds")
}

func TestCreateLogsTable_Error(t *testing.T) {
	db := newMockPgxDB()
	db.expectExec("CREATE TABLE IF NOT EXISTS", errors.New("table error"))

	err := createLogsTable(context.Background(), db, "test_logs", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table error")
}

func TestCreateMetricsTables_Success(t *testing.T) {
	db := newMockPgxDB()

	err := createMetricsTables(context.Background(), db, "otel_metrics", 0)
	require.NoError(t, err)

	// 5 metric tables, each: 1 CREATE + 3 ALTER (enableJSONBColumnar) = 4 execs.
	assert.Len(t, db.execCalls, 20)
}

func TestCreateMetricsTables_Error(t *testing.T) {
	db := newMockPgxDB()
	db.expectExec("CREATE TABLE IF NOT EXISTS", errors.New("creation failed"))

	err := createMetricsTables(context.Background(), db, "otel_metrics", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creation failed")
}

func TestCreateMetricSubTable_Errors(t *testing.T) {
	tests := []struct {
		name string
		fn   func(context.Context, pgxDB, string, time.Duration) error
	}{
		{"gauge", createMetricsGaugeTable},
		{"sum", createMetricsSumTable},
		{"histogram", createMetricsHistogramTable},
		{"summary", createMetricsSummaryTable},
		{"exp_histogram", createMetricsExpHistogramTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newMockPgxDB()
			db.expectExec("CREATE TABLE IF NOT EXISTS", errors.New("error"))

			err := tt.fn(context.Background(), db, "test_table", 0)
			require.Error(t, err)
		})
	}
}

func TestEnableJSONBColumnar(t *testing.T) {
	db := newMockPgxDB()
	enableJSONBColumnar(context.Background(), db, "test_table", "col1", "col2")

	assert.Len(t, db.execCalls, 2)
	assert.Contains(t, db.execCalls[0], "ALTER TABLE")
	assert.Contains(t, db.execCalls[0], "col1")
	assert.Contains(t, db.execCalls[1], "col2")
}

func TestValueToInterface_AllTypes(t *testing.T) {
	// String
	v := pcommon.NewValueStr("hello")
	assert.Equal(t, "hello", valueToInterface(v))

	// Int
	v = pcommon.NewValueInt(42)
	assert.Equal(t, int64(42), valueToInterface(v))

	// Double
	v = pcommon.NewValueDouble(3.14)
	assert.InDelta(t, 3.14, valueToInterface(v), 0.0001)

	// Bool
	v = pcommon.NewValueBool(true)
	assert.Equal(t, true, valueToInterface(v))

	// Bytes
	v = pcommon.NewValueBytes()
	v.Bytes().FromRaw([]byte{1, 2, 3})
	result := valueToInterface(v)
	assert.Equal(t, []byte{1, 2, 3}, result)

	// Slice
	v = pcommon.NewValueSlice()
	v.Slice().AppendEmpty().SetStr("item1")
	v.Slice().AppendEmpty().SetInt(123)
	result = valueToInterface(v)
	slice, ok := result.([]any)
	require.True(t, ok)
	assert.Len(t, slice, 2)
	assert.Equal(t, "item1", slice[0])
	assert.Equal(t, int64(123), slice[1])

	// Map
	v = pcommon.NewValueMap()
	v.Map().PutStr("key", "value")
	v.Map().PutInt("num", 456)
	result = valueToInterface(v)
	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", m["key"])
	assert.Equal(t, int64(456), m["num"])

	// Empty/default
	v = pcommon.NewValueEmpty()
	result = valueToInterface(v)
	assert.Equal(t, "", result)
}

func TestGetServiceName_NoServiceName(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host.name", "test-host")
	assert.Equal(t, "unknown", getServiceName(resource))
}

func TestOpenDB_InvalidDSN(t *testing.T) {
	t.Run("malformed DSN", func(t *testing.T) {
		_, err := openDB(context.Background(), "::not-a-valid-dsn::")
		require.Error(t, err)
	})

	t.Run("unreachable host", func(t *testing.T) {
		// pgxpool can parse this, but the ping fails immediately (port 1 closed).
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := openDB(ctx, "postgresql://user:pass@localhost:1/db")
		require.Error(t, err)
	})
}

func TestAttributesToJSON(t *testing.T) {
	t.Run("empty attributes", func(t *testing.T) {
		attrs := pcommon.NewMap()
		result, err := attributesToJSON(attrs)
		require.NoError(t, err)
		assert.JSONEq(t, "{}", string(result))
	})

	t.Run("mixed attributes", func(t *testing.T) {
		attrs := pcommon.NewMap()
		attrs.PutStr("str", "value")
		attrs.PutInt("int", 42)
		attrs.PutDouble("double", 3.14)
		attrs.PutBool("bool", true)

		result, err := attributesToJSON(attrs)
		require.NoError(t, err)
		assert.Contains(t, string(result), `"str":"value"`)
		assert.Contains(t, string(result), `"int":42`)
		assert.Contains(t, string(result), `"bool":true`)
	})
}
