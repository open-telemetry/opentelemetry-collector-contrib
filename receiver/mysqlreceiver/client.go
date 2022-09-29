// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"database/sql"
	"fmt"

	// registers the mysql driver
	"github.com/go-sql-driver/mysql"
)

type client interface {
	Connect() error
	getGlobalStats() (map[string]string, error)
	getInnodbStats() (map[string]string, error)
	getTableIoWaitsStats() ([]TableIoWaitsStats, error)
	getIndexIoWaitsStats() ([]IndexIoWaitsStats, error)
	Close() error
}

type mySQLClient struct {
	connStr string
	client  *sql.DB
}

type IoWaitsStats struct {
	schema      string
	name        string
	countDelete int64
	countFetch  int64
	countInsert int64
	countUpdate int64
	timeDelete  int64
	timeFetch   int64
	timeInsert  int64
	timeUpdate  int64
}

type TableIoWaitsStats struct {
	IoWaitsStats
}

type IndexIoWaitsStats struct {
	IoWaitsStats
	index string
}

var _ client = (*mySQLClient)(nil)

func newMySQLClient(conf *Config) client {
	driverConf := mysql.Config{
		User:                 conf.Username,
		Passwd:               conf.Password,
		Net:                  conf.Transport,
		Addr:                 conf.Endpoint,
		DBName:               conf.Database,
		AllowNativePasswords: conf.AllowNativePasswords,
	}
	connStr := driverConf.FormatDSN()

	return &mySQLClient{
		connStr: connStr,
	}
}

func (c *mySQLClient) Connect() error {
	clientDB, err := sql.Open("mysql", c.connStr)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	c.client = clientDB
	return nil
}

// getGlobalStats queries the db for global status metrics.
func (c *mySQLClient) getGlobalStats() (map[string]string, error) {
	query := "SHOW GLOBAL STATUS;"
	return Query(*c, query)
}

// getInnodbStats queries the db for innodb metrics.
func (c *mySQLClient) getInnodbStats() (map[string]string, error) {
	query := "SELECT name, count FROM information_schema.innodb_metrics WHERE name LIKE '%buffer_pool_size%';"
	return Query(*c, query)
}

// getTableIoWaitsStats queries the db for table_io_waits metrics.
func (c *mySQLClient) getTableIoWaitsStats() ([]TableIoWaitsStats, error) {
	query := "SELECT OBJECT_SCHEMA, OBJECT_NAME, " +
		"COUNT_DELETE, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE," +
		"SUM_TIMER_DELETE, SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE" +
		"FROM performance_schema.table_io_waits_summary_by_table" +
		"WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema');"
	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []TableIoWaitsStats
	for rows.Next() {
		var s TableIoWaitsStats
		err := rows.Scan(&s.schema, &s.name,
			&s.countDelete, &s.countFetch, &s.countInsert, &s.countUpdate,
			&s.timeDelete, &s.timeFetch, &s.timeInsert, &s.timeUpdate)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// getIndexIoWaitsStats queries the db for index_io_waits metrics.
func (c *mySQLClient) getIndexIoWaitsStats() ([]IndexIoWaitsStats, error) {
	query := "SELECT OBJECT_SCHEMA, OBJECT_NAME, ifnull(INDEX_NAME, 'NONE') as INDEX_NAME," +
		"COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE, COUNT_DELETE," +
		"SUM_TIMER_FETCH, SUM_TIMER_INSERT, SUM_TIMER_UPDATE, SUM_TIMER_DELETE" +
		"FROM performance_schema.table_io_waits_summary_by_index_usage" +
		"WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema');"

	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []IndexIoWaitsStats
	for rows.Next() {
		var s IndexIoWaitsStats
		err := rows.Scan(&s.schema, &s.name, &s.index,
			&s.countDelete, &s.countFetch, &s.countInsert, &s.countUpdate,
			&s.timeDelete, &s.timeFetch, &s.timeInsert, &s.timeUpdate)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

func Query(c mySQLClient, query string) (map[string]string, error) {
	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := map[string]string{}
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return nil, err
		}
		stats[key] = val
	}

	return stats, nil
}

func (c *mySQLClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
