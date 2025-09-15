// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoracledbreceiver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"

	go_ora "github.com/sijms/go-ora/v2"
	"go.uber.org/zap"
)

// dbClient interface for database operations
type dbClient interface {
	metricRows(ctx context.Context, query string, args ...any) ([]metricRow, error)
	getConnection() (*sql.DB, error)
	close() error
}

type metricRow map[string]string

// oracleDBClient implements the dbClient interface for Oracle database connections
type oracleDBClient struct {
	logger     *zap.Logger
	dataSource string
	db         *sql.DB
}

// newDBClient creates a new Oracle database client
func newDBClient(cfg *Config, logger *zap.Logger) (dbClient, error) {
	dataSource := getDataSource(*cfg)
	if dataSource == "" {
		return nil, errors.New("empty data source")
	}

	client := &oracleDBClient{
		logger:     logger,
		dataSource: dataSource,
	}

	return client, nil
}

// getConnection returns a database connection
func (c *oracleDBClient) getConnection() (*sql.DB, error) {
	if c.db != nil {
		// Test the existing connection
		if err := c.db.Ping(); err == nil {
			return c.db, nil
		}
		// Close the bad connection
		c.db.Close()
		c.db = nil
	}

	// Create new connection
	db, err := sql.Open("oracle", c.dataSource)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	c.db = db
	c.logger.Info("Successfully connected to Oracle database")
	return c.db, nil
}

// close closes the database connection
func (c *oracleDBClient) close() error {
	if c.db != nil {
		err := c.db.Close()
		c.db = nil
		return err
	}
	return nil
}

// metricRows executes a query and returns the results as metric rows
func (c *oracleDBClient) metricRows(ctx context.Context, query string, args ...any) ([]metricRow, error) {
	db, err := c.getConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []metricRow
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(metricRow)
		for i, column := range columns {
			val := values[i]
			if val != nil {
				row[column] = fmt.Sprintf("%v", val)
			} else {
				row[column] = ""
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// getDataSource builds the data source string from configuration
func getDataSource(cfg Config) string {
	if cfg.DataSource != "" {
		return cfg.DataSource
	}

	// Build data source from individual components
	if cfg.Endpoint == "" && cfg.Service == "" {
		return ""
	}

	var dataSource string
	if cfg.Endpoint != "" {
		// Parse endpoint to get host and port
		if u, err := url.Parse("oracle://" + cfg.Endpoint); err == nil {
			dataSource = go_ora.BuildUrl(u.Hostname(), getPort(u.Port()), cfg.Service, cfg.Username, cfg.Password, nil)
		}
	}

	return dataSource
}

// getPort returns the port number, defaulting to 1521 if not specified
func getPort(portStr string) int {
	if portStr == "" {
		return 1521
	}

	// Parse port string to int
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 1521
	}

	return port
}
