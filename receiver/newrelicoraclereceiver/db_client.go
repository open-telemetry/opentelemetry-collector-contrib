// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"database/sql"
	"strconv"

	"go.uber.org/zap"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/models"
)

type metricRow = models.MetricRow

type dbClient = models.DbClient

type clientProviderFunc = models.ClientProviderFunc

type dbSqlClient struct {
	db     *sql.DB
	sql    string
	logger *zap.Logger
}

func newDbClient(db *sql.DB, sql string, logger *zap.Logger) models.DbClient {
	return &dbSqlClient{
		db:     db,
		sql:    sql,
		logger: logger,
	}
}

func (c *dbSqlClient) MetricRows(ctx context.Context, args ...any) ([]models.MetricRow, error) {
	stmt, err := c.db.PrepareContext(ctx, c.sql)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metricRows []models.MetricRow

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		valuePtrs := make([]interface{}, len(columnNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err = rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(models.MetricRow)
		for i, colName := range columnNames {
			if values[i] == nil {
				row[colName] = ""
			} else {
				switch v := values[i].(type) {
				case string:
					row[colName] = v
				case []byte:
					row[colName] = string(v)
				case int64:
					row[colName] = strconv.FormatInt(v, 10)
				case float64:
					row[colName] = strconv.FormatFloat(v, 'f', -1, 64)
				default:
					row[colName] = ""
				}
			}
		}
		metricRows = append(metricRows, row)
	}

	return metricRows, rows.Err()
}
