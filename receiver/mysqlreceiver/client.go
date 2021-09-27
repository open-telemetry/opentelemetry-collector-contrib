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

package mysqlreceiver

import (
	"database/sql"
	"fmt"

	// registers the mysql driver
	_ "github.com/go-sql-driver/mysql"
)

type client interface {
	getGlobalStats() (map[string]string, error)
	getInnodbStats() (map[string]string, error)
	Close() error
}

type mySQLClient struct {
	client *sql.DB
}

var _ client = (*mySQLClient)(nil)

type mySQLConfig struct {
	username string
	password string
	database string
	endpoint string
}

func newMySQLClient(conf mySQLConfig) (*mySQLClient, error) {
	connStr := fmt.Sprintf("%s:%s@tcp(%s)/%s", conf.username, conf.password, conf.endpoint, conf.database)

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, err
	}

	return &mySQLClient{
		client: db,
	}, nil
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
	return c.client.Close()
}
