// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"

import (
	"context"
	"errors"

	// Do not register any Db drivers here: users should register the ones that are applicable to them.
	"go.uber.org/zap"
)

type StringMap map[string]string

type DbClient interface {
	QueryRows(ctx context.Context, args ...any) ([]StringMap, error)
}

type DbSQLClient struct {
	Db        Db
	Logger    *zap.Logger
	Telemetry TelemetryConfig
	SQL       string
}

func NewDbClient(db Db, sql string, logger *zap.Logger, telemetry TelemetryConfig) DbClient {
	return DbSQLClient{
		Db:        db,
		SQL:       sql,
		Logger:    logger,
		Telemetry: telemetry,
	}
}

func (cl DbSQLClient) QueryRows(ctx context.Context, args ...any) ([]StringMap, error) {
	cl.Logger.Debug("Running query", cl.prepareQueryFields(cl.SQL, args)...)
	sqlRows, err := cl.Db.QueryContext(ctx, cl.SQL, args...)
	if err != nil {
		return nil, err
	}
	var out []StringMap
	colTypes, err := sqlRows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	scanner := newRowScanner(colTypes)
	var warnings []error
	for sqlRows.Next() {
		err = scanner.scan(sqlRows)
		if err != nil {
			return nil, err
		}
		sm, scanErr := scanner.toStringMap()
		if scanErr != nil {
			warnings = append(warnings, scanErr)
		}
		out = append(out, sm)
	}
	return out, errors.Join(warnings...)
}

func (cl DbSQLClient) prepareQueryFields(sql string, args []any) []zap.Field {
	var logFields []zap.Field
	if cl.Telemetry.Logs.Query {
		logFields = append(logFields, zap.String("query", sql))
		logFields = append(logFields, zap.Any("parameters", args))
	}
	return logFields
}

// This is only used for testing, but need to be exposed to other packages.
type FakeDBClient struct {
	RequestCounter int
	StringMaps     [][]StringMap
	Err            error
}

func (c *FakeDBClient) QueryRows(context.Context, ...any) ([]StringMap, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	idx := c.RequestCounter
	c.RequestCounter++
	return c.StringMaps[idx], nil
}
