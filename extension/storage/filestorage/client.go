// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"time"

	"go.etcd.io/bbolt"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

var defaultBucket = []byte(`default`)

type fileStorageClient struct {
	db *bbolt.DB
}

func bboltOptions(timeout time.Duration) *bbolt.Options {
	return &bbolt.Options{
		Timeout:        timeout,
		NoSync:         true,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	}
}

func newClient(filePath string, timeout time.Duration) (*fileStorageClient, error) {
	options := bboltOptions(timeout)
	db, err := bbolt.Open(filePath, 0600, options)
	if err != nil {
		return nil, err
	}

	initBucket := func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(defaultBucket)
		return err
	}
	if err := db.Update(initBucket); err != nil {
		return nil, err
	}

	return &fileStorageClient{db}, nil
}

// Get will retrieve data from storage that corresponds to the specified key
func (c *fileStorageClient) Get(ctx context.Context, key string) ([]byte, error) {
	op := storage.GetOperation(key)
	err := c.Batch(ctx, op)
	if err != nil {
		return nil, err
	}

	return op.Value, nil
}

// Set will store data. The data can be retrieved using the same key
func (c *fileStorageClient) Set(ctx context.Context, key string, value []byte) error {
	return c.Batch(ctx, storage.SetOperation(key, value))
}

// Delete will delete data associated with the specified key
func (c *fileStorageClient) Delete(ctx context.Context, key string) error {
	return c.Batch(ctx, storage.DeleteOperation(key))
}

// Batch executes the specified operations in order. Get operation results are updated in place
func (c *fileStorageClient) Batch(_ context.Context, ops ...storage.Operation) error {
	batch := func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(defaultBucket)
		if bucket == nil {
			return errors.New("storage not initialized")
		}

		var err error
		for _, op := range ops {
			switch op.Type {
			case storage.Get:
				op.Value = bucket.Get([]byte(op.Key))
			case storage.Set:
				err = bucket.Put([]byte(op.Key), op.Value)
			case storage.Delete:
				err = bucket.Delete([]byte(op.Key))
			default:
				return errors.New("wrong operation type")
			}

			if err != nil {
				return err
			}
		}

		return nil
	}

	return c.db.Update(batch)
}

// Close will close the database
func (c *fileStorageClient) Close(_ context.Context) error {
	return c.db.Close()
}

// Compact database. Use temporary file as helper as we cannot replace database in-place
func (c *fileStorageClient) Compact(ctx context.Context, compactionDirectory string, timeout time.Duration, maxTransactionSize int64) (*fileStorageClient, error) {
	// create temporary file in compactionDirectory
	file, err := ioutil.TempFile(compactionDirectory, "tempdb")
	if err != nil {
		return nil, err
	}

	// use temporary file as compaction target
	options := bboltOptions(timeout)

	// cannot reuse newClient as db shouldn't contain any bucket
	db, err := bbolt.Open(file.Name(), 0600, options)
	if err != nil {
		return nil, err
	}

	if err := bbolt.Compact(db, c.db, maxTransactionSize); err != nil {
		return nil, err
	}

	dbPath := c.db.Path()
	tempDBPath := db.Path()

	db.Close()
	c.Close(ctx)

	// replace current db file with compacted db file
	if err := os.Remove(dbPath); err != nil {
		return nil, err
	}

	if err := os.Rename(tempDBPath, dbPath); err != nil {
		return nil, err
	}

	return newClient(dbPath, timeout)
}
