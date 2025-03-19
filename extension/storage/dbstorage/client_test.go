// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

var testTableName = "exporter_otlp_test"

func Test_newClient(t *testing.T) {
	t.Run("Should return client with PostgreSQL query(s)", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		queries := getDialectQueries(driverPostgreSQL)

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryCreateTable, testTableName))).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName)))

		_, err = newClient(context.Background(), zap.L(), db, driverPostgreSQL, testTableName)
		assert.NoError(t, err)
	})
	t.Run("Should return client with Sqlite query(s)", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		queries := getDialectQueries(driverSQLite)

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryCreateTable, testTableName))).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName)))
		mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName)))

		_, err = newClient(context.Background(), zap.L(), db, driverSQLite, testTableName)
		assert.NoError(t, err)
	})
}

func Test_dbStorageClient_Get(t *testing.T) {
	queries := getDialectQueries(driverSQLite)

	t.Run("Should get a row without transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow([]byte("value")))

		got, err := client.Get(context.Background(), "test")
		assert.NoError(t, err)
		assert.Equal(t, []byte("value"), got)
	})
	t.Run("Should get a row within transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverPostgreSQL)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName))).
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
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow([]byte("first")).AddRow([]byte("second")))

		got, err := client.Get(context.Background(), "test")
		assert.NoError(t, err)
		assert.Equal(t, []byte("first"), got)
	})
	t.Run("Shouldn't return error if no records selected", func(t *testing.T) {
		client, mock := newTestClient(t, driverPostgreSQL)
		defer client.db.Close()

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName))).
			WithArgs("test").
			WillReturnRows(sqlmock.NewRows([]string{"value"}))

		got, err := client.Get(context.Background(), "test")
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
}

func Test_dbStorageClient_Set(t *testing.T) {
	queries := getDialectQueries(driverSQLite)

	t.Run("Should delete a row without transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName))).
			WithArgs("test", []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))

		assert.NoError(t, client.Set(context.Background(), "test", []byte("value")))
	})
	t.Run("Should delete a row within transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName))).
			WithArgs("test", []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		assert.NoError(t, client.set(context.Background(), "test", []byte("value"), tx))
		assert.NoError(t, tx.Commit())
	})
	t.Run("Shouldn't return error if no record exists", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName))).
			WithArgs("test", []byte("value")).
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, client.Set(context.Background(), "test", []byte("value")))
	})
}

func Test_dbStorageClient_Delete(t *testing.T) {
	queries := getDialectQueries(driverSQLite)

	t.Run("Should delete a row without transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))

		assert.NoError(t, client.Delete(context.Background(), "test"))
	})
	t.Run("Should delete a row within transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		assert.NoError(t, client.delete(context.Background(), "test", tx))
		assert.NoError(t, tx.Commit())
	})
	t.Run("Shouldn't return error if no record exists", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(0, 0))

		assert.NoError(t, client.Delete(context.Background(), "test"))
	})
}

