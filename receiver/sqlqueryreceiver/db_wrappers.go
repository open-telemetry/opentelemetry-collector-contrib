// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"
	"database/sql"
)

// These are wrappers and interfaces around sql.DB so that it can be swapped out for testing.

type db interface {
	QueryContext(ctx context.Context, query string, args ...any) (rows, error)
}

type rows interface {
	ColumnTypes() ([]colType, error)
	Next() bool
	Scan(dest ...any) error
}

type colType interface {
	Name() string
}

type dbWrapper struct {
	db *sql.DB
}

func (d dbWrapper) QueryContext(ctx context.Context, query string, args ...any) (rows, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	return rowsWrapper{rows}, err
}

type rowsWrapper struct {
	rows *sql.Rows
}

func (r rowsWrapper) ColumnTypes() ([]colType, error) {
	types, err := r.rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	var out []colType
	for _, columnType := range types {
		out = append(out, colWrapper{columnType})
	}
	return out, nil
}

func (r rowsWrapper) Next() bool {
	return r.rows.Next()
}

func (r rowsWrapper) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

type colWrapper struct {
	ct *sql.ColumnType
}

func (c colWrapper) Name() string {
	return c.ct.Name()
}
