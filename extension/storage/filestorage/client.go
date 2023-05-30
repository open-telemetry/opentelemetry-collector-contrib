// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
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

	oneMiB = 1048576
)

type fileStorageClient struct {
	logger          *zap.Logger
	compactionMutex sync.RWMutex
	db              *bbolt.DB
	compactionCfg   *CompactionConfig
	openTimeout     time.Duration
	cancel          context.CancelFunc
	closed          bool
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

	client := &fileStorageClient{logger: logger, db: db, compactionCfg: compactionCfg, openTimeout: timeout}
	if compactionCfg.OnRebound {
		client.startCompactionLoop(context.Background())
	}

	return client, nil
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
				value := bucket.Get([]byte(op.Key))
				if value != nil {
					// the output of Bucket.Get is only valid within a transaction, so we need to make a copy
					// to be able to return the value
					op.Value = make([]byte, len(value))
					copy(op.Value, value)
				} else {
					op.Value = nil
				}
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
	return c.db.Update(batch)
}

// Close will close the database
func (c *fileStorageClient) Close(_ context.Context) error {
	c.compactionMutex.Lock()
	defer c.compactionMutex.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	c.closed = true
	return c.db.Close()
}

// Compact database. Use temporary file as helper as we cannot replace database in-place
func (c *fileStorageClient) Compact(compactionDirectory string, timeout time.Duration, maxTransactionSize int64) error {
	var err error
	var file *os.File
	var compactedDb *bbolt.DB

	// create temporary file in compactionDirectory
	file, err = os.CreateTemp(compactionDirectory, "tempdb")
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

	c.compactionMutex.Lock()
	defer c.compactionMutex.Unlock()
	if c.closed {
		c.logger.Debug("skipping compaction since database is already closed")
		return nil
	}

	c.logger.Debug("starting compaction",
		zap.String(directoryKey, c.db.Path()),
		zap.String(tempDirectoryKey, file.Name()))

	// cannot reuse newClient as db shouldn't contain any bucket
	compactedDb, err = bbolt.Open(file.Name(), 0600, options)
	if err != nil {
		return err
	}

	compactionStart := time.Now()

	if err = bbolt.Compact(compactedDb, c.db, maxTransactionSize); err != nil {
		return err
	}

	dbPath := c.db.Path()
	compactedDbPath := compactedDb.Path()

	c.db.Close()
	compactedDb.Close()

	var openErr error
	// replace current db file with compacted db file
	// we reopen the DB file irrespective of the success of the replace, as we can't leave it closed
	moveErr := moveFileWithFallback(compactedDbPath, dbPath)
	c.db, openErr = bbolt.Open(dbPath, 0600, options)

	// if we got errors for both rename and open, we'd rather return the open one
	// this should not happen in any kind of normal circumstance - maybe we should panic instead?
	if openErr != nil {
		return fmt.Errorf("failed to open db after compaction: %w", openErr)
	}
	if moveErr != nil {
		// if we only failed the remove, we're mostly ok and should just log a warning
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			if pathErr.Op == "remove" {
				c.logger.Warn("failed to remove temporary db after compaction",
					zap.String(directoryKey, c.db.Path()),
					zap.String(tempDirectoryKey, file.Name()),
					zap.Error(moveErr),
				)
				return nil
			}
		}
		return fmt.Errorf("failed to move compacted database, compaction aborted: %w", moveErr)
	}

	c.logger.Info("finished compaction",
		zap.String(directoryKey, dbPath),
		zap.Duration(elapsedKey, time.Since(compactionStart)))

	return nil
}

// startCompactionLoop provides asynchronous compaction function
func (c *fileStorageClient) startCompactionLoop(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)

	go func() {
		c.logger.Debug("starting compaction loop",
			zap.Duration("compaction_check_interval", c.compactionCfg.CheckInterval))

		compactionTicker := time.NewTicker(c.compactionCfg.CheckInterval)
		defer compactionTicker.Stop()

		for {
			select {
			case <-compactionTicker.C:
				if c.shouldCompact() {
					err := c.Compact(c.compactionCfg.Directory, c.openTimeout, c.compactionCfg.MaxTransactionSize)
					if err != nil {
						c.logger.Error("compaction failure",
							zap.String(directoryKey, c.compactionCfg.Directory),
							zap.Error(err))
					}
				}
			case <-ctx.Done():
				c.logger.Debug("shutting down compaction loop")
				return
			}
		}
	}()
}

// shouldCompact checks whether the conditions for online compaction are met
func (c *fileStorageClient) shouldCompact() bool {
	if !c.compactionCfg.OnRebound {
		return false
	}

	totalSizeBytes, dataSizeBytes, err := c.getDbSize()
	if err != nil {
		c.logger.Error("failed to get db size", zap.Error(err))
		return false
	}

	c.logger.Debug("shouldCompact check",
		zap.Int64("totalSizeBytes", totalSizeBytes),
		zap.Int64("dataSizeBytes", dataSizeBytes))

	if dataSizeBytes > c.compactionCfg.ReboundTriggerThresholdMiB*oneMiB ||
		totalSizeBytes < c.compactionCfg.ReboundNeededThresholdMiB*oneMiB {
		return false
	}

	c.logger.Debug("shouldCompact returns true",
		zap.Int64("totalSizeBytes", totalSizeBytes),
		zap.Int64("dataSizeBytes", dataSizeBytes))

	return true
}

func (c *fileStorageClient) getDbSize() (totalSizeResult int64, dataSizeResult int64, errResult error) {
	var totalSize int64

	err := c.db.View(func(tx *bbolt.Tx) error {
		totalSize = tx.Size()
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	dbStats := c.db.Stats()
	dataSize := totalSize - int64(dbStats.FreeAlloc)
	return totalSize, dataSize, nil
}

// moveFileWithFallback is the equivalent of os.Rename, except it falls back to
// a non-atomic Truncate and Copy if the arguments are on different filesystems
func moveFileWithFallback(src string, dest string) error {
	var err error
	if err = os.Rename(src, dest); err == nil {
		return nil
	}

	// EXDEV is the error code for linking cross-device, we want to continue if we encounter it
	// other errors, we simply return as-is
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	// if we're trying to rename across devices, try truncate and copy instead
	data, err := os.ReadFile(src) // assuming the file isn't too big
	if err != nil {
		return err
	}

	if err = os.Truncate(dest, 0); err != nil {
		return err
	}

	if err = os.WriteFile(dest, data, 0600); err != nil {
		return err
	}

	err = os.Remove(src)
	return err
}
