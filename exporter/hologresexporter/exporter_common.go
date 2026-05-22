// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// pgxDB defines the database operations used by the Hologres exporters.
// It is satisfied by *hologresPool in production code; tests can supply a
// lightweight implementation (mockPgxDB). CopyFrom uses the PostgreSQL
// binary COPY protocol with STREAM_MODE TRUE, which Hologres requires for
// binary-format imports (FIXED COPY).
type pgxDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Ping(ctx context.Context) error
	Close()
}

// hologresPool wraps *pgxpool.Pool and implements pgxDB.
//
// pgx's stock Conn.CopyFrom hardcodes a SQL of "copy ... from stdin binary"
// without WITH options, but Hologres rejects binary-format COPY unless the
// statement also specifies STREAM_MODE TRUE (FIXED COPY mode). So this
// wrapper drives the COPY protocol at the pgconn layer with a custom SQL
// that includes STREAM_MODE TRUE, while still using the standard pgx binary
// row encoder for value serialization.
type hologresPool struct {
	pool *pgxpool.Pool
}

func (h *hologresPool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return h.pool.Exec(ctx, sql, arguments...)
}

func (h *hologresPool) Ping(ctx context.Context) error {
	return h.pool.Ping(ctx)
}

func (h *hologresPool) Close() {
	h.pool.Close()
}

// binaryCopyHeader is the fixed file header prepended to every binary COPY
// stream: 11-byte signature + int32 flags(0) + int32 header-extension-len(0).
var binaryCopyHeader = []byte("PGCOPY\n\xff\r\n\x00\x00\x00\x00\x00\x00\x00\x00\x00")

// CopyFrom performs a binary-format PostgreSQL COPY with Hologres's
// STREAM_MODE TRUE option. It serializes rows on the fly and streams them to
// the server via the pgconn-level CopyFrom API.
func (h *hologresPool) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	conn, err := h.pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	quotedCols := make([]string, len(columnNames))
	for i, col := range columnNames {
		quotedCols[i] = pgx.Identifier{col}.Sanitize()
	}
	quotedColList := strings.Join(quotedCols, ", ")
	quotedTable := tableName.Sanitize()

	// Resolve column type OIDs by preparing a SELECT against the target table.
	// Using the unnamed prepared statement ("") avoids polluting the statement
	// cache and is auto-deallocated by the next prepare/execute on the connection.
	pgxConn := conn.Conn()
	sd, err := pgxConn.Prepare(ctx, "", fmt.Sprintf("select %s from %s", quotedColList, quotedTable))
	if err != nil {
		return 0, fmt.Errorf("failed to describe %s for COPY: %w", quotedTable, err)
	}
	if len(sd.Fields) != len(columnNames) {
		return 0, fmt.Errorf("column count mismatch for %s: described %d, expected %d", quotedTable, len(sd.Fields), len(columnNames))
	}
	typeMap := pgxConn.TypeMap()

	// Encode rows into the PostgreSQL binary COPY wire format in memory.
	buf := make([]byte, 0, 4096)
	buf = append(buf, binaryCopyHeader...)

	var count int64
	for rowSrc.Next() {
		values, verr := rowSrc.Values()
		if verr != nil {
			return 0, verr
		}
		if len(values) != len(columnNames) {
			return 0, fmt.Errorf("expected %d values, got %d", len(columnNames), len(values))
		}
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(values)))
		for i, v := range values {
			buf, err = appendBinaryCopyField(typeMap, buf, sd.Fields[i].DataTypeOID, v)
			if err != nil {
				return 0, fmt.Errorf("failed to encode column %s: %w", columnNames[i], err)
			}
		}
		count++
	}
	if rerr := rowSrc.Err(); rerr != nil {
		return 0, rerr
	}
	// File trailer: int16 = -1.
	buf = binary.BigEndian.AppendUint16(buf, uint16(0xFFFF))

	copySQL := fmt.Sprintf("COPY %s (%s) FROM STDIN WITH (FORMAT binary, STREAM_MODE TRUE)", quotedTable, quotedColList)
	if _, err := pgxConn.PgConn().CopyFrom(ctx, bytes.NewReader(buf), copySQL); err != nil {
		return 0, err
	}
	return count, nil
}

