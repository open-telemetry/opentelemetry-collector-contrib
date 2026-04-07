// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	// DefaultBatchSize is the max number of rows per multi-value INSERT.
	// Keeps total payload under typical max_allowed_packet (4MB).
	DefaultBatchSize = 500
)

// BatchInsert executes multi-value INSERT statements for high throughput.
//
// insertPrefix: "INSERT INTO `db`.`table` (col1, col2, ...) VALUES "
// valuePlaceholder: "(?, ?, ...)" — one row's placeholder
// rows: each element is one row's parameter values
//
// Rows are split into chunks of batchSize and each chunk is executed as:
//
//	INSERT INTO ... VALUES (?,?,...), (?,?,...), ...
func BatchInsert(ctx context.Context, db *sql.DB, insertPrefix, valuePlaceholder string, rows [][]any, batchSize int) error {
	if len(rows) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	for start := 0; start < len(rows); start += batchSize {
		end := start + batchSize
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]

		if err := execBatch(ctx, db, insertPrefix, valuePlaceholder, chunk); err != nil {
			return err
		}
	}
	return nil
}

func execBatch(ctx context.Context, db *sql.DB, insertPrefix, valuePlaceholder string, chunk [][]any) error {
	n := len(chunk)
	// Build "VALUES (?,?,...), (?,?,...), ..."
	var sb strings.Builder
	sb.WriteString(insertPrefix)
	for i := range n {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(valuePlaceholder)
	}

	// Flatten all args
	colsPerRow := len(chunk[0])
	args := make([]any, 0, n*colsPerRow)
	for _, row := range chunk {
		args = append(args, row...)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)
	if err != nil {
		return fmt.Errorf("batch insert (%d rows): %w", n, err)
	}
	return nil
}

// BuildInsertPrefix returns "INSERT INTO `db`.`table` (col1, col2, ...) VALUES "
// from a SQL template like "INSERT INTO `%s`.`%s` (...) VALUES (?, ?, ...)"
func BuildInsertPrefix(sqlTemplate, database, tableName string) string {
	rendered := fmt.Sprintf(sqlTemplate, database, tableName)
	// Find ") VALUES (" which marks the boundary between column list and value placeholders.
	// We search for ") VALUES" to avoid matching column names that contain "values".
	upper := strings.ToUpper(rendered)
	idx := strings.Index(upper, ") VALUES")
	if idx < 0 {
		return rendered
	}
	// Keep everything up to and including the closing ")" of columns, then append " VALUES "
	return rendered[:idx+1] + " VALUES "
}

// BuildValuePlaceholder returns "(?, ?, ...)" with n placeholders.
func BuildValuePlaceholder(n int) string {
	if n <= 0 {
		return "()"
	}
	var sb strings.Builder
	sb.WriteByte('(')
	for i := range n {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('?')
	}
	sb.WriteByte(')')
	return sb.String()
}
