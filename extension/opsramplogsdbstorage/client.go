//go:build !freebsd

package opsramplogsdbstorage

import (
	"context"
	"errors"
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

type dbStorageClient struct {
	prefix string
	db     *leveldb.DB
}

func newClient(db *leveldb.DB, prefix string) (*dbStorageClient, error) {
	return &dbStorageClient{
		prefix: prefix,
		db:     db,
	}, nil
}

// Get will retrieve data from storage that corresponds to the specified key
func (c *dbStorageClient) Get(_ context.Context, key string) ([]byte, error) {
	dbKey := fmt.Sprintf("%s_%s", c.prefix, key)

	b, err := c.db.Get([]byte(dbKey), nil)
	if err == nil {
		return b, nil
	}
	return nil, nil
}

// Set will store data. The data can be retrieved using the same key
func (c *dbStorageClient) Set(_ context.Context, key string, value []byte) error {
	dbKey := fmt.Sprintf("%s_%s", c.prefix, key)

	return c.db.Put([]byte(dbKey), value, nil)
}

// Delete will delete data associated with the specified key
func (c *dbStorageClient) Delete(_ context.Context, key string) error {
	dbKey := fmt.Sprintf("%s_%s", c.prefix, key)

	return c.db.Delete([]byte(dbKey), nil)
}

// Batch executes the specified operations in order. Get operation results are updated in place
func (c *dbStorageClient) Batch(ctx context.Context, ops ...storage.Operation) error {
	var err error
	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			op.Value, err = c.Get(ctx, op.Key)
		case storage.Set:
			err = c.Set(ctx, op.Key, op.Value)
		case storage.Delete:
			err = c.Delete(ctx, op.Key)
		default:
			return errors.New("wrong operation type")
		}

		if err != nil {
			return err
		}
	}
	return err
}

// Close will close the database
func (c *dbStorageClient) Close(_ context.Context) error {
	c.db.Close()
	return nil
}
