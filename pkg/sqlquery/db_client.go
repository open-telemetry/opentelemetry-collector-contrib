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

package sqlquery // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sqlquery"

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	// register db drivers
	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/snowflakedb/gosnowflake"
	"go.uber.org/zap"
)

type DbClient interface {
	MetricRows(ctx context.Context) ([]MetricRow, error)
}

type MetricRow map[string]string

type dbSQLClient struct {
	db     *sql.DB
	logger *zap.Logger
	sql    string
}

func NewDbClient(db *sql.DB, sql string, logger *zap.Logger) DbClient {
	return dbSQLClient{
		db:     db,
		sql:    sql,
		logger: logger,
	}
}

func (cl dbSQLClient) MetricRows(ctx context.Context) ([]MetricRow, error) {
	sqlRows, err := cl.db.QueryContext(ctx, cl.sql)
	if err != nil {
		return nil, err
	}
	var out []MetricRow
	row := reusableRow{
		attrs: map[string]func() string{},
	}
	types, err := sqlRows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	for _, sqlType := range types {
		colName := sqlType.Name()
		var v interface{}
		row.attrs[colName] = func() string {
			format := "%v"
			if reflect.TypeOf(v).Kind() == reflect.Slice {
				// The Postgres driver returns a []uint8 (a string) for decimal and numeric types,
				// which we want to render as strings. e.g. "4.1" instead of "[52, 46, 49]".
				// Other slice types get the same treatment.
				format = "%s"
			}
			return fmt.Sprintf(format, v)
		}
		row.scanDest = append(row.scanDest, &v)
	}
	for sqlRows.Next() {
		err = sqlRows.Scan(row.scanDest...)
		if err != nil {
			return nil, err
		}
		out = append(out, row.toMetricRow())
	}
	return out, nil
}

type reusableRow struct {
	attrs    map[string]func() string
	scanDest []interface{}
}

func (row reusableRow) toMetricRow() MetricRow {
	out := MetricRow{}
	for k, f := range row.attrs {
		out[k] = f()
	}
	return out
}
