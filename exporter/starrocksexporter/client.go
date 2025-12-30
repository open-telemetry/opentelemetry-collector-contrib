package starrocksexporter

import (
	"database/sql"
	"fmt"
	"sync"
)

// dbClient holds the database connection and a reference count.
type dbClient struct {
	db       *sql.DB
	refCount int
}

var (
	// clientMap stores shared database clients keyed by DSN.
	clientMap = make(map[string]*dbClient)
	// clientMu protects access to clientMap.
	clientMu sync.Mutex
)

// GetDBClient returns a shared database client for the given config.
// If a client for the DSN already exists, its reference count is incremented.
// Otherwise, a new client is created.
func GetDBClient(cfg *Config) (*sql.DB, error) {
	clientMu.Lock()
	defer clientMu.Unlock()

	dsn, err := cfg.buildDSN()
	if err != nil {
		return nil, fmt.Errorf("failed to build DSN: %w", err)
	}

	if client, ok := clientMap[dsn]; ok {
		client.refCount++
		return client.db, nil
	}

	// Create new DB connection
	db, err := cfg.buildStarRocksDB()
	if err != nil {
		return nil, err
	}

	clientMap[dsn] = &dbClient{
		db:       db,
		refCount: 1,
	}

	return db, nil
}

// ReleaseDBClient decrements the reference count for the client associated with the config.
// If the reference count reaches zero, the database connection is closed and removed from the map.
func ReleaseDBClient(cfg *Config) error {
	clientMu.Lock()
	defer clientMu.Unlock()

	dsn, err := cfg.buildDSN()
	if err != nil {
		return fmt.Errorf("failed to build DSN: %w", err)
	}

	client, ok := clientMap[dsn]
	if !ok {
		// Client not found, maybe already released or never created
		return nil
	}

	client.refCount--
	if client.refCount <= 0 {
		err = client.db.Close()
		delete(clientMap, dsn)
		if err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}
	}

	return nil
}
