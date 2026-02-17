// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestClientOperations(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(t.Context()))
	})

	ctx := t.Context()
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
		require.NoError(t, client.Close(t.Context()))
	})

	ctx := t.Context()
	testSetEntries := []*storage.Operation{
		storage.SetOperation("testKey1", []byte("testValue1")),
		storage.SetOperation("testKey2", []byte("testValue2")),
	}

	testGetEntries := []*storage.Operation{
		storage.GetOperation("testKey1"),
		storage.GetOperation("testKey2"),
	}

	// Make sure nothing is there
	err = client.Batch(ctx, testGetEntries...)
	require.NoError(t, err)

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
	testEntriesUpdate := []*storage.Operation{
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
	testEntriesDelete := []*storage.Operation{
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
				value, err := c.Get(t.Context(), testKey)
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
				err := c.Set(t.Context(), testKey, testValue)
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
				err := c.Delete(t.Context(), testKey)
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
				require.NoError(t, client.Close(t.Context()))
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
				require.NoError(t, client.Close(t.Context()))
			})

			// 1. Fill up the database
			ctx := t.Context()

			entrySize := int64(400_000)

			numEntries := int64(0)
			for ; ; numEntries++ {
				batchWrite := []*storage.Operation{
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
		require.NoError(t, client.Close(t.Context()))
	})

	ctx := t.Context()

	// Make sure the compaction conditions will be met by putting and deleting large chunk of data
	batchWrite := []*storage.Operation{
		storage.SetOperation("large-payload", make([]byte, 10000000)),
	}
	err = client.Batch(ctx, batchWrite...)
	require.NoError(t, err)
	err = client.Batch(ctx, storage.DeleteOperation("large-payload"))
	require.NoError(t, err)

	// Start a couple of concurrent threads and see how they add/remove data as needed without failures
	clientOperationsThread := func(t *testing.T, id int) {
		repeats := 10
		for i := range repeats {
			batchWrite := []*storage.Operation{
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

	for i := range 10 {
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
		require.NoError(b, client.Close(b.Context()))
	})

	ctx := b.Context()
	testKey := "testKey"
	testValue := []byte("testValue")

	// Pre-populate so we measure "Get hit" performance
	require.NoError(b, client.Set(ctx, testKey, testValue))

	for b.Loop() {
		v, err := client.Get(ctx, testKey)
		require.NoError(b, err)
		require.Equal(b, testValue, v)
	}
}

func BenchmarkClientGet100(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(b.Context()))
	})

	ctx := b.Context()

	// Pre-populate keys so we measure "hit" batch performance
	for i := range 100 {
		key := fmt.Sprintf("testKey-%d", i)
		require.NoError(b, client.Set(ctx, key, []byte("testValue")))
	}

	testEntries := make([]*storage.Operation, 100)
	for i := range 100 {
		testEntries[i] = storage.GetOperation(fmt.Sprintf("testKey-%d", i))
	}

	for b.Loop() {
		require.NoError(b, client.Batch(ctx, testEntries...))
	}
}

func BenchmarkClientSet(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(b.Context()))
	})

	ctx := b.Context()
	testKey := "testKey"
	testValue := []byte("testValue")

	for b.Loop() {
		require.NoError(b, client.Set(ctx, testKey, testValue))
	}
}

func BenchmarkClientSet100(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(b.Context()))
	})
	ctx := b.Context()

	testEntries := make([]*storage.Operation, 100)
	for i := range 100 {
		testEntries[i] = storage.SetOperation(fmt.Sprintf("testKey-%d", i), []byte("testValue"))
	}

	for b.Loop() {
		require.NoError(b, client.Batch(ctx, testEntries...))
	}
}

func BenchmarkClientDelete(b *testing.B) {
	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	b.Cleanup(func() {
		require.NoError(b, client.Close(b.Context()))
	})

	ctx := b.Context()
	testKey := "testKey"
	testValue := []byte("testValue")

	// Setup: insert unique keys so they exist before being deleted
	// We create b.N distinct keys so each Delete is a delete-hit
	for i := range b.N {
		key := fmt.Sprintf("%s-%d", testKey, i)
		require.NoError(b, client.Set(ctx, key, testValue))
	}

	i := 0
	for b.Loop() {
		key := fmt.Sprintf("%s-%d", testKey, i)
		require.NoError(b, client.Delete(ctx, key))
		i++
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
		require.NoError(b, client.Close(b.Context()))
	})

	ctx := b.Context()

	// Prefill with large entries
	for n := range entryCount {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	// Delete them all to build a large freelist / large file
	for n := range entryCount {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Delete(ctx, testKey))
	}

	testKey = "testKey"
	testValue := []byte("testValue")

	for b.Loop() {
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

	// Setup: create large DB
	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)
	ctx := b.Context()

	for n := range entryCount {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	require.NoError(b, client.Close(ctx))

	var tempClient *fileStorageClient

	for b.Loop() {
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

	// Initial setup: create a large DB file with mostly deleted data
	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)

	ctx := b.Context()

	for n := range entryCount {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	// Leave one key in the db
	for n := 0; n < entryCount-1; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Delete(ctx, testKey))
	}

	require.NoError(b, client.Close(ctx))

	for n := range b.N {
		testDbFile := filepath.Join(tempDir, fmt.Sprintf("my_db%d", n))
		err = os.Link(dbFile, testDbFile)
		require.NoError(b, err)

		client, err = newClient(zap.NewNop(), testDbFile, time.Second, &CompactionConfig{}, false)
		require.NoError(b, err)

		b.StartTimer()
		require.NoError(b, client.Compact(tempDir, time.Second, 65536))
		b.StopTimer()

		require.NoError(b, client.Close(ctx))
	}
}

func BenchmarkClientCompactDb(b *testing.B) {
	entrySizeInBytes := 1024 * 128
	entryCount := 160
	entry := make([]byte, entrySizeInBytes)
	var testKey string

	tempDir := b.TempDir()
	dbFile := filepath.Join(tempDir, "my_db")

	// Setup: fill DB, then delete half of the keys
	client, err := newClient(zap.NewNop(), dbFile, time.Second, &CompactionConfig{}, false)
	require.NoError(b, err)

	ctx := b.Context()

	for n := range entryCount {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Set(ctx, testKey, entry))
	}

	// Leave half the keys in the DB
	for n := 0; n < entryCount/2; n++ {
		testKey = fmt.Sprintf("testKey-%d", n)
		require.NoError(b, client.Delete(ctx, testKey))
	}

	require.NoError(b, client.Close(ctx))

	for n := range b.N {
		testDbFile := filepath.Join(tempDir, fmt.Sprintf("my_db%d", n))
		err = os.Link(dbFile, testDbFile)
		require.NoError(b, err)

		client, err = newClient(zap.NewNop(), testDbFile, time.Second, &CompactionConfig{}, false)
		require.NoError(b, err)

		b.StartTimer()
		require.NoError(b, client.Compact(tempDir, time.Second, 65536))
		b.StopTimer()

		require.NoError(b, client.Close(ctx))
	}
}
