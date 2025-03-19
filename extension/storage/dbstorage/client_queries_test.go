// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getDialectQueries(t *testing.T) {
	t.Run("Should return default set if unknown driver specified", func(t *testing.T) {
		got := getDialectQueries("unknown")
		assert.Equal(t, sqlGenericCreateTableQuery, got.QueryCreateTable)
		assert.Equal(t, sqlGenericGetQuery, got.QueryGetRow)
		assert.Equal(t, sqlGenericInsertQuery, got.QuerySetRow)
		assert.Equal(t, sqlGenericDeleteQuery, got.QueryDeleteRow)
		assert.Equal(t, sqlGenericMultiGetQuery, got.QueryGetMultiRows)
		assert.Equal(t, sqlGenericMultiInsertQuery, got.QuerySetMultiRows)
		assert.Equal(t, sqlGenericMultiDeleteQuery, got.QueryDeleteMultiRows)
	})
	t.Run("Should return driver-specific queries", func(t *testing.T) {
		got := getDialectQueries(driverSQLite)
		assert.NotEqual(t, sqlGenericCreateTableQuery, got.QueryCreateTable)
	})
}

func Test_dbDialect_Prepare(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	t.Run("Should prepare queries", func(t *testing.T) {
		dialect := newDBDialect(driverPostgreSQL, testTableName)
		mock.ExpectPrepare(regexp.QuoteMeta(dialect.Queries.QueryGetRow))
		mock.ExpectPrepare(regexp.QuoteMeta(dialect.Queries.QuerySetRow))
		mock.ExpectPrepare(regexp.QuoteMeta(dialect.Queries.QueryDeleteRow))
		require.NoError(t, dialect.Prepare(context.Background(), db))
	})
}

func Test_newDBDialect(t *testing.T) {
	t.Run("Should substitute table name on Dialect creation", func(t *testing.T) {
		got := newDBDialect(driverPostgreSQL, testTableName)
		assert.Contains(t, got.Queries.QueryCreateTable, testTableName)
		assert.Contains(t, got.Queries.QueryGetRow, testTableName)
		assert.Contains(t, got.Queries.QuerySetRow, testTableName)
		assert.Contains(t, got.Queries.QueryDeleteRow, testTableName)
		assert.Contains(t, got.Queries.QueryGetMultiRows, testTableName)
		assert.Contains(t, got.Queries.QuerySetMultiRows, testTableName)
		assert.Contains(t, got.Queries.QueryDeleteMultiRows, testTableName)
	})
}
