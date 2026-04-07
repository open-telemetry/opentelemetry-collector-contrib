// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal"

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

const (
	DefaultDatabase        = "otel"
	DefaultMaxOpenConns    = 10
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = 5 * time.Minute
)

// NewMySQLClient creates a new MySQL database connection.
func NewMySQLClient(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	db.SetMaxOpenConns(DefaultMaxOpenConns)
	db.SetMaxIdleConns(DefaultMaxIdleConns)
	db.SetConnMaxLifetime(DefaultConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}

	return db, nil
}

// NewMySQLClientNoDB creates a MySQL connection without specifying a database.
// This is used to create the database before connecting to it.
func NewMySQLClientNoDB(dsn string) (*sql.DB, error) {
	noDB := stripDBFromDSN(dsn)
	return NewMySQLClient(noDB)
}

// stripDBFromDSN removes the database name from a DSN.
// e.g. "user:pass@tcp(host:port)/dbname?params" -> "user:pass@tcp(host:port)/?params"
func stripDBFromDSN(dsn string) string {
	// Split at '?' to handle query params
	parts := strings.SplitN(dsn, "?", 2)
	base := parts[0]

	// Find the last '/' which separates host from dbname
	slashIdx := strings.LastIndex(base, "/")
	if slashIdx < 0 {
		return dsn
	}

	stripped := base[:slashIdx+1]
	if len(parts) > 1 {
		stripped += "?" + parts[1]
	}
	return stripped
}

// CreateDatabase creates the database if it does not exist.
func CreateDatabase(ctx context.Context, db *sql.DB, database string) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4", database)
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return nil
}

// CreateTable executes a CREATE TABLE statement.
func CreateTable(ctx context.Context, db *sql.DB, ddl string) error {
	_, err := db.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}
	return nil
}
