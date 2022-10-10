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
	"errors"
	"fmt"
	"strings"

	// registers the mysql driver
	"github.com/go-sql-driver/mysql"
)

type client interface {
	Connect() error
	getGlobalStats() (map[string]string, error)
	getInnodbStats() (map[string]string, error)
	getTableIoWaitsStats() ([]TableIoWaitsStats, error)
	getIndexIoWaitsStats() ([]IndexIoWaitsStats, error)
	getReplicaStatusStats() ([]ReplicaStatusStats, error)
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

type ReplicaStatusStats struct {
	replicaIOState            string
	sourceHost                string
	sourceUser                string
	sourcePort                int64
	connectRetry              int64
	sourceLogFile             string
	readSourceLogPos          int64
	relayLogFile              string
	relayLogPos               int64
	relaySourceLogFile        string
	replicaIORunning          string
	replicaSQLRunning         string
	replicateDoDB             string
	replicateIgnoreDB         string
	replicateDoTable          string
	replicateIgnoreTable      string
	replicateWildDoTable      string
	replicateWildIgnoreTable  string
	lastErrno                 int64
	lastError                 string
	skipCounter               int64
	execSourceLogPos          int64
	relayLogSpace             int64
	untilCondition            string
	untilLogFile              string
	untilLogPos               string
	sourceSSLAllowed          string
	sourceSSLCAFile           string
	sourceSSLCAPath           string
	sourceSSLCert             string
	sourceSSLCipher           string
	sourceSSLKey              string
	secondsBehindSource       sql.NullInt64
	sourceSSLVerifyServerCert string
	lastIOErrno               int64
	lastIOError               string
	lastSQLErrno              int64
	lastSQLError              string
	replicateIgnoreServerIds  string
	sourceServerID            int64
	sourceUUID                string
	sourceInfoFile            string
	sqlDelay                  int64
	sqlRemainingDelay         sql.NullInt64
	replicaSQLRunningState    string
	sourceRetryCount          int64
	sourceBind                string
	lastIOErrorTimestamp      string
	lastSQLErrorTimestamp     string
	sourceSSLCrl              string
	sourceSSLCrlpath          string
	retrievedGtidSet          string
	executedGtidSet           string
	autoPosition              string
	replicateRewriteDB        string
	channelName               string
	sourceTLSVersion          string
	sourcePublicKeyPath       string
	getSourcePublicKey        int64
	networkNamespace          string
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

func (c *mySQLClient) getReplicaStatusStats() ([]ReplicaStatusStats, error) {
	query := "SHOW REPLICA STATUS"
	var me *mysql.MySQLError
	rows, err := c.client.Query(query)
	if err != nil {
		if !errors.As(err, &me) {
			return nil, err
		}

		if me.Number != 1064 {
			return nil, err
		}

		// fallback to deprecated command for older versions
		query = "SHOW SLAVE STATUS"
		rows, err = c.client.Query(query)
		if err != nil {
			return nil, err
		}
	}

	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var stats []ReplicaStatusStats
	for rows.Next() {
		var s ReplicaStatusStats
		dest := []interface{}{}
		for _, col := range cols {
			switch strings.ToLower(col) {
			case "replica_io_state":
				fallthrough
			case "slave_io_state":
				dest = append(dest, &s.replicaIOState)
			case "source_host":
				fallthrough
			case "master_host":
				dest = append(dest, &s.sourceHost)
			case "source_user":
				fallthrough
			case "master_user":
				dest = append(dest, &s.sourceUser)
			case "source_port":
				fallthrough
			case "master_port":
				dest = append(dest, &s.sourcePort)
			case "connect_retry":
				dest = append(dest, &s.connectRetry)
			case "source_log_file":
				fallthrough
			case "master_log_file":
				dest = append(dest, &s.sourceLogFile)
			case "read_source_log_pos":
				fallthrough
			case "read_master_log_pos":
				dest = append(dest, &s.readSourceLogPos)
			case "relay_log_file":
				dest = append(dest, &s.relayLogFile)
			case "relay_log_pos":
				dest = append(dest, &s.relayLogPos)
			case "relay_source_log_file":
				fallthrough
			case "relay_master_log_file":
				dest = append(dest, &s.relaySourceLogFile)
			case "replica_io_running":
				fallthrough
			case "slave_io_running":
				dest = append(dest, &s.replicaIORunning)
			case "replica_sql_running":
				fallthrough
			case "slave_sql_running":
				dest = append(dest, &s.replicaSQLRunning)
			case "replicate_do_db":
				dest = append(dest, &s.replicateDoDB)
			case "replicate_ignore_db":
				dest = append(dest, &s.replicateIgnoreDB)
			case "replicate_do_table":
				dest = append(dest, &s.replicateDoTable)
			case "replicate_ignore_table":
				dest = append(dest, &s.replicateIgnoreTable)
			case "replicate_wild_do_table":
				dest = append(dest, &s.replicateWildDoTable)
			case "replicate_wild_ignore_table":
				dest = append(dest, &s.replicateWildIgnoreTable)
			case "last_errno":
				dest = append(dest, &s.lastErrno)
			case "last_error":
				dest = append(dest, &s.lastError)
			case "skip_counter":
				dest = append(dest, &s.skipCounter)
			case "exec_source_log_pos":
				fallthrough
			case "exec_master_log_pos":
				dest = append(dest, &s.execSourceLogPos)
			case "relay_log_space":
				dest = append(dest, &s.relayLogSpace)
			case "until_condition":
				dest = append(dest, &s.untilCondition)
			case "until_log_file":
				dest = append(dest, &s.untilLogFile)
			case "until_log_pos":
				dest = append(dest, &s.untilLogPos)
			case "source_ssl_allowed":
				fallthrough
			case "master_ssl_allowed":
				dest = append(dest, &s.sourceSSLAllowed)
			case "source_ssl_ca_file":
				fallthrough
			case "master_ssl_ca_file":
				dest = append(dest, &s.sourceSSLCAFile)
			case "source_ssl_ca_path":
				fallthrough
			case "master_ssl_ca_path":
				dest = append(dest, &s.sourceSSLCAPath)
			case "source_ssl_cert":
				fallthrough
			case "master_ssl_cert":
				dest = append(dest, &s.sourceSSLCert)
			case "source_ssl_cipher":
				fallthrough
			case "master_ssl_cipher":
				dest = append(dest, &s.sourceSSLCipher)
			case "source_ssl_key":
				fallthrough
			case "master_ssl_key":
				dest = append(dest, &s.sourceSSLKey)
			case "seconds_behind_source":
				fallthrough
			case "seconds_behind_master":
				dest = append(dest, &s.secondsBehindSource)
			case "source_ssl_verify_server_cert":
				fallthrough
			case "master_ssl_verify_server_cert":
				dest = append(dest, &s.sourceSSLVerifyServerCert)
			case "last_io_errno":
				dest = append(dest, &s.lastIOErrno)
			case "last_io_error":
				dest = append(dest, &s.lastIOError)
			case "last_sql_errno":
				dest = append(dest, &s.lastSQLErrno)
			case "last_sql_error":
				dest = append(dest, &s.lastSQLError)
			case "replicate_ignore_server_ids":
				dest = append(dest, &s.replicateIgnoreServerIds)
			case "source_server_id":
				fallthrough
			case "master_server_id":
				dest = append(dest, &s.sourceServerID)
			case "source_uuid":
				fallthrough
			case "master_uuid":
				dest = append(dest, &s.sourceUUID)
			case "source_info_file":
				fallthrough
			case "master_info_file":
				dest = append(dest, &s.sourceInfoFile)
			case "sql_delay":
				dest = append(dest, &s.sqlDelay)
			case "sql_remaining_delay":
				dest = append(dest, &s.sqlRemainingDelay)
			case "replica_sql_running_state":
				fallthrough
			case "slave_sql_running_state":
				dest = append(dest, &s.replicaSQLRunningState)
			case "source_retry_count":
				fallthrough
			case "master_retry_count":
				dest = append(dest, &s.sourceRetryCount)
			case "source_bind":
				fallthrough
			case "master_bind":
				dest = append(dest, &s.sourceBind)
			case "last_io_error_timestamp":
				dest = append(dest, &s.lastIOErrorTimestamp)
			case "last_sql_error_timestamp":
				dest = append(dest, &s.lastSQLErrorTimestamp)
			case "source_ssl_crl":
				fallthrough
			case "master_ssl_crl":
				dest = append(dest, &s.sourceSSLCrl)
			case "source_ssl_crlpath":
				fallthrough
			case "master_ssl_crlpath":
				dest = append(dest, &s.sourceSSLCrlpath)
			case "retrieved_gtid_set":
				dest = append(dest, &s.retrievedGtidSet)
			case "executed_gtid_set":
				dest = append(dest, &s.executedGtidSet)
			case "auto_position":
				dest = append(dest, &s.autoPosition)
			case "replicate_rewrite_db":
				dest = append(dest, &s.replicateRewriteDB)
			case "channel_name":
				dest = append(dest, &s.channelName)
			case "source_tls_version":
				fallthrough
			case "master_tls_version":
				dest = append(dest, &s.sourceTLSVersion)
			case "source_public_key_path":
				dest = append(dest, &s.sourcePublicKeyPath)
			case "get_source_public_key":
				dest = append(dest, &s.getSourcePublicKey)
			case "network_namespace":
				dest = append(dest, &s.networkNamespace)
			default:
				return nil, fmt.Errorf("unknown column name %s for replica status", col)
			}
		}
		err := rows.Scan(dest...)

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
