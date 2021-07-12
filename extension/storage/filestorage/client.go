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

package filestorage

import (
	"context"
	"errors"
	"time"

	"go.etcd.io/bbolt"
)

var defaultBucket = []byte(`default`)

type fileStorageClient struct {
	db *bbolt.DB
}

func newClient(filePath string, timeout time.Duration) (*fileStorageClient, error) {
	options := &bbolt.Options{
		Timeout: timeout,
		NoSync:  true,
	}
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
	results, err := c.GetBatch(ctx, []string{key})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// GetBatch will retrieve data from storage that corresponds to the specified collection of keys in a single transaction
func (c *fileStorageClient) GetBatch(_ context.Context, keys []string) ([][]byte, error) {
	results := make([][]byte, len(keys))
	get := func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(defaultBucket)
		if bucket == nil {
			return errors.New("storage not initialized")
		}

		for i, key := range keys {
			results[i] = bucket.Get([]byte(key))
		}
		return nil // no error
	}

	if err := c.db.Update(get); err != nil {
		return nil, err
	}
	return results, nil
}

// Set will store data. The data can be retrieved using the same key
func (c *fileStorageClient) Set(ctx context.Context, key string, value []byte) error {
	return c.SetBatch(ctx, map[string][]byte{key: value})
}

// SetBatch will store data for a set of entries in a transaction. The data can be retrieved using the same keys
func (c *fileStorageClient) SetBatch(_ context.Context, entries map[string][]byte) error {
	set := func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(defaultBucket)
		if bucket == nil {
			return errors.New("storage not initialized")
		}

		for key, value := range entries {
			var err error

			if value == nil {
				err = bucket.Delete([]byte(key))
			} else {
				err = bucket.Put([]byte(key), value)
			}

			if err != nil {
				return err
			}
		}

		return nil
	}

	return c.db.Update(set)
}

// Delete will delete data associated with the specified key
func (c *fileStorageClient) Delete(ctx context.Context, key string) error {
	return c.DeleteBatch(ctx, []string{key})
}

// DeleteBatch will delete data associated with the specified keys in a single transaction
func (c *fileStorageClient) DeleteBatch(_ context.Context, keys []string) error {
	delete := func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(defaultBucket)
		if bucket == nil {
			return errors.New("storage not initialized")
		}
		for _, key := range keys {
			if err := bucket.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	}

	return c.db.Update(delete)
}

// Close will close the database
func (c *fileStorageClient) Close(_ context.Context) error {
	return c.db.Close()
}
