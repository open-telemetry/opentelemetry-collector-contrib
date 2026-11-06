// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var errIteration = errors.New("connection lost while reading rows")

// failingRowsDriver returns rows that yield one row and then fail with
// errIteration, the way a query does when the connection drops mid-read.
type failingRowsDriver struct {
	closedRows atomic.Int32
}

func (d *failingRowsDriver) Connect(context.Context) (driver.Conn, error) {
	return &failingRowsConn{driver: d}, nil
}

func (d *failingRowsDriver) Driver() driver.Driver {
	return d
}

func (d *failingRowsDriver) Open(string) (driver.Conn, error) {
	return &failingRowsConn{driver: d}, nil
}

type failingRowsConn struct {
	driver *failingRowsDriver
}

func (*failingRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (*failingRowsConn) Close() error { return nil }

func (*failingRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *failingRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &failingRows{driver: c.driver}, nil
}

type failingRows struct {
	driver *failingRowsDriver
	read   int
}

func (*failingRows) Columns() []string { return []string{"NAME"} }

func (r *failingRows) Close() error {
	r.driver.closedRows.Add(1)
	return nil
}

func (r *failingRows) Next(dest []driver.Value) error {
	r.read++
	switch r.read {
	case 1:
		dest[0] = "a"
		return nil
	case 2:
		return errIteration
	default:
		return io.EOF
	}
}

func TestMetricRowsReturnsIterationError(t *testing.T) {
	d := &failingRowsDriver{}
	db := sql.OpenDB(d)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	client := newDbClient(db, "SELECT NAME FROM T", zap.NewNop())
	rows, err := client.metricRows(t.Context())

	require.ErrorIs(t, err, errIteration)
	assert.Nil(t, rows)
	// database/sql also closes the rows itself when Next fails, so this is a
	// sanity check and does not cover the deferred Close in metricRows.
	assert.Equal(t, int32(1), d.closedRows.Load())
}