// appendBinaryCopyField appends one binary-format field (int32 length + data,
// or int32 -1 for NULL) to buf.
func appendBinaryCopyField(m *pgtype.Map, buf []byte, oid uint32, arg any) ([]byte, error) {
	lenPos := len(buf)
	buf = binary.BigEndian.AppendUint32(buf, 0xFFFFFFFF) // placeholder for -1 (NULL)
	argBuf, err := m.Encode(oid, pgx.BinaryFormatCode, arg, buf)
	if err != nil {
		return nil, err
	}
	if argBuf == nil {
		// Encoder returned NULL; keep the -1 placeholder.
		return buf, nil
	}
	buf = argBuf
	dataLen := int32(len(buf) - lenPos - 4)
	binary.BigEndian.PutUint32(buf[lenPos:lenPos+4], uint32(dataLen))
	return buf, nil
}

// openDB creates a new pgx connection pool wrapped for Hologres compatibility.
func openDB(ctx context.Context, dsn string) (*hologresPool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return &hologresPool{pool: pool}, nil
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
func createTracesTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create traces table: %w", err)
	}

	// Enable JSONB columnar optimization (executed separately, not in a transaction).
	alterQueries := []string{
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN resource_attributes SET (enable_columnar_type = ON)`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN span_attributes SET (enable_columnar_type = ON)`, tableName),
	}
	for _, q := range alterQueries {
		// Ignore errors (may already be set).
		_, _ = db.Exec(ctx, q)
	}

	return nil
}

// createLogsTable creates the logs table in Hologres.
func createLogsTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	// Create full-text index (executed separately, not in a transaction).
	indexName := strings.ReplaceAll(tableName, ".", "_") + "_body_idx"
	indexQuery := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s USING FULLTEXT (body) WITH (tokenizer = 'standard')`, indexName, tableName)
	// Ignore errors (may already exist or version may not support it).
	_, _ = db.Exec(ctx, indexQuery)

	// Enable JSONB columnar optimization.
	alterQueries := []string{
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN resource_attributes SET (enable_columnar_type = ON)`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN scope_attributes SET (enable_columnar_type = ON)`, tableName),
		fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN log_attributes SET (enable_columnar_type = ON)`, tableName),
	}
	for _, q := range alterQueries {
		_, _ = db.Exec(ctx, q)
	}

	return nil
}

// createMetricsTables creates all five metric type tables in Hologres.
func createMetricsTables(ctx context.Context, db pgxDB, metricsTableName string, ttl time.Duration) error {
	creators := []struct {
		suffix string
		fn     func(context.Context, pgxDB, string, time.Duration) error
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
func createMetricsGaugeTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create gauge metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsSumTable creates the sum metrics table.
func createMetricsSumTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create sum metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsHistogramTable creates the histogram metrics table.
func createMetricsHistogramTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create histogram metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsSummaryTable creates the summary metrics table.
func createMetricsSummaryTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create summary metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// createMetricsExpHistogramTable creates the exponential histogram metrics table.
func createMetricsExpHistogramTable(ctx context.Context, db pgxDB, tableName string, ttl time.Duration) error {
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

	if _, err := db.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create exponential histogram metrics table: %w", err)
	}

	enableJSONBColumnar(ctx, db, tableName, "resource_attributes", "scope_attributes", "attributes")
	return nil
}

// enableJSONBColumnar enables columnar type optimization for JSONB columns.
// Errors are ignored since the setting may already exist or the version may not support it.
func enableJSONBColumnar(ctx context.Context, db pgxDB, tableName string, columns ...string) {
	for _, col := range columns {
		q := fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s SET (enable_columnar_type = ON)`, tableName, col)
		_, _ = db.Exec(ctx, q)
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
