// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDBSQLClient_SingleRow(t *testing.T) {
	cl := DbSQLClient{
		Db:     fakeDB{rowVals: [][]any{{42, 123.4, "hello", true, []uint8{52, 46, 49}}}},
		Logger: zap.NewNop(),
		SQL:    "",
	}
	rows, err := cl.QueryRows(context.Background())
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.EqualValues(t, map[string]string{
		"col_0": "42",
		"col_1": "123.4",
		"col_2": "hello",
		"col_3": "true",
		"col_4": "4.1",
	}, rows[0])
}

func TestDBSQLClient_MultiRow(t *testing.T) {
	cl := DbSQLClient{
		Db: fakeDB{rowVals: [][]any{
			{42, 123.4, "hello", true, []uint8{52, 46, 49}},
			{43, 123.5, "goodbye", false, []uint8{52, 46, 50}},
		}},
		Logger: zap.NewNop(),
		SQL:    "",
	}
	rows, err := cl.QueryRows(context.Background())
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.EqualValues(t, map[string]string{
		"col_0": "42",
		"col_1": "123.4",
		"col_2": "hello",
		"col_3": "true",
		"col_4": "4.1",
	}, rows[0])
	assert.EqualValues(t, map[string]string{
		"col_0": "43",
		"col_1": "123.5",
		"col_2": "goodbye",
		"col_3": "false",
		"col_4": "4.2",
	}, rows[1])
}

func TestDBSQLClient_Nulls(t *testing.T) {
	cl := DbSQLClient{
		Db: fakeDB{rowVals: [][]any{
			{42, nil, 111}, // NULLs from the DB map to nil here
		}},
		Logger: zap.NewNop(),
		SQL:    "",
	}
	rows, err := cl.QueryRows(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNullValueWarning)
	assert.Len(t, rows, 1)
	assert.EqualValues(t, map[string]string{
		"col_0": "42",
		"col_2": "111",
	}, rows[0])
}

func TestDBSQLClient_Nulls_MultiRow(t *testing.T) {
	cl := DbSQLClient{
		Db: fakeDB{rowVals: [][]any{
			{42, nil},
			{43, nil},
		}},
		Logger: zap.NewNop(),
		SQL:    "",
	}
	rows, err := cl.QueryRows(context.Background())
	assert.Error(t, err)

	var e interface{ Unwrap() []error }
	if errors.As(err, &e) {
		uw := e.Unwrap()
		assert.Len(t, uw, 2)

		for _, err := range uw {
			assert.ErrorIs(t, err, ErrNullValueWarning)
		}
	}
	assert.Len(t, rows, 2)
	assert.EqualValues(t, map[string]string{
		"col_0": "42",
	}, rows[0])
	assert.EqualValues(t, map[string]string{
		"col_0": "43",
	}, rows[1])
}

type fakeDB struct {
	rowVals [][]any
}

func (db fakeDB) QueryContext(context.Context, string, ...any) (rows, error) {
	return &fakeRows{vals: db.rowVals}, nil
}

type fakeRows struct {
	vals [][]any
	row  int
}

func (r *fakeRows) ColumnTypes() ([]colType, error) {
	var out []colType
	for i := 0; i < len(r.vals[0]); i++ {
		out = append(out, fakeCol{fmt.Sprintf("col_%d", i)})
	}
	return out, nil
}

func (r *fakeRows) Next() bool {
	return r.row < len(r.vals)
}

func (r *fakeRows) Scan(dest ...any) error {
	for i := range dest {
		ptr := dest[i].(*any)
		*ptr = r.vals[r.row][i]
	}
	r.row++
	return nil
}

type fakeCol struct {
	name string
}

func (c fakeCol) Name() string {
	return c.name
}
