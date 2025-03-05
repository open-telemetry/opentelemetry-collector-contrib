// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

// goimports and gci has unresolvable conflict here, so we have to disable one of them
//nolint:gci
import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// Postgres driver
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"

	// SQLite driver
	_ "modernc.org/sqlite"
)

const (
	createTableSqlite = "create table if not exists %s (key text primary key, value blob)"
	createTable       = "create table if not exists %s (key text primary key, value text)"
	getQueryText      = "select value from %s where key=$1"
	setQueryText      = "insert into %s(key, value) values($1,$2) on conflict(key) do update set value=$3"
	deleteQueryText   = "delete from %s where key=$1"
)

type dbStorageClient struct {
	logger      *zap.Logger
	db          *sql.DB
	getQuery    *sql.Stmt
	setQuery    *sql.Stmt
	deleteQuery *sql.Stmt
}

func newClient(ctx context.Context, logger *zap.Logger, db *sql.DB, driverName string, tableName string) (*dbStorageClient, error) {
	createTableSQL := createTable
	if driverName == driverSQLite {
		createTableSQL = createTableSqlite
	}
	var err error
	_, err = db.ExecContext(ctx, fmt.Sprintf(createTableSQL, tableName))
	if err != nil {
		return nil, err
	}

	selectQuery, err := db.PrepareContext(ctx, fmt.Sprintf(getQueryText, tableName))
	if err != nil {
		return nil, err
	}
	setQuery, err := db.PrepareContext(ctx, fmt.Sprintf(setQueryText, tableName))
	if err != nil {
		return nil, err
	}
	deleteQuery, err := db.PrepareContext(ctx, fmt.Sprintf(deleteQueryText, tableName))
	if err != nil {
		return nil, err
	}
	return &dbStorageClient{logger, db, selectQuery, setQuery, deleteQuery}, nil
}

// Get will retrieve data from storage that corresponds to the specified key
func (c *dbStorageClient) Get(ctx context.Context, key string) ([]byte, error) {
	return c.get(ctx, key, nil)
}

// Set will store data. The data can be retrieved using the same key
func (c *dbStorageClient) Set(ctx context.Context, key string, value []byte) error {
	return c.set(ctx, key, value, nil)
}

// Delete will delete data associated with the specified key
func (c *dbStorageClient) Delete(ctx context.Context, key string) error {
	return c.delete(ctx, key, nil)
}

// Batch executes the specified operations in order. Get operation results are updated in place
func (c *dbStorageClient) Batch(ctx context.Context, ops ...*storage.Operation) error {
	// Start a new transaction
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// In case of any error we should roll back whole transaction to keep DB in consistent state
	// In case of successful commit - tx.Rollback() will be a no-op here as tx is already closed
	defer func() {
		// We should ignore error related already finished transaction here
		// It might happened, for example, if Context was canceled outside of Batch() function
		// in this case whole transaction will be rolled back by sql package and we'll receive ErrTxDone here,
		// which is actually not an issue because transaction was correctly closed with rollback
		if rollbackErr := tx.Rollback(); !errors.Is(rollbackErr, sql.ErrTxDone) {
			c.logger.Error("Failed to rollback Batch() transaction", zap.Error(rollbackErr))
		}
	}()

	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value, err = c.get(ctx, op.Key, tx)
		case storage.Set:
			err = c.set(ctx, op.Key, op.Value, tx)
		case storage.Delete:
			err = c.delete(ctx, op.Key, tx)
		default:
			return errors.New("wrong operation type")
		}

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Close will close the database
func (c *dbStorageClient) Close(_ context.Context) error {
	if err := c.setQuery.Close(); err != nil {
		return err
	}
	if err := c.deleteQuery.Close(); err != nil {
		return err
	}
	return c.getQuery.Close()
}

func (c *dbStorageClient) get(ctx context.Context, key string, tx *sql.Tx) ([]byte, error) {
	rows, err := c.wrapTx(c.getQuery, tx).QueryContext(ctx, key)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, nil
	}

	var result []byte
	if err := rows.Scan(&result); err != nil {
		return result, err
	}

	return result, rows.Close()
}

func (c *dbStorageClient) set(ctx context.Context, key string, value []byte, tx *sql.Tx) error {
	_, err := c.wrapTx(c.setQuery, tx).ExecContext(ctx, key, value, value)
	return err
}

func (c *dbStorageClient) delete(ctx context.Context, key string, tx *sql.Tx) error {
	_, err := c.wrapTx(c.deleteQuery, tx).ExecContext(ctx, key)
	return err
}

func (c *dbStorageClient) wrapTx(stmt *sql.Stmt, tx *sql.Tx) *sql.Stmt {
	if tx != nil {
		return tx.Stmt(stmt)
	}

	return stmt
}
