// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const DefaultDatabase = "default"

// DatabaseFromDSN returns the database specified in the DSN. Empty if unset.
func DatabaseFromDSN(dsn string) (string, error) {
	opt, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("failed to parse DSN: %w", err)
	}

	return opt.Auth.Database, nil
}

// NewClickhouseClientFromOptions creates a new ClickHouse client from a clickhouse.Options struct.
func NewClickhouseClientFromOptions(opt *clickhouse.Options) (driver.Conn, error) {
	// Always connect to default database since configured database may not exist yet.
	// TODO: only do this if createSchema is true
	opt.Auth.Database = DefaultDatabase

	conn, err := clickhouse.Open(opt)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// GenerateTTLExpr generates a TTL expression for a ClickHouse table.
func GenerateTTLExpr(ttl time.Duration, timeField string) string {
	if ttl > 0 {
		switch {
		case ttl%(24*time.Hour) == 0:
			return fmt.Sprintf(`TTL %s + toIntervalDay(%d)`, timeField, ttl/(24*time.Hour))
		case ttl%(time.Hour) == 0:
			return fmt.Sprintf(`TTL %s + toIntervalHour(%d)`, timeField, ttl/time.Hour)
		case ttl%(time.Minute) == 0:
			return fmt.Sprintf(`TTL %s + toIntervalMinute(%d)`, timeField, ttl/time.Minute)
		default:
			return fmt.Sprintf(`TTL %s + toIntervalSecond(%d)`, timeField, ttl/time.Second)
		}
	}

	return ""
}

// CreateDatabase runs the DDL for creating a database, with optional cluster string
func CreateDatabase(ctx context.Context, db driver.Conn, database, clusterStr string) error {
	if database == DefaultDatabase {
		return nil
	}

	ddl := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %q %s", database, clusterStr)

	err := db.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	return nil
}

// GetTableColumns returns the column names on a table for schema detection
func GetTableColumns(ctx context.Context, db driver.Conn, database, table string) ([]string, error) {
	descTable := fmt.Sprintf("DESC TABLE %q.%q", database, table)
	rows, err := db.Query(ctx, descTable)
	if err != nil {
		return nil, fmt.Errorf("get table columns: %w", err)
	}

	var columnNames []string
	for rows.Next() {
		var columnName, skip string
		scanErr := rows.Scan(&columnName, &skip, &skip, &skip, &skip, &skip, &skip)
		if scanErr != nil {
			return nil, fmt.Errorf("scan table column: %w", scanErr)
		}

		columnNames = append(columnNames, columnName)
	}

	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("get table columns rows close: %w", err)
	}

	return columnNames, nil
}
