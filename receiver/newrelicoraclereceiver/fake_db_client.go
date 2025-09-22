// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
)

// fakeDbClient is a mock database client for testing
type fakeDbClient struct {
	rows []metricRow
}

func newFakeDbClient(db *sql.DB, sql string, logger *zap.Logger) dbClient {
	// Return predefined test data for session count
	rows := []metricRow{
		{"SESSION_COUNT": "15"},
	}
	return &fakeDbClient{rows: rows}
}

func (c *fakeDbClient) metricRows(ctx context.Context, args ...any) ([]metricRow, error) {
	return c.rows, nil
}
