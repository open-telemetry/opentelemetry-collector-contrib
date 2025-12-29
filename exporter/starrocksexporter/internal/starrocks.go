// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const DefaultDatabase = "default"

// CreateDatabase runs the DDL for creating a database
func CreateDatabase(ctx context.Context, db *sql.DB, database string) error {
	if database == DefaultDatabase {
		return nil
	}

	ddl := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", database)
	_, err := db.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	return nil
}

// GetTableColumns returns the column names on a table for schema detection
func GetTableColumns(ctx context.Context, db *sql.DB, database, table string) ([]string, error) {
	query := fmt.Sprintf("DESC `%s`.`%s`", database, table)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get table columns: %w", err)
	}
	defer rows.Close()

	var columnNames []string
	for rows.Next() {
		var columnName, columnType, null, key, defaultValue, extra sql.NullString
		if err := rows.Scan(&columnName, &columnType, &null, &key, &defaultValue, &extra); err != nil {
			return nil, fmt.Errorf("scan table column: %w", err)
		}
		if columnName.Valid {
			columnNames = append(columnNames, columnName.String)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get table columns rows error: %w", err)
	}

	return columnNames, nil
}

// GenerateTTLExpr generates a TTL expression for a StarRocks table.
// StarRocks uses dynamic partition deletion for TTL, which is different from ClickHouse.
// For now, we'll return empty string as StarRocks handles TTL differently.
func GenerateTTLExpr(ttl time.Duration, timeField string) string {
	// StarRocks doesn't support TTL in the same way as ClickHouse
	// TTL is typically handled through dynamic partition deletion
	// This can be configured separately in StarRocks
	return ""
}



