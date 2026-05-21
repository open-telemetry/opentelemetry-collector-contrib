// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// openDB creates a new database connection pool.
func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

// ttlClause returns the time_to_live_in_seconds WITH-clause fragment for Hologres DDL.
// Hologres uses table-level TTL via time_to_live_in_seconds (in seconds) for non-partitioned
// tables. Returns an empty string when ttl <= 0.
func ttlClause(ttl time.Duration) string {
	if ttl <= 0 {
		return ""
	}
	seconds := int64(ttl.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf(",\n    time_to_live_in_seconds = '%d'", seconds)
}

// createTracesTable creates the traces table in Hologres.
func createTracesTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"           TIMESTAMPTZ NOT NULL,
    trace_id              TEXT NOT NULL,
    span_id               TEXT NOT NULL,
    parent_span_id        TEXT DEFAULT '',
    trace_state           TEXT DEFAULT '',
    span_name             TEXT NOT NULL DEFAULT '',
    span_kind             TEXT DEFAULT '',
    service_name          TEXT NOT NULL DEFAULT '',
    resource_attributes   JSONB,
    scope_name            TEXT DEFAULT '',
    scope_version         TEXT DEFAULT '',
    span_attributes       JSONB,
    duration              BIGINT DEFAULT 0,
    status_code           TEXT DEFAULT '',
    status_message        TEXT DEFAULT '',
    events                JSONB,
    links                 JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'trace_id',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,span_kind,status_code',
    dictionary_encoding_columns = 'service_name:auto,span_name:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create traces table: %w", err)
	}

	// Enable JSONB columnar optimization (executed separately, not in a transaction).
	alterQueries := []string{
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN resource_attributes SET (enable_columnar_type = ON)`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN span_attributes SET (enable_columnar_type = ON)`, tableName),
	}
	for _, q := range alterQueries {
		// Ignore errors (may already be set).
		db.ExecContext(ctx, q)
	}

	return nil
}

// createLogsTable creates the logs table in Hologres.
func createLogsTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"           TIMESTAMPTZ NOT NULL,
    trace_id              TEXT DEFAULT '',
    span_id               TEXT DEFAULT '',
    trace_flags           INTEGER DEFAULT 0,
    severity_text         TEXT DEFAULT '',
    severity_number       INTEGER DEFAULT 0,
    service_name          TEXT NOT NULL DEFAULT '',
    body                  TEXT DEFAULT '',
    resource_attributes   JSONB,
    scope_name            TEXT DEFAULT '',
    scope_version         TEXT DEFAULT '',
    scope_attributes      JSONB,
    log_attributes        JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'service_name',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,severity_text',
    dictionary_encoding_columns = 'service_name:auto,severity_text:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	// Create full-text index (executed separately, not in a transaction).
	indexName := strings.ReplaceAll(tableName, ".", "_") + "_body_idx"
	indexQuery := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s USING FULLTEXT (body) WITH (tokenizer = 'standard')`, indexName, tableName)
	// Ignore errors (may already exist or version may not support it).
	db.ExecContext(ctx, indexQuery)

	// Enable JSONB columnar optimization.
	alterQueries := []string{
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN resource_attributes SET (enable_columnar_type = ON)`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN scope_attributes SET (enable_columnar_type = ON)`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN log_attributes SET (enable_columnar_type = ON)`, tableName),
	}
	for _, q := range alterQueries {
		db.ExecContext(ctx, q)
	}

	return nil
}

// createMetricsTables creates all five metric type tables in Hologres.
func createMetricsTables(ctx context.Context, db *sql.DB, metricsTableName string, ttl time.Duration) error {
	creators := []struct {
		suffix string
		fn     func(context.Context, *sql.DB, string, time.Duration) error
	}{
		{"gauge", createMetricsGaugeTable},
		{"sum", createMetricsSumTable},
		{"histogram", createMetricsHistogramTable},
		{"summary", createMetricsSummaryTable},
		{"exp_histogram", createMetricsExpHistogramTable},
	}

	for _, c := range creators {
		tableName := fmt.Sprintf("%s_%s", metricsTableName, c.suffix)
		if err := c.fn(ctx, db, tableName, ttl); err != nil {
			return err
		}
	}
	return nil
}

// createMetricsGaugeTable creates the gauge metrics table.
func createMetricsGaugeTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"           TIMESTAMPTZ NOT NULL,
    metric_name           TEXT NOT NULL,
    service_name          TEXT NOT NULL DEFAULT '',
    value                 DOUBLE PRECISION,
    flags                 INTEGER DEFAULT 0,
    resource_attributes   JSONB,
    scope_name            TEXT DEFAULT '',
    scope_version         TEXT DEFAULT '',
    scope_attributes      JSONB,
    attributes            JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'service_name',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,metric_name',
    dictionary_encoding_columns = 'service_name:auto,metric_name:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create gauge metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsSumTable creates the sum metrics table.
func createMetricsSumTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"                TIMESTAMPTZ NOT NULL,
    start_timestamp            TIMESTAMPTZ,
    metric_name                TEXT NOT NULL,
    service_name               TEXT NOT NULL DEFAULT '',
    value                      DOUBLE PRECISION,
    flags                      INTEGER DEFAULT 0,
    is_monotonic               BOOLEAN DEFAULT false,
    aggregation_temporality    TEXT DEFAULT '',
    resource_attributes        JSONB,
    scope_name                 TEXT DEFAULT '',
    scope_version              TEXT DEFAULT '',
    scope_attributes           JSONB,
    attributes                 JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'service_name',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,metric_name',
    dictionary_encoding_columns = 'service_name:auto,metric_name:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create sum metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsHistogramTable creates the histogram metrics table.
func createMetricsHistogramTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"                TIMESTAMPTZ NOT NULL,
    start_timestamp            TIMESTAMPTZ,
    metric_name                TEXT NOT NULL,
    service_name               TEXT NOT NULL DEFAULT '',
    count                      BIGINT DEFAULT 0,
    sum                        DOUBLE PRECISION,
    min                        DOUBLE PRECISION,
    max                        DOUBLE PRECISION,
    flags                      INTEGER DEFAULT 0,
    bucket_counts              TEXT DEFAULT '',
    explicit_bounds            TEXT DEFAULT '',
    aggregation_temporality    TEXT DEFAULT '',
    resource_attributes        JSONB,
    scope_name                 TEXT DEFAULT '',
    scope_version              TEXT DEFAULT '',
    scope_attributes           JSONB,
    attributes                 JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'service_name',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,metric_name',
    dictionary_encoding_columns = 'service_name:auto,metric_name:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create histogram metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsSummaryTable creates the summary metrics table.
func createMetricsSummaryTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"           TIMESTAMPTZ NOT NULL,
    start_timestamp       TIMESTAMPTZ,
    metric_name           TEXT NOT NULL,
    service_name          TEXT NOT NULL DEFAULT '',
    count                 BIGINT DEFAULT 0,
    sum                   DOUBLE PRECISION,
    flags                 INTEGER DEFAULT 0,
    quantile_values       TEXT DEFAULT '',
    quantile_counts       TEXT DEFAULT '',
    resource_attributes   JSONB,
    scope_name            TEXT DEFAULT '',
    scope_version         TEXT DEFAULT '',
    scope_attributes      JSONB,
    attributes            JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'service_name',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,metric_name',
    dictionary_encoding_columns = 'service_name:auto,metric_name:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create summary metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsExpHistogramTable creates the exponential histogram metrics table.
func createMetricsExpHistogramTable(ctx context.Context, db *sql.DB, tableName string, ttl time.Duration) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    "timestamp"                TIMESTAMPTZ NOT NULL,
    start_timestamp            TIMESTAMPTZ,
    metric_name                TEXT NOT NULL,
    service_name               TEXT NOT NULL DEFAULT '',
    count                      BIGINT DEFAULT 0,
    sum                        DOUBLE PRECISION,
    min                        DOUBLE PRECISION,
    max                        DOUBLE PRECISION,
    scale                      INTEGER DEFAULT 0,
    zero_count                 BIGINT DEFAULT 0,
    flags                      INTEGER DEFAULT 0,
    positive_offset            INTEGER DEFAULT 0,
    positive_bucket_counts     TEXT DEFAULT '',
    negative_offset            INTEGER DEFAULT 0,
    negative_bucket_counts     TEXT DEFAULT '',
    aggregation_temporality    TEXT DEFAULT '',
    resource_attributes        JSONB,
    scope_name                 TEXT DEFAULT '',
    scope_version              TEXT DEFAULT '',
    scope_attributes           JSONB,
    attributes                 JSONB
)
WITH (
    orientation = 'column',
    distribution_key = 'service_name',
    clustering_key = '"timestamp":asc',
    bitmap_columns = 'service_name,metric_name',
    dictionary_encoding_columns = 'service_name:auto,metric_name:auto'%s
)`, tableName, ttlClause(ttl))

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create exponential histogram metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// enableJSONBColumnar enables columnar type optimization for JSONB columns.
// Errors are ignored since the setting may already exist or the version may not support it.
func enableJSONBColumnar(ctx context.Context, db *sql.DB, tableName string, columns ...string) {
	for _, col := range columns {
		q := fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s SET (enable_columnar_type = ON)`, tableName, col)
		db.ExecContext(ctx, q)
	}
}

// attributesToJSON converts pdata attributes to JSON bytes for JSONB storage.
func attributesToJSON(attrs pcommon.Map) ([]byte, error) {
	m := make(map[string]any, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		m[k] = valueToInterface(v)
		return true
	})
	return json.Marshal(m)
}

// valueToInterface converts a pcommon.Value to a Go interface{}.
func valueToInterface(v pcommon.Value) any {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return v.Int()
	case pcommon.ValueTypeDouble:
		return v.Double()
	case pcommon.ValueTypeBool:
		return v.Bool()
	case pcommon.ValueTypeBytes:
		return v.Bytes().AsRaw()
	case pcommon.ValueTypeSlice:
		slice := v.Slice()
		result := make([]any, slice.Len())
		for i := range slice.Len() {
			result[i] = valueToInterface(slice.At(i))
		}
		return result
	case pcommon.ValueTypeMap:
		m := make(map[string]any)
		v.Map().Range(func(k string, val pcommon.Value) bool {
			m[k] = valueToInterface(val)
			return true
		})
		return m
	default:
		return v.AsString()
	}
}

// getServiceName extracts service.name from resource attributes.
func getServiceName(resource pcommon.Resource) string {
	attrs := resource.Attributes()
	if v, ok := attrs.Get("service.name"); ok {
		return v.AsString()
	}
	return "unknown"
}
