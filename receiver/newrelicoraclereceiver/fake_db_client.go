// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"database/sql"

	"go.uber.org/zap"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
)

// fakeDbClient is a mock database client for testing
type fakeDbClient struct {
	rows []models.MetricRow
}

func newFakeDbClient(db *sql.DB, sql string, logger *zap.Logger) models.DbClient {
	// Return predefined test data for session count
	rows := []models.MetricRow{
		{"SESSION_COUNT": "15"},
	}
	return &fakeDbClient{rows: rows}
}

func (c *fakeDbClient) MetricRows(ctx context.Context, args ...any) ([]models.MetricRow, error) {
	return c.rows, nil
}
