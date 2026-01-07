package starrocksexporter

import (
	"database/sql"
	"fmt"
	"sync"
)

// dbClient holds the database connection and a reference count.
type dbClient struct {
	db        *sql.DB
	refCount  int
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
			return client.db, nil
		}
	}

	// Create new DB connection outside the map check
	// to avoid race conditions with concurrent GetDBClient calls
	db, err := cfg.buildStarRocksDB()
	if err != nil {
		return nil, err
	}

	// Double-check: another goroutine might have created a client while we were building ours
	if client, ok := clientMap[endpoint]; ok {
		if !client.cleanedUp {
			// Another goroutine created a client, use it and close the one we just built
			db.Close() // Close the duplicate connection
			client.refCount++
			return client.db, nil
		}
		// If cleaned up, delete and use our new one
		delete(clientMap, endpoint)
	}

	clientMap[endpoint] = &dbClient{
		db:        db,
		refCount:  1,
		cleanedUp: false,
	}

	return db, nil
}

// ReleaseDBClient decrements the reference count for the client associated with the config.
// When refCount reaches zero, the pool is closed immediately to prevent connection accumulation.
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

	// Close immediately if no more references
	if client.refCount == 0 && !client.cleanedUp {
		if err := client.db.Close(); err != nil {
			// Log error but continue with cleanup
			fmt.Printf("warning: failed to close connection pool for %s: %v\n", endpoint, err)
		}
		client.cleanedUp = true
		delete(clientMap, endpoint)
	}

	return nil
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
