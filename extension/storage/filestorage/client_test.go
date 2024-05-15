// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestClientOperations(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.TODO()))
	})

	ctx := context.Background()
	testKey := "testKey"
	testValue := []byte("testValue")

	// Make sure nothing is there
	value, err := client.Get(ctx, testKey)
	require.NoError(t, err)
	require.Nil(t, value)

	// Set it
	err = client.Set(ctx, testKey, testValue)
	require.NoError(t, err)

	// Get it back out, make sure it's right
	value, err = client.Get(ctx, testKey)
	require.NoError(t, err)
	require.Equal(t, testValue, value)

	// Delete it
	err = client.Delete(ctx, testKey)
	require.NoError(t, err)

	// Make sure it's gone
	value, err = client.Get(ctx, testKey)
	require.NoError(t, err)
	require.Nil(t, value)
}

func TestClientBatchOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(context.TODO()))
	})

	ctx := context.Background()
	testSetEntries := []storage.Operation{
		storage.SetOperation("testKey1", []byte("testValue1")),
		storage.SetOperation("testKey2", []byte("testValue2")),
	}

	testGetEntries := []storage.Operation{
		storage.GetOperation("testKey1"),
		storage.GetOperation("testKey2"),
	}

	// Make sure nothing is there
	err = client.Batch(ctx, testGetEntries...)
	require.NoError(t, err)
	require.Equal(t, testGetEntries, testGetEntries)

	// Set it
	err = client.Batch(ctx, testSetEntries...)
	require.NoError(t, err)

	// Get it back out, make sure it's right
	err = client.Batch(ctx, testGetEntries...)
	require.NoError(t, err)
	for i := range testGetEntries {
		require.Equal(t, testSetEntries[i].Key, testGetEntries[i].Key)
		require.Equal(t, testSetEntries[i].Value, testGetEntries[i].Value)
	}

	// Update it (the first entry should be empty and the second one removed)
	testEntriesUpdate := []storage.Operation{
		storage.SetOperation("testKey1", []byte{}),
		storage.DeleteOperation("testKey2"),
	}
	err = client.Batch(ctx, testEntriesUpdate...)
	require.NoError(t, err)

	// Get it back out, make sure it's right
	err = client.Batch(ctx, testGetEntries...)
	require.NoError(t, err)
	for i := range testGetEntries {
		require.Equal(t, testEntriesUpdate[i].Key, testGetEntries[i].Key)
		require.Equal(t, testEntriesUpdate[i].Value, testGetEntries[i].Value)
	}

	// Delete it all
	testEntriesDelete := []storage.Operation{
		storage.DeleteOperation("testKey1"),
		storage.DeleteOperation("testKey2"),
	}
	err = client.Batch(ctx, testEntriesDelete...)
	require.NoError(t, err)

	// Make sure it's gone
	err = client.Batch(ctx, testGetEntries...)
	require.NoError(t, err)
	for i := range testGetEntries {
		require.Equal(t, testGetEntries[i].Key, testEntriesDelete[i].Key)
		require.Nil(t, testGetEntries[i].Value)

	}
}

func TestNewClientTransactionErrors(t *testing.T) {
	timeout := 100 * time.Millisecond

	testKey := "testKey"
	testValue := []byte("testValue")

	testCases := []struct {
		name     string
		setup    func(*bbolt.Tx) error
		validate func(*testing.T, *fileStorageClient)
	}{
		{
			name: "get",
			setup: func(tx *bbolt.Tx) error {
				return tx.DeleteBucket(defaultBucket)
			},
			validate: func(t *testing.T, c *fileStorageClient) {
				value, err := c.Get(context.Background(), testKey)
				require.Error(t, err)
				require.Equal(t, "storage not initialized", err.Error())
				require.Nil(t, value)
			},
		},
		{
			name: "set",
			setup: func(tx *bbolt.Tx) error {
				return tx.DeleteBucket(defaultBucket)
			},
			validate: func(t *testing.T, c *fileStorageClient) {
				err := c.Set(context.Background(), testKey, testValue)
				require.Error(t, err)
				require.Equal(t, "storage not initialized", err.Error())
			},
		},
		{
			name: "delete",
			setup: func(tx *bbolt.Tx) error {
				return tx.DeleteBucket(defaultBucket)
			},
			validate: func(t *testing.T, c *fileStorageClient) {
				err := c.Delete(context.Background(), testKey)
				require.Error(t, err)
				require.Equal(t, "storage not initialized", err.Error())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			tempDir := t.TempDir()
			dbFile := filepath.Join(tempDir, "my_db")

			client, err := newClient(zap.NewNop(), dbFile, timeout, &CompactionConfig{}, false)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, client.Close(context.TODO()))
			})

			// Create a problem
			require.NoError(t, client.db.Update(tc.setup))

			// Validate expected behavior
			tc.validate(t, client)

			require.NoError(t, client.db.Close())
		})
	}
}

