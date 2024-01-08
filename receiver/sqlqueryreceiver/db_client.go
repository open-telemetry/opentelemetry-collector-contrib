// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"

	// register db drivers
	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/microsoft/go-mssqldb/integratedauth/krb5"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/snowflakedb/gosnowflake"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type stringMap map[string]string

type dbClient interface {
	queryRows(ctx context.Context, args ...any) ([]stringMap, error)
}

type dbSQLClient struct {
	db     db
	logger *zap.Logger
	sql    string
}

func newDbClient(db db, sql string, logger *zap.Logger) dbClient {
	return dbSQLClient{
		db:     db,
		sql:    sql,
		logger: logger,
	}
}

func (cl dbSQLClient) queryRows(ctx context.Context, args ...any) ([]stringMap, error) {
	sqlRows, err := cl.db.QueryContext(ctx, cl.sql, args...)
	if err != nil {
		return nil, err
	}
	var out []stringMap
	colTypes, err := sqlRows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	scanner := newRowScanner(colTypes)
	var warnings error
	for sqlRows.Next() {
		err = scanner.scan(sqlRows)
		if err != nil {
			return nil, err
		}
		sm, scanErr := scanner.toStringMap()
		if scanErr != nil {
			warnings = multierr.Append(warnings, scanErr)
		}
		out = append(out, sm)
	}
	return out, warnings
}
