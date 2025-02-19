// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

var testTableName = "exporter_otlp_test"

func Test_newClient(t *testing.T) {
	t.Run("Should return client with generic query(s)", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(createTable, testTableName))).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName)))

		_, err = newClient(context.Background(), zap.L(), db, driverPostgreSQL, testTableName)
		assert.NoError(t, err)
	})
	t.Run("Should return client with Sqlite specific query(s)", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(createTableSqlite, testTableName))).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName)))

		_, err = newClient(context.Background(), zap.L(), db, driverSQLite, testTableName)
		assert.NoError(t, err)
	})
}

func Test_dbStorageClient_Get(t *testing.T) {
	t.Run("Should get a row without transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow([]byte("value")))

		got, err := client.Get(context.Background(), "test")
		assert.NoError(t, err)
		assert.Equal(t, []byte("value"), got)
	})
	t.Run("Should get a row within transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow([]byte("value")))
		mock.ExpectCommit()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		got, err := client.get(context.Background(), "test", tx)
		assert.NoError(t, err)
		assert.Equal(t, []byte("value"), got)
		assert.NoError(t, tx.Commit())
	})
	t.Run("Should return only first row from set", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow([]byte("first")).AddRow([]byte("second")))

		got, err := client.Get(context.Background(), "test")
		assert.NoError(t, err)
		assert.Equal(t, []byte("first"), got)
	})
	t.Run("Shouldn't return error if no records selected", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}))

		got, err := client.Get(context.Background(), "test")
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
}

func Test_dbStorageClient_Set(t *testing.T) {
	t.Run("Should delete a row without transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName))).
			WithArgs("test", []byte("value"), []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))

		assert.NoError(t, client.Set(context.Background(), "test", []byte("value")))
	})
	t.Run("Should delete a row within transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName))).
			WithArgs("test", []byte("value"), []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		assert.NoError(t, client.set(context.Background(), "test", []byte("value"), tx))
		assert.NoError(t, tx.Commit())
	})
	t.Run("Shouldn't return error if no record exists", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName))).
			WithArgs("test", []byte("value"), []byte("value")).
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, client.Set(context.Background(), "test", []byte("value")))
	})
}

func Test_dbStorageClient_Delete(t *testing.T) {
	t.Run("Should delete a row without transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))

		assert.NoError(t, client.Delete(context.Background(), "test"))
	})
	t.Run("Should delete a row within transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		assert.NoError(t, client.delete(context.Background(), "test", tx))
		assert.NoError(t, tx.Commit())
	})
	t.Run("Shouldn't return error if no record exists", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, client.Delete(context.Background(), "test"))
	})
}

func Test_dbStorageClient_Batch(t *testing.T) {
	t.Run("Should run set of operations in a single transaction per call", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName))).
			WithArgs("test", []byte("value"), []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		ops := []*storage.Operation{
			storage.SetOperation("test", []byte("value")),
			storage.DeleteOperation("test"),
		}
		assert.NoError(t, client.Batch(context.Background(), ops...))
		assert.NoError(t, client.Batch(context.Background(), ops[1]))
	})
	t.Run("Should return error if any operation failed, with transaction rollback", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName))).
			WithArgs("test", []byte("value"), []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName))).
			WithArgs("test").
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		ops := []*storage.Operation{
			storage.SetOperation("test", []byte("value")),
			storage.DeleteOperation("test"),
			storage.GetOperation("test"),
		}
		assert.ErrorIs(t, client.Batch(context.Background(), ops...), sql.ErrConnDone)
	})
}

func Test_dbStorageClient_wrapTx(t *testing.T) {
	t.Run("should wrap prepared statement in transaction", func(t *testing.T) {
		client, mock := newTestClient(t)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		//nolint:errcheck
		defer tx.Rollback()

		stmt := client.getQuery
		assert.NotEqual(t, stmt, client.wrapTx(stmt, tx))
	})
	t.Run("shouldn't wrap prepared statement without transaction", func(t *testing.T) {
		client, _ := newTestClient(t)
		defer client.db.Close()

		stmt := client.getQuery
		assert.Equal(t, stmt, client.wrapTx(stmt, nil))
	})
}

func newTestClient(t *testing.T) (*dbStorageClient, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(getQueryText, testTableName)))
	mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(setQueryText, testTableName)))
	mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(deleteQueryText, testTableName)))

	selectQuery, err := db.PrepareContext(context.Background(), fmt.Sprintf(getQueryText, testTableName))
	require.NoError(t, err)
	setQuery, err := db.PrepareContext(context.Background(), fmt.Sprintf(setQueryText, testTableName))
	require.NoError(t, err)
	deleteQuery, err := db.PrepareContext(context.Background(), fmt.Sprintf(deleteQueryText, testTableName))
	require.NoError(t, err)

	return &dbStorageClient{
		logger:      zap.L(),
		db:          db,
		getQuery:    selectQuery,
		setQuery:    setQuery,
		deleteQuery: deleteQuery,
	}, mock
}