func TestNewClientErrorsOnInvalidBucket(t *testing.T) {
	temp := defaultBucket
	defaultBucket = nil

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.Error(t, err)
	require.Nil(t, client)

	defaultBucket = temp
}

func TestClientReboundCompaction(t *testing.T) {
	testCases := []struct {
		testName                   string
		reboundNeededThresholdMiB  int64
		reboundTriggerThresholdMiB int64
		fillStorageAboveMiB        int64
		drainStorageBelowMiB       int64
		shouldTriggerCompaction    bool
	}{
		{
			testName:                   "should trigger compaction",
			reboundNeededThresholdMiB:  4,
			reboundTriggerThresholdMiB: 1,
			fillStorageAboveMiB:        10,
			drainStorageBelowMiB:       1,
			shouldTriggerCompaction:    true,
		},
		{
			testName:                   "should not trigger compaction because upper threshold not met",
			reboundNeededThresholdMiB:  20,
			reboundTriggerThresholdMiB: 1,
			fillStorageAboveMiB:        10,
			drainStorageBelowMiB:       1,
			shouldTriggerCompaction:    false,
		},
		{
			testName:                   "should not trigger compaction because lower threshold not met",
			reboundNeededThresholdMiB:  10,
			reboundTriggerThresholdMiB: 1,
			fillStorageAboveMiB:        20,
			drainStorageBelowMiB:       5,
			shouldTriggerCompaction:    false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			tempDir := t.TempDir()
			dbFile := filepath.Join(tempDir, "my_db")

			checkInterval := time.Second

			logger, _ := zap.NewDevelopment()
			client, err := newClient(logger, dbFile, time.Second, &CompactionConfig{
				OnRebound:                  true,
				CheckInterval:              checkInterval,
				ReboundNeededThresholdMiB:  testCase.reboundNeededThresholdMiB,
				ReboundTriggerThresholdMiB: testCase.reboundTriggerThresholdMiB,
			}, false)
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, client.Close(context.TODO()))
			})

			// 1. Fill up the database
			ctx := context.Background()

			entrySize := int64(400_000)

			numEntries := int64(0)
			for ; ; numEntries++ {
				batchWrite := []storage.Operation{
					storage.SetOperation(fmt.Sprintf("foo-%d", numEntries), make([]byte, entrySize)),
					storage.SetOperation(fmt.Sprintf("bar-%d", numEntries), []byte("testValueBar")),
				}
				err = client.Batch(ctx, batchWrite...)
				require.NoError(t, err)

				totalSize, _, err := client.getDbSize()
				require.NoError(t, err)
				if totalSize > testCase.fillStorageAboveMiB*oneMiB {
					break
				}
			}

			require.Eventually(t,
				func() bool {
					totalSize, _, dbErr := client.getDbSize()
					require.NoError(t, dbErr)
					return totalSize > testCase.fillStorageAboveMiB*oneMiB
				},
				10*time.Second, 5*time.Millisecond, "database allocated space for data",
			)

			// 2. Remove the large entries
			for i := 0; i < int(numEntries); i++ {
				_, realSize, err := client.getDbSize()
				require.NoError(t, err)
				if realSize < testCase.drainStorageBelowMiB*oneMiB {
					break
				}

				err = client.Batch(ctx, storage.DeleteOperation(fmt.Sprintf("foo-%d", i)))
				require.NoError(t, err)
			}

			if testCase.shouldTriggerCompaction {
				require.Eventually(t,
					func() bool {
						// The check is performed while the database might be compacted, hence we're reusing the mutex here
						// (getDbSize is not called from outside the compaction loop otherwise)
						client.compactionMutex.Lock()
						defer client.compactionMutex.Unlock()

						totalSize, _, dbErr := client.getDbSize()
						require.NoError(t, dbErr)
						return totalSize < testCase.drainStorageBelowMiB*oneMiB
					},
					10*time.Second, 5*time.Millisecond, "Compaction did not happen, but it should have.",
				)
			} else {
				// Wait for compaction check interval (twice) to make sure compaction does not happen.
				time.Sleep(checkInterval * 2)

				// Check that compaction did not happen.
				totalSize, _, dbErr := client.getDbSize()
				require.NoError(t, dbErr)
				require.GreaterOrEqual(t, totalSize, testCase.fillStorageAboveMiB*oneMiB, "Compaction happened, but it should have not.")
			}
		})
	}
}

