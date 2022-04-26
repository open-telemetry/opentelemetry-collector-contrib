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
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

var defaultBucket = []byte(`default`)

const (
	elapsedKey       = "elapsed"
	directoryKey     = "directory"
	tempDirectoryKey = "tempDirectory"
)

type fileStorageClient struct {
	logger          *zap.Logger
	compactionMutex sync.RWMutex
	db              *bbolt.DB
	compactionCfg   *CompactionConfig
	openTimeout     time.Duration
}

func bboltOptions(timeout time.Duration) *bbolt.Options {
	return &bbolt.Options{
		Timeout:        timeout,
		NoSync:         true,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	}
}

func newClient(logger *zap.Logger, filePath string, timeout time.Duration, compactionCfg *CompactionConfig) (*fileStorageClient, error) {
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
		_ = db.Close()
		return nil, err
	}

	return &fileStorageClient{logger: logger, db: db, compactionCfg: compactionCfg, openTimeout: timeout}, nil
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
func (c *fileStorageClient) Batch(ctx context.Context, ops ...storage.Operation) error {
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

	c.compactionMutex.RLock()
	defer c.compactionMutex.RUnlock()
	err := c.db.Update(batch)
	if c.shouldCompactOnWrite() {
		go func() {
			onWriteErr := c.Compact(ctx, c.compactionCfg.Directory, c.openTimeout, c.compactionCfg.MaxTransactionSize)
			if onWriteErr != nil {
				c.logger.Error("compaction failure",
					zap.String(directoryKey, c.compactionCfg.Directory),
					zap.Error(onWriteErr))
			}
		}()
	}
	return err
}

// Close will close the database
func (c *fileStorageClient) Close(_ context.Context) error {
	return c.db.Close()
}

// Compact database. Use temporary file as helper as we cannot replace database in-place
func (c *fileStorageClient) Compact(ctx context.Context, compactionDirectory string, timeout time.Duration, maxTransactionSize int64) error {
	var err error
	var file *os.File
	var compactedDb *bbolt.DB

	// create temporary file in compactionDirectory
	file, err = ioutil.TempFile(compactionDirectory, "tempdb")
	if err != nil {
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}

	defer func() {
		_, statErr := os.Stat(file.Name())
		if statErr == nil {
			// File still exists and needs to be removed
			if removeErr := os.Remove(file.Name()); removeErr != nil {
				c.logger.Error("removing temporary compaction file failed", zap.Error(removeErr))
			}
		}
	}()

	// use temporary file as compaction target
	options := bboltOptions(timeout)

	c.logger.Debug("starting compaction",
		zap.String(directoryKey, c.db.Path()),
		zap.String(tempDirectoryKey, file.Name()))

	// cannot reuse newClient as db shouldn't contain any bucket
	compactedDb, err = bbolt.Open(file.Name(), 0600, options)
	if err != nil {
		return err
	}

	c.compactionMutex.Lock()
	defer c.compactionMutex.Unlock()
	compactionStart := time.Now()

	if err = bbolt.Compact(compactedDb, c.db, maxTransactionSize); err != nil {
		return err
	}

	dbPath := c.db.Path()
	compactedDbPath := compactedDb.Path()

	c.db.Close()
	compactedDb.Close()

	// replace current db file with compacted db file
	if err = os.Remove(dbPath); err != nil {
		return err
	}

	if err = os.Rename(compactedDbPath, dbPath); err != nil {
		return err
	}

	c.db, err = bbolt.Open(dbPath, 0600, options)

	c.logger.Info("finished compaction",
		zap.String(directoryKey, dbPath),
		zap.Duration(elapsedKey, time.Since(compactionStart)))

	return err
}

// shouldCompactOnWrite checks whether the conditions for online compaction are met
func (c *fileStorageClient) shouldCompactOnWrite() bool {
	if !c.compactionCfg.OnRebound {
		return false
	}

	dbStats := c.db.Stats()
	if dbStats.FreelistInuse > int(c.compactionCfg.ReboundSizeBelowMiB) ||
		dbStats.FreeAlloc+dbStats.FreelistInuse < int(c.compactionCfg.ReboundTotalSizeAboveMiB) {
		return false
	}

	c.logger.Debug("shouldCompactOnWrite returns true",
		zap.Int("FreelistInuse", dbStats.FreelistInuse),
		zap.Int("FreeAlloc", dbStats.FreeAlloc))

	return true
}
