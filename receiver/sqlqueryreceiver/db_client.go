// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"context"

	// register db drivers
	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/snowflakedb/gosnowflake"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type stringMap map[string]string

type dbClient interface {
	metricRows(ctx context.Context) ([]stringMap, error)
}

type dbSQLClient struct {
	db     db
	logger *zap.Logger
	sql    string
}

func newDbClient(db db, sql string, logger *zap.Logger) dbClient {
	return dbSQLClient{
		db:     db,
		sql:    sql,
		logger: logger,
	}
}

func (cl dbSQLClient) metricRows(ctx context.Context) ([]stringMap, error) {
	sqlRows, err := cl.db.QueryContext(ctx, cl.sql)
	if err != nil {
		return nil, err
	}
	var out []stringMap
	colTypes, err := sqlRows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	scanner := newRowScanner(colTypes)
	var warnings error
	for sqlRows.Next() {
		err = scanner.scan(sqlRows)
		if err != nil {
			return nil, err
		}
		sm, scanErr := scanner.toStringMap()
		if scanErr != nil {
			warnings = multierr.Append(warnings, scanErr)
		}
		out = append(out, sm)
	}
	return out, warnings
}