func TestClientConcurrentCompaction(t *testing.T) {
	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)

	tempDir := t.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	stepInterval := time.Millisecond * 5

	client, err := newClient(logger, dbFile, time.Second, &CompactionConfig{
		OnRebound:                  true,
		CheckInterval:              stepInterval * 2,
		ReboundNeededThresholdMiB:  1,
		ReboundTriggerThresholdMiB: 5,
	}, false)
	require.NoError(t, err)

	t.Cleanup(func() {
		// At least one compaction should have happened
		require.GreaterOrEqual(t, len(logObserver.FilterMessage("finished compaction").All()), 1)
		require.NoError(t, client.Close(context.TODO()))
	})

	ctx := context.Background()

	// Make sure the compaction conditions will be met by putting and deleting large chunk of data
	batchWrite := []storage.Operation{
		storage.SetOperation("large-payload", make([]byte, 10000000)),
	}
	err = client.Batch(ctx, batchWrite...)
	require.NoError(t, err)
	err = client.Batch(ctx, storage.DeleteOperation("large-payload"))
	require.NoError(t, err)

	// Start a couple of concurrent threads and see how they add/remove data as needed without failures
	clientOperationsThread := func(t *testing.T, id int) {
		repeats := 10
		for i := 0; i < repeats; i++ {
			batchWrite := []storage.Operation{
				storage.SetOperation(fmt.Sprintf("foo-%d-%d", id, i), make([]byte, 1000)),
				storage.SetOperation(fmt.Sprintf("bar-%d-%d", id, i), []byte("testValueBar")),
			}
			terr := client.Batch(ctx, batchWrite...)
			require.NoError(t, terr)

			terr = client.Batch(ctx, storage.DeleteOperation(fmt.Sprintf("foo-%d-%d", id, i)))
			require.NoError(t, terr)

			result, terr := client.Get(ctx, fmt.Sprintf("foo-%d-%d", id, i))
			require.NoError(t, terr)
			require.Equal(t, []byte(nil), result)

			result, terr = client.Get(ctx, fmt.Sprintf("bar-%d-%d", id, i))
			require.NoError(t, terr)
			require.Equal(t, []byte("testValueBar"), result)

			// Make sure the requests are somewhat spaced
			time.Sleep(stepInterval)
		}
	}

	for i := 0; i < 10; i++ {
		i := i
		t.Run(fmt.Sprintf("client-operations-thread-%d", i), func(t *testing.T) {
			t.Parallel()
			clientOperationsThread(t, i)
		})
	}
}

func BenchmarkClientGet(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()
	testKey := "testKey"

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err = client.Get(ctx, testKey)
		require.NoError(b, err)
	}
}

func BenchmarkClientGet100(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()

	testEntries := make([]storage.Operation, 100)
	for i := 0; i < 100; i++ {
		testEntries[i] = storage.GetOperation(fmt.Sprintf("testKey-%d", i))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		require.NoError(b, client.Batch(ctx, testEntries...))
	}
}