func Test_dbStorageClient_Batch(t *testing.T) {
	queries := getDialectQueries(driverSQLite)

	t.Run("Should run set of operations in a single transaction per call", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName))).
			WithArgs("test", []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName))).
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
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName))).
			WithArgs("test", []byte("value")).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName))).
			WithArgs("test").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName))).
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
	t.Run("Should squash multiple identical Get Operations", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		ops := []*storage.Operation{
			storage.GetOperation("test1"),
			storage.GetOperation("test2"),
			storage.GetOperation("test3"),
		}
		q := fmt.Sprintf(queries.QueryGetMultiRows, testTableName)
		q1 := strings.Replace(q, "$1", generatePlaceholders(2, 0), 1)
		q2 := strings.Replace(q, "$1", generatePlaceholders(1, 0), 1)

		client.dialect.MaxAggregationSize = 2
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(q1)).
			WithArgs("test1", "test2").
			WillReturnRows(
				sqlmock.NewRows([]string{"key", "value"}).
					AddRow("test1", []byte("first")).
					AddRow("test2", []byte("second")),
			)
		mock.ExpectQuery(regexp.QuoteMeta(q2)).
			WithArgs("test3").
			WillReturnRows(
				sqlmock.NewRows([]string{"key", "value"}).
					AddRow("test3", []byte("third")),
			)
		mock.ExpectCommit()

		assert.NoError(t, client.Batch(context.Background(), ops...))
		assert.Equal(t, []byte("first"), ops[0].Value)
		assert.Equal(t, []byte("second"), ops[1].Value)
		assert.Equal(t, []byte("third"), ops[2].Value)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("Should squash multiple identical Set Operations", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		ops := []*storage.Operation{
			storage.SetOperation("test1", []byte("first")),
			storage.SetOperation("test2", []byte("second")),
			storage.SetOperation("test3", []byte("third")),
		}
		q := fmt.Sprintf(queries.QuerySetMultiRows, testTableName)
		q = strings.Replace(q, "$1", generatePlaceholders(len(ops), 2), 1)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(q)).
			WithArgs("test1", []byte("first"), "test2", []byte("second"), "test3", []byte("third")).
			WillReturnResult(sqlmock.NewResult(3, 3))
		mock.ExpectCommit()

		assert.NoError(t, client.Batch(context.Background(), ops...))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("Should squash multiple identical Delete Operations", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		ops := []*storage.Operation{
			storage.DeleteOperation("test1"),
			storage.DeleteOperation("test2"),
			storage.DeleteOperation("test3"),
		}
		q := fmt.Sprintf(queries.QueryDeleteMultiRows, testTableName)
		q = strings.Replace(q, "$1", generatePlaceholders(len(ops), 0), 1)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(q)).
			WithArgs("test1", "test2", "test3").
			WillReturnResult(sqlmock.NewResult(3, 3))
		mock.ExpectCommit()

		assert.NoError(t, client.Batch(context.Background(), ops...))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func Test_dbStorageClient_Close(t *testing.T) {
	t.Run("Shouldn't fail on already closed connection", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)

		mock.ExpectClose()

		assert.NoError(t, client.db.Close())
		assert.NoError(t, client.Close(context.Background()))
	})
}

func Test_wrapTx(t *testing.T) {
	t.Run("should wrap prepared statement in transaction", func(t *testing.T) {
		client, mock := newTestClient(t, driverSQLite)
		defer client.db.Close()

		mock.ExpectBegin()
		mock.ExpectRollback()

		tx, err := client.db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		//nolint:errcheck
		defer tx.Rollback()

		stmt := client.dialect.GetRowStmt
		assert.NotEqual(t, stmt, wrapTx(stmt, tx))
	})
	t.Run("shouldn't wrap prepared statement without transaction", func(t *testing.T) {
		client, _ := newTestClient(t, driverSQLite)
		defer client.db.Close()

		stmt := client.dialect.GetRowStmt
		assert.Equal(t, stmt, wrapTx(stmt, nil))
	})
}

func Test_generatePlaceholders(t *testing.T) {
	type args struct {
		n       int
		groupBy int
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty string on zero placeholders",
			args: args{
				n:       0,
				groupBy: 0,
			},
			want: "",
		},
		{
			name: "Single placeholder",
			args: args{
				n:       1,
				groupBy: 0,
			},
			want: "$1",
		},
		{
			name: "Simple set of placeholders",
			args: args{
				n:       5,
				groupBy: 0,
			},
			want: "$1,$2,$3,$4,$5",
		},
		{
			name: "Single group of placeholders",
			args: args{
				n:       1,
				groupBy: 1,
			},
			want: "($1)",
		},
		{
			name: "Grouped set of placeholders",
			args: args{
				n:       5,
				groupBy: 2,
			},
			want: "($1,$2),($3,$4),($5,$6),($7,$8),($9,$10)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, generatePlaceholders(tt.args.n, tt.args.groupBy))
		})
	}
}

func newTestClient(t *testing.T, driverName string) (*dbStorageClient, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	queries := getDialectQueries(driverName)

	mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QueryGetRow, testTableName)))
	mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QuerySetRow, testTableName)))
	mock.ExpectPrepare(regexp.QuoteMeta(fmt.Sprintf(queries.QueryDeleteRow, testTableName)))

	dialect := newDBDialect(driverName, testTableName)
	err = dialect.Prepare(context.Background(), db)
	require.NoError(t, err)

	return &dbStorageClient{
		logger:  zap.L(),
		db:      db,
		dialect: dialect,
	}, mock
}
