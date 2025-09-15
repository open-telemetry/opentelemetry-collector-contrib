// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/azuread"
	"go.uber.org/zap"
)

// SQLConnection represents a wrapper around a SQL Server connection
type SQLConnection struct {
	Connection *sqlx.DB
	Host       string
	Config     *Config
	logger     *zap.Logger
}

// AuthConnector interface for different authentication methods
type AuthConnector interface {
	Connect(cfg *Config, dbName string) (*sqlx.DB, error)
}

// SQLAuthConnector handles standard SQL Server authentication
type SQLAuthConnector struct{}

func (s SQLAuthConnector) Connect(cfg *Config, dbName string) (*sqlx.DB, error) {
	connectionURL := cfg.CreateConnectionURL(dbName)
	return sqlx.Connect("mssql", connectionURL)
}

// AzureADAuthConnector handles Azure AD Service Principal authentication
type AzureADAuthConnector struct{}

func (a AzureADAuthConnector) Connect(cfg *Config, dbName string) (*sqlx.DB, error) {
	connectionURL := cfg.CreateAzureADConnectionURL(dbName)
	return sqlx.Connect(azuread.DriverName, connectionURL)
}

// NewConnection creates a new SQL Server connection with proper authentication
func NewConnection(cfg *Config) (*sql.DB, error) {
	ctx := context.Background()
	conn, err := NewSQLConnection(ctx, cfg, nil)
	if err != nil {
		return nil, err
	}
	return conn.Connection.DB, nil
}

// NewSQLConnection creates a new SQL Server connection with proper authentication
func NewSQLConnection(ctx context.Context, cfg *Config, logger *zap.Logger) (*SQLConnection, error) {
	return createConnectionWithAuth(ctx, cfg, "", logger)
}

// NewDatabaseConnection creates a connection to a specific database
func NewDatabaseConnection(ctx context.Context, cfg *Config, dbName string, logger *zap.Logger) (*SQLConnection, error) {
	return createConnectionWithAuth(ctx, cfg, dbName, logger)
}

// createConnectionWithAuth creates a connection using the appropriate authentication method
func createConnectionWithAuth(ctx context.Context, cfg *Config, dbName string, logger *zap.Logger) (*SQLConnection, error) {
	connector, err := determineAuthMethod(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to determine authentication method: %w", err)
	}

	db, err := connector.Connect(cfg, dbName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SQL Server: %w", err)
	}

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping SQL Server: %w", err)
	}

	return &SQLConnection{
		Connection: db,
		Host:       cfg.Hostname,
		Config:     cfg,
		logger:     logger,
	}, nil
}

// determineAuthMethod determines which authentication method to use based on configuration
func determineAuthMethod(cfg *Config, logger *zap.Logger) (AuthConnector, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	switch {
	case cfg.IsAzureADAuth():
		logger.Debug("Detected Azure AD Service Principal authentication - using ClientID, TenantID, and ClientSecret")
		return AzureADAuthConnector{}, nil
	default:
		logger.Debug("Using SQL Server authentication")
		return SQLAuthConnector{}, nil
	}
}

// Close closes the SQL connection
func (sc *SQLConnection) Close() {
	if err := sc.Connection.Close(); err != nil {
		sc.logger.Warn("Unable to close SQL Connection", zap.Error(err))
	}
}

// Query runs a query and loads results into v
func (sc *SQLConnection) Query(ctx context.Context, v interface{}, query string) error {
	sc.logger.Debug("Running query", zap.String("query", query))
	return sc.Connection.SelectContext(ctx, v, query)
}

// QueryRow runs a query that returns a single row
func (sc *SQLConnection) QueryRow(ctx context.Context, query string) *sql.Row {
	sc.logger.Debug("Running single row query", zap.String("query", query))
	return sc.Connection.QueryRowContext(ctx, query)
}

// Queryx runs a query and returns a set of rows
func (sc *SQLConnection) Queryx(ctx context.Context, query string) (*sqlx.Rows, error) {
	sc.logger.Debug("Running queryx", zap.String("query", query))
	return sc.Connection.QueryxContext(ctx, query)
}

// Ping tests the connection to the database
func (sc *SQLConnection) Ping(ctx context.Context) error {
	return sc.Connection.PingContext(ctx)
}

// Stats returns database connection statistics
func (sc *SQLConnection) Stats() sql.DBStats {
	return sc.Connection.Stats()
}