func BenchmarkClientSet(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()
	testKey := "testKey"
	testValue := []byte("testValue")

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		require.NoError(b, client.Set(ctx, testKey, testValue))
	}
}

func BenchmarkClientSet100(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})
	ctx := context.Background()

	testEntries := make([]storage.Operation, 100)
	for i := 0; i < 100; i++ {
		testEntries[i] = storage.SetOperation(fmt.Sprintf("testKey-%d", i), []byte("testValue"))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		require.NoError(b, client.Batch(ctx, testEntries...))
	}
}

func BenchmarkClientDelete(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()
	testKey := "testKey"

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		require.NoError(b, client.Delete(ctx, testKey))
	}
}

// check the performance impact of the max lifetime DB size
// bolt doesn't compact the freelist automatically, so there's a cost even if the data is deleted
func BenchmarkClientSetLargeDB(b *testing.B) {
	entrySizeInBytes := 1024 * 1024
	entryCount := 2000
	entry := make([]byte, entrySizeInBytes)
	var testKey string

	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()

	for n := 0; n < entryCount; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	for n := 0; n < entryCount; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Delete(ctx, testKey))
	}

	testKey = "testKey"
	testValue := []byte("testValue")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		require.NoError(b, client.Set(ctx, testKey, testValue))
	}
}

// check the cost of opening an existing DB with data
// this can change depending on freelist type and whether it's synced to disk
func BenchmarkClientInitLargeDB(b *testing.B) {
	entrySizeInBytes := 1024 * 1024
	entry := make([]byte, entrySizeInBytes)
	entryCount := 2000
	var testKey string

	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()

	for n := 0; n < entryCount; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	err = client.Close(ctx)
	require.NoError(b, err)

	var tempClient *fileStorageClient
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		tempClient, err = newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
		require.NoError(b, err)
		b.StopTimer()
		err = tempClient.Close(ctx)
		require.NoError(b, err)
		b.StartTimer()
	}
}

func BenchmarkClientCompactLargeDBFile(b *testing.B) {
	entrySizeInBytes := 1024 * 1024
	entryCount := 2000
	entry := make([]byte, entrySizeInBytes)
	var testKey string

	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()

	for n := 0; n < entryCount; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	// Leave one key in the db
	for n := 0; n < entryCount-1; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Delete(ctx, testKey))
	}

	require.NoError(b, client.Close(ctx))

	b.ResetTimer()
	b.StopTimer()
	for n := 0; n < b.N; n++ {
		testDbFile := filepath.Join(tempDir, fmt.Sprintf("my_db%d", n))
		err = os.Link(dbFile, testDbFile)
		require.NoError(b, err)
		client, err = newClient(zap.NewNop(), testDbFile, time.Second, &CompactionConfig{}, false)
		require.NoError(b, err)
		b.StartTimer()
		require.NoError(b, client.Compact(tempDir, time.Second, 65536))
		b.StopTimer()
	}
}

func BenchmarkClientCompactDb(b *testing.B) {
	entrySizeInBytes := 1024 * 128
	entryCount := 160
	entry := make([]byte, entrySizeInBytes)
	var testKey string

	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(context.TODO()))
	})

	ctx := context.Background()

	for n := 0; n < entryCount; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	// Leave half the keys in the DB
	for n := 0; n < entryCount/2; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Delete(ctx, testKey))
	}

	require.NoError(b, client.Close(ctx))

	b.ResetTimer()
	b.StopTimer()
	for n := 0; n < b.N; n++ {
		testDbFile := filepath.Join(tempDir, fmt.Sprintf("my_db%d", n))
		err = os.Link(dbFile, testDbFile)
		require.NoError(b, err)
		client, err = newClient(zap.NewNop(), testDbFile, time.Second, &CompactionConfig{}, false)
		require.NoError(b, err)
		b.StartTimer()
		require.NoError(b, client.Compact(tempDir, time.Second, 65536))
		b.StopTimer()
	}
}
