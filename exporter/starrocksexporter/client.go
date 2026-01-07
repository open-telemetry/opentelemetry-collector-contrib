package starrocksexporter

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// dbClient holds the database connection and a reference count.
type dbClient struct {
	db        *sql.DB
	refCount  int
	lastUsed  time.Time
	cleanedUp bool
}

var (
	// clientMap stores shared database clients keyed by endpoint (host:port).
	// This ensures all exporters connecting to the same StarRocks instance
	// share the same connection pool, regardless of database.
	clientMap = make(map[string]*dbClient)
	// clientMu protects access to clientMap.
	clientMu sync.Mutex
)

const (
	// poolCleanupTimeout is how long to keep an idle pool before closing it.
	// This allows pools to be reused during collector restarts without
	// accumulating connections indefinitely.
	poolCleanupTimeout = 1 * time.Minute
)

// GetDBClient returns a shared database client for the given config.
// If a client for the endpoint already exists, its reference count is incremented.
// Otherwise, a new client is created.
// The pool is keyed by endpoint (host:port) to maximize connection reuse.
func GetDBClient(cfg *Config) (*sql.DB, error) {
	clientMu.Lock()
	defer clientMu.Unlock()

	// Use endpoint as key instead of full DSN
	// This allows multiple exporters to connect to different databases
	// on the same StarRocks instance while sharing the connection pool
	endpoint := cfg.Endpoint

	if client, ok := clientMap[endpoint]; ok {
		if client.cleanedUp {
			// Pool was marked for cleanup, create a new one
			delete(clientMap, endpoint)
		} else {
			client.refCount++
			client.lastUsed = time.Now()
			return client.db, nil
		}
	}

	// Create new DB connection
	db, err := cfg.buildStarRocksDB()
	if err != nil {
		return nil, err
	}

	clientMap[endpoint] = &dbClient{
		db:        db,
		refCount:  1,
		lastUsed:  time.Now(),
		cleanedUp: false,
	}

	return db, nil
}

// ReleaseDBClient decrements the reference count for the client associated with the config.
// When refCount reaches zero, the pool is marked for cleanup and will be closed
// after poolCleanupTimeout if no new exporters start using it.
func ReleaseDBClient(cfg *Config) error {
	clientMu.Lock()
	defer clientMu.Unlock()

	endpoint := cfg.Endpoint

	client, ok := clientMap[endpoint]
	if !ok {
		// Client not found, maybe already released or never created
		return nil
	}

	// Decrement refCount
	client.refCount--
	if client.refCount < 0 {
		client.refCount = 0
	}

	client.lastUsed = time.Now()

	// If no more references, schedule cleanup
	if client.refCount == 0 && !client.cleanedUp {
		// Start background goroutine to cleanup after timeout
		go cleanupIdlePool(endpoint, poolCleanupTimeout)
	}

	return nil
}

// cleanupIdlePool waits for the timeout and then closes the pool if still idle.
// This function runs in a background goroutine.
func cleanupIdlePool(endpoint string, timeout time.Duration) {
	time.Sleep(timeout)

	clientMu.Lock()
	defer clientMu.Unlock()

	client, ok := clientMap[endpoint]
	if !ok {
		return
	}

	// Only close if still idle (refCount == 0) and enough time has passed
	if client.refCount == 0 && time.Since(client.lastUsed) >= timeout {
		if err := client.db.Close(); err != nil {
			// Log error but continue with cleanup
			fmt.Printf("warning: failed to close connection pool for %s: %v\n", endpoint, err)
		}
		client.cleanedUp = true
		delete(clientMap, endpoint)
	}
}

// CloseAllDBClients closes all database connections and clears the connection pool.
// This should only be called when shutting down the entire collector.
func CloseAllDBClients() error {
	clientMu.Lock()
	defer clientMu.Unlock()

	var errs []error
	for endpoint, client := range clientMap {
		if err := client.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection for endpoint %s: %w", endpoint, err))
		}
	}

	// Clear the map
	clientMap = make(map[string]*dbClient)

	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors while closing connections: %v", len(errs), errs)
	}

	return nil
}
