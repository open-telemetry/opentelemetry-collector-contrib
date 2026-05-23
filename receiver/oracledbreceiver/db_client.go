// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	// register db driver
	_ "github.com/sijms/go-ora/v2"
	"go.uber.org/zap"
)

type dbClient interface {
	metricRows(ctx context.Context, args ...any) ([]metricRow, error)
}

type metricRow map[string]string

type dbSQLClient struct {
	db     *sql.DB
	logger *zap.Logger
	sql    string
}

func newDbClient(db *sql.DB, sql string, logger *zap.Logger) dbClient {
	return dbSQLClient{
		db:     db,
		sql:    sql,
		logger: logger,
	}
}

func (cl dbSQLClient) metricRows(ctx context.Context, args ...any) ([]metricRow, error) {
	sqlRows, err := cl.db.QueryContext(ctx, cl.sql, args...)
	if err != nil {
		return nil, err
	}
	var out []metricRow
	row := reusableRow{
		attrs: map[string]func() string{},
	}
	types, err := sqlRows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	for _, sqlType := range types {
		colName := sqlType.Name()
		var v any
		row.attrs[colName] = func() string {
			format := "%v"
			if v == nil {
				return ""
			}
			if reflect.TypeOf(v).Kind() == reflect.Slice {
				// The Postgres driver returns a []uint8 (a string) for decimal and numeric types,
				// which we want to render as strings. e.g. "4.1" instead of "[52, 46, 49]".
				// Other slice types get the same treatment.
				format = "%s"
			}
			return fmt.Sprintf(format, v)
		}
		row.scanDest = append(row.scanDest, &v)
	}
	for sqlRows.Next() {
		err = sqlRows.Scan(row.scanDest...)
		if err != nil {
			return nil, err
		}
		out = append(out, row.toMetricRow())
	}
	return out, nil
}

type reusableRow struct {
	attrs    map[string]func() string
	scanDest []any
}

func (row reusableRow) toMetricRow() metricRow {
	out := metricRow{}
	for k, f := range row.attrs {
		out[strings.ToUpper(k)] = f()
	}
	return out
}
