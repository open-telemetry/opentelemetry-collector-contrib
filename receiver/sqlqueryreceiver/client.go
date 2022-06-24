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
	"database/sql"
	"fmt"
	"strconv"
)

type client interface {
	Connect() error
	Close() error
	getQueries() ([]stat, error)
}

type sqlQueryClient struct {
	driver  string
	connStr string
	queries []Query
	client  *sql.DB
}

type stat struct {
	name        string
	value       float64
	isMonotonic bool
	dimensions  map[string]string
}

func (c *sqlQueryClient) Connect() error {

	clientDB, err := sql.Open(c.driver, c.connStr)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	c.client = clientDB
	return nil
}

func Scan(list *sql.Rows) (rows []map[string]interface{}) {
	fields, _ := list.Columns()
	for list.Next() {
		scans := make([]interface{}, len(fields))
		row := make(map[string]interface{})

		for i := range scans {
			scans[i] = &scans[i]
		}
		list.Scan(scans...)
		for i, v := range scans {
			var value = ""
			if v != nil {
				value = fmt.Sprintf("%s", v)
			}
			row[fields[i]] = value
		}
		rows = append(rows, row)
	}
	return
}

func (c *sqlQueryClient) getQueries() (stats []stat, err error) {

	// iterate through config queries
	for _, q := range c.queries {
		resp, err := c.client.Query(q.SQL)

		if err != nil {
			panic(err)
		}
		// returns map of values
		rows := Scan(resp)
		for _, row := range rows {
			for _, m := range q.Metrics {
				s, _ := strconv.ParseFloat(row[m.ValueColumn].(string), 32)
				stat := stat{
					name:        m.MetricName,
					value:       s,
					isMonotonic: m.IsMonotonic,
					dimensions:  m.Tags,
				}

				for _, att := range m.AttributeColumns {
					v, pres := row[att]
					if pres {
						stat.dimensions[att] = v.(string)
					}
				}

				stats = append(stats, stat)
			}
		}
	}
	return
}

func (c *sqlQueryClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
