// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // Postgres driver
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	_ "modernc.org/sqlite" // SQLite driver
)

type dbStorageClient struct {
	logger  *zap.Logger
	db      *sql.DB
	dialect *dbDialect
}

func newClient(ctx context.Context, logger *zap.Logger, db *sql.DB, driverName, tableName string) (*dbStorageClient, error) {
	dialect := newDBDialect(driverName, tableName)

	// Create storage table if not exists
	_, err := db.ExecContext(ctx, dialect.Queries.QueryCreateTable)
	if err != nil {
		return nil, err
	}

	// Set Prepared Statements for some regularly used queries for performance optimization
	if err := dialect.Prepare(ctx, db); err != nil {
		return nil, err
	}

	return &dbStorageClient{
		logger:  logger,
		db:      db,
		dialect: dialect,
	}, nil
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

	// Batch optimization when we have set of operations with the same OpType
	// It might give us big performance improvement with lower resource utilization on big batches
	if squashOp, can := canAggregate(ops...); can {
		if batchErr := c.aggregatedBatch(ctx, tx, squashOp, ops...); batchErr != nil {
			return batchErr
		}
		return tx.Commit()
	}

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

// Close will close all prepared statements
// Caller is responsible of closing used DB connection after this
func (c *dbStorageClient) Close(_ context.Context) error {
	return c.dialect.Close()
}

func (c *dbStorageClient) get(ctx context.Context, key string, tx *sql.Tx) ([]byte, error) {
	rows, err := wrapTx(c.dialect.GetRowStmt, tx).QueryContext(ctx, key)
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
	_, err := wrapTx(c.dialect.SetRowStmt, tx).ExecContext(ctx, key, value)
	return err
}

func (c *dbStorageClient) delete(ctx context.Context, key string, tx *sql.Tx) error {
	_, err := wrapTx(c.dialect.DeleteRowStmt, tx).ExecContext(ctx, key)
	return err
}

func (c *dbStorageClient) aggregatedBatch(ctx context.Context, tx *sql.Tx, squashOp storage.OpType, ops ...*storage.Operation) error {
	var opFunc func(ctx context.Context, tx *sql.Tx, ops ...*storage.Operation) error

	switch squashOp {
	case storage.Get:
		opFunc = c.batchGet
	case storage.Set:
		opFunc = c.batchSet
	case storage.Delete:
		opFunc = c.batchDelete
	default:
		return errors.New("wrong operation type")
	}

	// Iterate over operations in chunks with length = c.dialect.MaxAggregationSize
	opsLen := len(ops)
	for i := 0; i < opsLen; i += c.dialect.MaxAggregationSize {
		end := min(i+c.dialect.MaxAggregationSize, opsLen)

		if err := opFunc(ctx, tx, ops[i:end]...); err != nil {
			return err
		}
	}

	return nil
}

func (c *dbStorageClient) batchGet(ctx context.Context, tx *sql.Tx, ops ...*storage.Operation) error {
	opsCount := len(ops)
	// Form a multi-row SELECT Query
	placeholders := generatePlaceholders(opsCount, 0)
	query := strings.Replace(c.dialect.Queries.QueryGetMultiRows, "$1", placeholders, 1)

	// Create helper structs for passing data to query and getting result back
	keys := make([]any, opsCount)
	keysIdx := make(map[string]int, opsCount)
	for idx, op := range ops {
		keys[idx] = op.Key
		keysIdx[op.Key] = idx
	}

	rows, err := tx.QueryContext(ctx, query, keys...)
	if err != nil {
		return err
	}
	// Make sure that all associated resource are closed on func exit
	defer rows.Close()

	// Iterate over returned rows
	for rows.Next() {
		var key string
		var value []byte

		if err := rows.Scan(&key, &value); err != nil {
			return err
		}

		// If we haven't received row for some key - just skip it and leave op.Value as is
		if idx, exists := keysIdx[key]; exists {
			ops[idx].Value = value
		}
	}

	// Check for any errors happened during rows scan
	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func (c *dbStorageClient) batchSet(ctx context.Context, tx *sql.Tx, ops ...*storage.Operation) error {
	opsCount := len(ops)
	// Form a multi-row INSERT Query
	placeholders := generatePlaceholders(opsCount, 2)

	vals := make([]any, opsCount*2)
	idx := 0
	for _, op := range ops {
		vals[idx] = op.Key
		idx++
		vals[idx] = op.Value
		idx++
	}

	query := strings.Replace(c.dialect.Queries.QuerySetMultiRows, "$1", placeholders, 1)
	_, err := tx.ExecContext(ctx, query, vals...)

	return err
}

func (c *dbStorageClient) batchDelete(ctx context.Context, tx *sql.Tx, ops ...*storage.Operation) error {
	opsCount := len(ops)
	// Form a multi-row DELETE Query
	placeholders := generatePlaceholders(opsCount, 0)
	query := strings.Replace(c.dialect.Queries.QueryDeleteMultiRows, "$1", placeholders, 1)

	vals := make([]any, opsCount)
	for idx, op := range ops {
		vals[idx] = op.Key
	}

	_, err := tx.ExecContext(ctx, query, vals...)

	return err
}

// wrapTx will wrap provided Statement in Transaction if it is provided
func wrapTx(stmt *sql.Stmt, tx *sql.Tx) *sql.Stmt {
	if tx != nil {
		return tx.Stmt(stmt)
	}

	return stmt
}

// canAggregate checks is possible to aggregate batch of Operations into single query
// As for now it's possible to aggregate only whole batch with a single Operation Type
// Returns aggregated Operation Type and aggregation flag
func canAggregate(ops ...*storage.Operation) (storage.OpType, bool) {
	// Empty case
	if len(ops) == 0 {
		return 0, false
	}

	// Nothing to squash
	if len(ops) == 1 {
		return 0, false
	}

	// Initial value
	resOp := ops[0].Type
	for _, op := range ops {
		// If next Operation is different - exit loop
		if resOp != op.Type {
			return 0, false
		}
		// Continue...
	}

	return resOp, true
}

// generatePlaceholders creates SQL placeholder for parametrized queries
// Positional placeholders, like "$N" is used as they are supported by all used SQL drivers
// By default, when `groupSize = 0` will generate N monotonic placeholders, i.e. "$1, $2, ... $n"
// If `groupSize > 0` - will generate placeholders groups with size = `groupSize`,
// i.e for `groupSize = 2` - "($1, $2), ($3, $4), ... ($n*groupSize-1, $n*groupSize)"
func generatePlaceholders(n, groupSize int) string {
	if n <= 0 {
		return ""
	}

	sb := &strings.Builder{}
	// Simple case when we don'e need to group placeholders
	if groupSize == 0 {
		for i := range n {
			sb.WriteByte('$')
			sb.WriteString(strconv.Itoa(i + 1))
			if i != n-1 {
				sb.WriteByte(',')
			}
		}

		return sb.String()
	}

	// Complex case when we need to group placeholders with defined group size
	n *= groupSize
	for i := 0; i < n; {
		sb.WriteByte('(')
		for range groupSize {
			i++
			sb.WriteByte('$')
			sb.WriteString(strconv.Itoa(i))

			if i%groupSize != 0 {
				sb.WriteByte(',')
			}
		}
		sb.WriteByte(')')
		if i != n {
			sb.WriteByte(',')
		}
	}

	return sb.String()
}
