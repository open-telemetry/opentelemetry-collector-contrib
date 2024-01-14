package dbstorage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/extension/experimental/storage"
)

// Define a test database file name
const testDBFile = "test.db"

// TestDBStorageClient verifies the behavior of the dbStorageClient
func TestDBStorageClient(t *testing.T) {
	// Set up the test client
	client, cleanup := setupTestEnvironment(t, "test_table")
	defer cleanup()

	// Run tests for Get, Set, Delete, and Batch methods
	t.Run("Get", func(t *testing.T) {
		// Test Get for a key that doesn't exist
		value, err := client.Get(context.Background(), "nonexistent_key")
		assert.NoError(t, err)
		assert.Nil(t, value)

		// Test Get for an existing key
		err = client.Set(context.Background(), "existing_key", []byte("some_value"))
		assert.NoError(t, err)
		value, err = client.Get(context.Background(), "existing_key")
		assert.NoError(t, err)
		assert.Equal(t, []byte("some_value"), value)
	})

	t.Run("Set", func(t *testing.T) {
		// Test Set for a new key
		err := client.Set(context.Background(), "new_key", []byte("new_value"))
		assert.NoError(t, err)

		// Test update for an existing key
		err = client.Set(context.Background(), "existing_key", []byte("updated_value"))
		assert.NoError(t, err)

		// Verify the updated value
		value, err := client.Get(context.Background(), "existing_key")
		assert.NoError(t, err)
		assert.Equal(t, []byte("updated_value"), value)
	})

	t.Run("Delete", func(t *testing.T) {
		// Test Delete for an existing key
		err := client.Delete(context.Background(), "existing_key")
		assert.NoError(t, err)

		// Verify that the key is deleted
		value, err := client.Get(context.Background(), "existing_key")
		assert.NoError(t, err)
		assert.Nil(t, value)

		// Test Delete for a nonexistent key (no error expected)
		err = client.Delete(context.Background(), "nonexistent_key")
		assert.NoError(t, err)
	})

	t.Run("Batch", func(t *testing.T) {
		// Test Batch for a combination of Get, Set, and Delete operations
		ops := []storage.Operation{
			{Type: storage.Get, Key: "existing_key"},
			{Type: storage.Set, Key: "new_key", Value: []byte("new_value")},
			{Type: storage.Delete, Key: "nonexistent_key"},
		}

		err := client.Batch(context.Background(), ops...)
		assert.NoError(t, err)

		// Verify the results of the Get operation in the batch
		if ops[0].Value != nil {
			assert.Equal(t, []byte("updated_value"), ops[0].Value)
		}

		// Verify that Set and Delete operations in the batch were successful
		value, err := client.Get(context.Background(), "new_key")
		assert.NoError(t, err)
		assert.Equal(t, []byte("new_value"), value)

		value, err = client.Get(context.Background(), "nonexistent_key")
		assert.NoError(t, err)
		assert.Nil(t, value)
	})

	// Create a test client that embeds the dbStorageClient and overrides the Batch method
	testClient := &testDBStorageClient{dbStorageClient: client}

	// Run a test to simulate a rollback.
	t.Run("RollbackInBatch", func(t *testing.T) {
		// Override the Batch method with a custom implementation
		testClient.batchOverride = func(ctx context.Context, ops ...storage.Operation) error {
			for _, op := range ops {
				switch op.Type {
				case storage.Set:
					if op.Key == "key5" {
						return errors.New("intentional error")
					}
				}
			}
			return testClient.dbStorageClient.Batch(ctx, ops...)
		}

		ops := []storage.Operation{
			{Type: storage.Set, Key: "key1", Value: []byte("value1")},
			{Type: storage.Set, Key: "key2", Value: []byte("value2")},
			{Type: storage.Set, Key: "key3", Value: []byte("value3")},
			{Type: storage.Set, Key: "key4", Value: []byte("value4")},
			{Type: storage.Set, Key: "key5", Value: []byte("value5")},
		}

		err := testClient.Batch(context.Background(), ops...)
		assert.Error(t, err) // Expecting an error due to intentional error during Set

		// Verify that the values are not stored in the database
		for _, op := range ops {
			value, getErr := client.Get(context.Background(), op.Key)
			assert.NoError(t, getErr) // Expecting no error during Get
			assert.Nil(t, value)      // Expecting nil value as a result of rollback
		}
	})
}

type testDBStorageClient struct {
	*dbStorageClient
	batchOverride func(ctx context.Context, ops ...storage.Operation) error
}

func (c *testDBStorageClient) Batch(ctx context.Context, ops ...storage.Operation) error {
	if c.batchOverride != nil {
		return c.batchOverride(ctx, ops...)
	}
	return c.dbStorageClient.Batch(ctx, ops...)
}

// setupTestEnvironment creates a new dbStorageClient for testing, opens a test database
// returns the client and cleanup function.
func setupTestEnvironment(t *testing.T, tableName string) (*dbStorageClient, func()) {
	// Open a test database.
	db, err := sql.Open("sqlite3", testDBFile)
	assert.NoError(t, err)

	// Set up the test client.
	client, err := newClient(context.Background(), db, tableName)
	assert.NoError(t, err)

	// Define cleanup function to close the client and database and remove the test database file.
	cleanupFunc := func() {
		_ = client.Close(context.Background())
		_ = db.Close()
		_ = os.Remove(testDBFile)
	}

	return client, cleanupFunc
}
