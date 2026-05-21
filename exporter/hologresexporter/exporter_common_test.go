// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	err = createTracesTable(context.Background(), db, "test_traces", 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateTracesTable_WithTTL(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	err = createTracesTable(context.Background(), db, "test_traces", 7*24*time.Hour)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateTracesTable_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(fmt.Errorf("permission denied"))

	err = createTracesTable(context.Background(), db, "test_traces", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCreateLogsTable_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	err = createLogsTable(context.Background(), db, "test_logs", 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateLogsTable_WithTTL(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	err = createLogsTable(context.Background(), db, "test_logs", 24*time.Hour)
	require.NoError(t, err)
}

func TestCreateLogsTable_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(fmt.Errorf("table error"))

	err = createLogsTable(context.Background(), db, "test_logs", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table error")
}

// expectMetricTableCreation sets expectations for one metric sub-table creation:
// 1 CREATE TABLE + 3 ALTER TABLE (enableJSONBColumnar).
func expectMetricTableCreation(mock sqlmock.Sqlmock) {
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestCreateMetricsTables_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// 5 metric tables: gauge, sum, histogram, summary, exp_histogram
	for range 5 {
		expectMetricTableCreation(mock)
	}

	err = createMetricsTables(context.Background(), db, "otel_metrics", 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateMetricsTables_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// First table (gauge) fails
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(fmt.Errorf("creation failed"))

	err = createMetricsTables(context.Background(), db, "otel_metrics", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creation failed")
}

func TestCreateMetricSubTable_Errors(t *testing.T) {
	tests := []struct {
		name string
		fn   func(context.Context, *sql.DB, string, time.Duration) error
	}{
		{"gauge", createMetricsGaugeTable},
		{"sum", createMetricsSumTable},
		{"histogram", createMetricsHistogramTable},
		{"summary", createMetricsSummaryTable},
		{"exp_histogram", createMetricsExpHistogramTable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectExec("CREATE TABLE IF NOT EXISTS").WillReturnError(fmt.Errorf("error"))

			err = tt.fn(context.Background(), db, "test_table", 0)
			require.Error(t, err)
		})
	}
}

func TestEnableJSONBColumnar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE").WillReturnResult(sqlmock.NewResult(0, 0))

	enableJSONBColumnar(context.Background(), db, "test_table", "col1", "col2")

	require.NoError(t, mock.ExpectationsWereMet())
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
	// Use a DSN that pgx can parse but connect fails immediately (port 1 is closed).
	_, err := openDB("postgresql://user:pass@localhost:1/db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping database")
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
