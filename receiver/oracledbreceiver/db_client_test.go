// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMetricRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("SELECT .*").
		WillReturnRows(sqlmock.NewRows([]string{"COL1", "COL2"}).
			AddRow("val1", "val2")).
		RowsWillBeClosed()

	client := newDbClient(db, "SELECT * FROM dual", zap.NewNop())
	rows, err := client.metricRows(context.Background())
	require.NoError(t, err)

	expected := []metricRow{
		{
			"COL1": "val1",
			"COL2": "val2",
		},
	}
	assert.Equal(t, expected, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
