// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

// Data models for Oracle session count metric

// MetricRow represents a row of metric data from Oracle
type MetricRow map[string]string

// DbClient interface for database operations
type DbClient interface {
	MetricRows(ctx context.Context, args ...any) ([]MetricRow, error)
}

// ClientProviderFunc creates a new database client
type ClientProviderFunc func(*sql.DB, string, *zap.Logger) DbClient

// DbProviderFunc provides a database connection
type DbProviderFunc func() (*sql.DB, error)
