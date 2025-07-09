// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	// Generic set of queries
	// Will NOT work on all most popular SQL DB's, see DB-specific override queries below
	// Tested to be working at least on PostgreSQL and SQLite
	// Not all queries are working on MSSQL, MySQL/MqriaDB, Oracle, etc.
	sqlGenericCreateTableQuery = "CREATE TABLE IF NOT EXISTS %s (key TEXT PRIMARY KEY, value TEXT)"
	sqlGenericGetQuery         = "SELECT value FROM %s WHERE key=$1"
	sqlGenericMultiGetQuery    = "SELECT key, value FROM %s WHERE key IN ($1)"
	sqlGenericInsertQuery      = "INSERT INTO %s(key, value) VALUES($1, $2) ON CONFLICT(key) DO UPDATE SET value=excluded.value"
	sqlGenericMultiInsertQuery = "INSERT INTO %s(key, value) VALUES $1 ON CONFLICT(key) DO UPDATE SET value=excluded.value"
	sqlGenericDeleteQuery      = "DELETE FROM %s WHERE key=$1"
	sqlGenericMultiDeleteQuery = "DELETE FROM %s WHERE key IN ($1)"
	// DB-specific queries
	// SQLite
	sqlSQLiteCreateTableQuery = "CREATE TABLE IF NOT EXISTS %s (key TEXT PRIMARY KEY, value BLOB)"
	// PostgreSQL
	sqlPostgreSQLCreateTableQuery = "CREATE TABLE IF NOT EXISTS %s (key TEXT PRIMARY KEY, value bytea)"
	// Other to be added later...

	// Max amount of similar queries that can be aggregated into single query, driver-specific
	// This numbers are taken from benchmarks when aggregation profit is leveled out on specific amount of aggregated queries
	maxAggregatedOpsSQLite     = 200
	maxAggregatedOpsPostgreSQL = 500
	// Lowest value for safety
	maxAggregatedOpsGeneric = maxAggregatedOpsSQLite
)

type sqlQuerySet struct {
	QueryCreateTable     string
	QueryGetRow          string
	QuerySetRow          string
	QueryDeleteRow       string
	QueryGetMultiRows    string
	QuerySetMultiRows    string
	QueryDeleteMultiRows string
}

// getDialectQueries returns set of queries used in this extension, driver-specific
func getDialectQueries(driverName string) sqlQuerySet {
	set := sqlQuerySet{
		QueryCreateTable:     sqlGenericCreateTableQuery,
		QueryGetRow:          sqlGenericGetQuery,
		QuerySetRow:          sqlGenericInsertQuery,
		QueryDeleteRow:       sqlGenericDeleteQuery,
		QueryGetMultiRows:    sqlGenericMultiGetQuery,
		QuerySetMultiRows:    sqlGenericMultiInsertQuery,
		QueryDeleteMultiRows: sqlGenericMultiDeleteQuery,
	}

	switch driverName {
	case driverSQLite:
		set.QueryCreateTable = sqlSQLiteCreateTableQuery
	case driverPostgreSQL:
		set.QueryCreateTable = sqlPostgreSQLCreateTableQuery
	}

	return set
}

type dbDialect struct {
	// Original set of DB-specific queries with table substitution in place
	// Some of them are used as is, some - will be prepared for multiple re-use
	Queries sqlQuerySet
	// Re-used queries as Prepared Statements
	GetRowStmt    *sql.Stmt
	SetRowStmt    *sql.Stmt
	DeleteRowStmt *sql.Stmt
	// Reasonable size of Operations that could be aggregated into single batch query
	// This limit is based on benchmark tests for each specific SQL driver
	MaxAggregationSize int
}

// Prepare will compile some regularly used queries into Prepared Statements
func (d *dbDialect) Prepare(ctx context.Context, db *sql.DB) error {
	var err error

	d.GetRowStmt, err = db.PrepareContext(ctx, d.Queries.QueryGetRow)
	if err != nil {
		return err
	}
	d.SetRowStmt, err = db.PrepareContext(ctx, d.Queries.QuerySetRow)
	if err != nil {
		return err
	}
	d.DeleteRowStmt, err = db.PrepareContext(ctx, d.Queries.QueryDeleteRow)

	return err
}

// Close will close all prepared statements available for this dialect
func (d *dbDialect) Close() error {
	stmts := []*sql.Stmt{
		d.GetRowStmt,
		d.SetRowStmt,
		d.DeleteRowStmt,
	}
	for _, stmt := range stmts {
		if err := stmt.Close(); err != nil {
			return err
		}
	}

	return nil
}

// newDBDialect creates a new DB dialect which is just a set of DB-specific queries/prepared statements
func newDBDialect(driverName string, tableName string) *dbDialect {
	queries := getDialectQueries(driverName)

	var aggSize int
	switch driverName {
	case driverSQLite:
		aggSize = maxAggregatedOpsSQLite
	case driverPostgreSQL:
		aggSize = maxAggregatedOpsPostgreSQL
	default:
		aggSize = maxAggregatedOpsGeneric
	}
	return &dbDialect{
		Queries: sqlQuerySet{
			QueryCreateTable:     fmt.Sprintf(queries.QueryCreateTable, tableName),
			QueryGetRow:          fmt.Sprintf(queries.QueryGetRow, tableName),
			QuerySetRow:          fmt.Sprintf(queries.QuerySetRow, tableName),
			QueryDeleteRow:       fmt.Sprintf(queries.QueryDeleteRow, tableName),
			QueryGetMultiRows:    fmt.Sprintf(queries.QueryGetMultiRows, tableName),
			QuerySetMultiRows:    fmt.Sprintf(queries.QuerySetMultiRows, tableName),
			QueryDeleteMultiRows: fmt.Sprintf(queries.QueryDeleteMultiRows, tableName),
		},
		MaxAggregationSize: aggSize,
	}
}
