// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"go.uber.org/zap"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	// registers the mysql driver
	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
)

type client interface {
	Connect() error
	checkPerformanceCollectionSettings()
	getVersion() (*version.Version, error)
	getGlobalStats() (map[string]string, error)
	getInnodbStats() (map[string]string, error)
	getTableStats() ([]TableStats, error)
	getTableIoWaitsStats() ([]TableIoWaitsStats, error)
	getIndexIoWaitsStats() ([]IndexIoWaitsStats, error)
	getStatementEventsStats() ([]StatementEventStats, error)
	getTableLockWaitEventStats() ([]tableLockWaitEventStats, error)
	getReplicaStatusStats() ([]ReplicaStatusStats, error)
	getQueryStats(since int64, topCount int) ([]QueryStats, error)
	getExplainPlanAsJsonForDigestQuery(digest string) (string, error)
	Close() error
}

type mySQLClient struct {
	connStr                        string
	client                         *sql.DB
	statementEventsDigestTextLimit int
	statementEventsLimit           int
	statementEventsTimeLimit       time.Duration
	logger                         *zap.Logger
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

type TableStats struct {
	schema           string
	name             string
	rows             int64
	averageRowLength int64
	dataLength       int64
	indexLength      int64
}

type StatementEventStats struct {
	schema                    string
	digest                    string
	digestText                string
	sumTimerWait              int64
	countErrors               int64
	countWarnings             int64
	countRowsAffected         int64
	countRowsSent             int64
	countRowsExamined         int64
	countCreatedTmpDiskTables int64
	countCreatedTmpTables     int64
	countSortMergePasses      int64
	countSortRows             int64
	countNoIndexUsed          int64
}

type tableLockWaitEventStats struct {
	schema                        string
	name                          string
	countReadNormal               int64
	countReadWithSharedLocks      int64
	countReadHighPriority         int64
	countReadNoInsert             int64
	countReadExternal             int64
	countWriteAllowWrite          int64
	countWriteConcurrentInsert    int64
	countWriteLowPriority         int64
	countWriteNormal              int64
	countWriteExternal            int64
	sumTimerReadNormal            int64
	sumTimerReadWithSharedLocks   int64
	sumTimerReadHighPriority      int64
	sumTimerReadNoInsert          int64
	sumTimerReadExternal          int64
	sumTimerWriteAllowWrite       int64
	sumTimerWriteConcurrentInsert int64
	sumTimerWriteLowPriority      int64
	sumTimerWriteNormal           int64
	sumTimerWriteExternal         int64
}

type ReplicaStatusStats struct {
	replicaIOState              string
	sourceHost                  string
	sourceUser                  string
	sourcePort                  int64
	connectRetry                int64
	sourceLogFile               string
	readSourceLogPos            int64
	relayLogFile                string
	relayLogPos                 int64
	relaySourceLogFile          string
	replicaIORunning            string
	replicaSQLRunning           string
	replicateDoDB               string
	replicateIgnoreDB           string
	replicateDoTable            string
	replicateIgnoreTable        string
	replicateWildDoTable        string
	replicateWildIgnoreTable    string
	lastErrno                   int64
	lastError                   string
	skipCounter                 int64
	execSourceLogPos            int64
	relayLogSpace               int64
	untilCondition              string
	untilLogFile                string
	untilLogPos                 string
	sourceSSLAllowed            string
	sourceSSLCAFile             string
	sourceSSLCAPath             string
	sourceSSLCert               string
	sourceSSLCipher             string
	sourceSSLKey                string
	secondsBehindSource         sql.NullInt64
	sourceSSLVerifyServerCert   string
	lastIOErrno                 int64
	lastIOError                 string
	lastSQLErrno                int64
	lastSQLError                string
	replicateIgnoreServerIDs    string
	sourceServerID              int64
	sourceUUID                  string
	sourceInfoFile              string
	sqlDelay                    int64
	sqlRemainingDelay           sql.NullInt64
	replicaSQLRunningState      string
	sourceRetryCount            int64
	sourceBind                  string
	lastIOErrorTimestamp        string
	lastSQLErrorTimestamp       string
	sourceSSLCrl                string
	sourceSSLCrlpath            string
	retrievedGtidSet            string
	executedGtidSet             string
	autoPosition                string
	replicateRewriteDB          string
	channelName                 string
	sourceTLSVersion            string
	sourcePublicKeyPath         string
	getSourcePublicKey          int64
	networkNamespace            string
	usingGtid                   string
	gtidIoPos                   string
	slaveDdlGroups              int64
	slaveNonTransactionalGroups int64
	slaveTransactionalGroups    int64
	retriedTransactions         int64
	maxRelayLogSize             int64
	executedLogEntries          int64
	slaveReceivedHeartbeats     int64
	slaveHeartbeatPeriod        int64
	gtidSlavePos                string
	masterLastEventTime         string
	slaveLastEventTime          string
	masterSlaveTimeDiff         string
	parallelMode                string
	replicateDoDomainIDs        string
	replicateIgnoreDomainIDs    string
}

type QueryStats struct {
	queryText          string  // The MySQL normalized query text
	queryDigest        string  // The MySQL query digest against the normalized query text
	schema             string  // schemas the query was run against
	count              int64   // The number of times the query was run since the provided timestamp
	lockTime           float64 // The total lock time for the query since the provided timestamp
	cpuTime            float64 // The total CPU time for the query since the provided timestamp
	rowsExamined       int64   // The total number of rows examined for the query since the provided timestamp
	rowsReturned       int64   // The total number of rows returned for the query since the provided timestamp
	totalDuration      float64 // The total duration of all calls to this query since the provided timestamp
	totalWait          int64   // The total wait time for all calls to this query since the database last started
	rowsAffected       int64   // The total number of rows affected for the query since the provided timestamp
	fullJoins          int64   // The total number of full joins for the query since the provided timestamp
	fullRangeJoins     int64   // The total number of full range joins for the query since the provided timestamp
	selectRanges       int64   // The total number of select ranges for the query since the provided timestamp
	selectRangesChecks int64   // The total number of select range checks for the query since the provided timestamp
	selectScans        int64   // The total number of select scans for the query since the provided timestamp
	sortMergePasses    int64   // The total number of sort merge passes for the query since the provided timestamp
	sortRanges         int64   // The total number of sort ranges for the query since the provided timestamp
	sortRows           int64   // The total number of sort rows for the query since the provided timestamp
	sortScans          int64   // The total number of sort scans for the query since the provided timestamp
	noIndexUsed        int64   // The total number of times no index was used for the query since the provided timestamp
	noGoodIndexUsed    int64   // The total number of times no good index was used for the query since the provided timestamp
	users              string  // The users that ran the query since the provided timestamp
	hosts              string  // The hosts that ran the query since the provided timestamp
	dbs                string  // The databases that ran the query since the provided timestamp
	querySample        string  // A sample of a literal query text for use in an EXPLAIN call

	diffTime int64 // only used during sort, not in scan
}

var _ client = (*mySQLClient)(nil)
var stringArrayRegex = regexp.MustCompile(`[\s"\[\]]`)

func newMySQLClient(conf *Config, logger *zap.Logger) (client, error) {
	tls, err := conf.TLS.LoadTLSConfig(context.Background())
	if err != nil {
		return nil, err
	}
	tlsConfig := ""
	if tls != nil {
		err := mysql.RegisterTLSConfig("custom", tls)
		if err != nil {
			return nil, err
		}
		tlsConfig = "custom"
	}

	driverConf := mysql.Config{
		User:                 conf.Username,
		Passwd:               string(conf.Password),
		Net:                  string(conf.Transport),
		Addr:                 conf.Endpoint,
		DBName:               conf.Database,
		AllowNativePasswords: conf.AllowNativePasswords,
		TLS:                  tls,
		TLSConfig:            tlsConfig,
	}
	connStr := driverConf.FormatDSN()

	return &mySQLClient{
		connStr:                        connStr,
		statementEventsDigestTextLimit: conf.StatementEvents.DigestTextLimit,
		statementEventsLimit:           conf.StatementEvents.Limit,
		statementEventsTimeLimit:       conf.StatementEvents.TimeLimit,
		logger:                         logger,
	}, nil
}

func (c *mySQLClient) Connect() error {
	clientDB, err := sql.Open("mysql", c.connStr)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	c.client = clientDB
	return nil
}

// getVersion queries the db for the version.
func (c *mySQLClient) getVersion() (*version.Version, error) {
	query := "SELECT VERSION();"
	var versionStr string
	err := c.client.QueryRow(query).Scan(&versionStr)
	if err != nil {
		return nil, err
	}
	vrsn, err := version.NewVersion(versionStr)
	return vrsn, err
}

// getGlobalStats queries the db for global status metrics.
func (c *mySQLClient) getGlobalStats() (map[string]string, error) {
	q := "SHOW GLOBAL STATUS;"
	return query(*c, q)
}

// getInnodbStats queries the db for innodb metrics.
func (c *mySQLClient) getInnodbStats() (map[string]string, error) {
	q := "SELECT name, count FROM information_schema.innodb_metrics WHERE name LIKE '%buffer_pool_size%';"
	return query(*c, q)
}

// getTableStats queries the db for information_schema table size metrics.
func (c *mySQLClient) getTableStats() ([]TableStats, error) {
	query := "SELECT TABLE_SCHEMA, TABLE_NAME, " +
		"COALESCE(TABLE_ROWS, 0) as TABLE_ROWS, " +
		"COALESCE(AVG_ROW_LENGTH, 0) as AVG_ROW_LENGTH, " +
		"COALESCE(DATA_LENGTH, 0) as DATA_LENGTH, " +
		"COALESCE(INDEX_LENGTH, 0) as  INDEX_LENGTH " +
		"FROM information_schema.TABLES " +
		"WHERE TABLE_SCHEMA NOT in ('information_schema', 'sys');"
	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []TableStats
	for rows.Next() {
		var s TableStats
		err := rows.Scan(&s.schema, &s.name,
			&s.rows, &s.averageRowLength,
			&s.dataLength, &s.indexLength)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

// getTableIoWaitsStats queries the db for table_io_waits metrics.
func (c *mySQLClient) getTableIoWaitsStats() ([]TableIoWaitsStats, error) {
	query := "SELECT OBJECT_SCHEMA, OBJECT_NAME, " +
		"COUNT_DELETE, COUNT_FETCH, COUNT_INSERT, COUNT_UPDATE," +
		"FLOOR(SUM_TIMER_DELETE/1000), FLOOR(SUM_TIMER_FETCH/1000), FLOOR(SUM_TIMER_INSERT/1000), FLOOR(SUM_TIMER_UPDATE/1000) " +
		"FROM performance_schema.table_io_waits_summary_by_table " +
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
		"FLOOR(SUM_TIMER_FETCH/1000), FLOOR(SUM_TIMER_INSERT/1000), FLOOR(SUM_TIMER_UPDATE/1000), FLOOR(SUM_TIMER_DELETE/1000) " +
		"FROM performance_schema.table_io_waits_summary_by_index_usage " +
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

func (c *mySQLClient) getStatementEventsStats() ([]StatementEventStats, error) {
	query := fmt.Sprintf("SELECT ifnull(SCHEMA_NAME, 'NONE') as SCHEMA_NAME, DIGEST,"+
		"LEFT(DIGEST_TEXT, %d) as DIGEST_TEXT, FLOOR(SUM_TIMER_WAIT/1000), SUM_ERRORS,"+
		"SUM_WARNINGS, SUM_ROWS_AFFECTED, SUM_ROWS_SENT, SUM_ROWS_EXAMINED,"+
		"SUM_CREATED_TMP_DISK_TABLES, SUM_CREATED_TMP_TABLES, SUM_SORT_MERGE_PASSES,"+
		"SUM_SORT_ROWS, SUM_NO_INDEX_USED "+
		"FROM performance_schema.events_statements_summary_by_digest "+
		"WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema') "+
		"AND last_seen > DATE_SUB(NOW(), INTERVAL %d SECOND) "+
		"ORDER BY SUM_TIMER_WAIT DESC "+
		"LIMIT %d",
		c.statementEventsDigestTextLimit,
		int64(c.statementEventsTimeLimit.Seconds()),
		c.statementEventsLimit)

	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []StatementEventStats
	for rows.Next() {
		var s StatementEventStats
		err := rows.Scan(&s.schema, &s.digest, &s.digestText,
			&s.sumTimerWait, &s.countErrors, &s.countWarnings,
			&s.countRowsAffected, &s.countRowsSent, &s.countRowsExamined, &s.countCreatedTmpDiskTables,
			&s.countCreatedTmpTables, &s.countSortMergePasses, &s.countSortRows, &s.countNoIndexUsed)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

func (c *mySQLClient) getTableLockWaitEventStats() ([]tableLockWaitEventStats, error) {
	query := "SELECT OBJECT_SCHEMA, OBJECT_NAME, COUNT_READ_NORMAL, COUNT_READ_WITH_SHARED_LOCKS," +
		"COUNT_READ_HIGH_PRIORITY, COUNT_READ_NO_INSERT, COUNT_READ_EXTERNAL, COUNT_WRITE_ALLOW_WRITE," +
		"COUNT_WRITE_CONCURRENT_INSERT, COUNT_WRITE_LOW_PRIORITY, COUNT_WRITE_NORMAL," +
		"COUNT_WRITE_EXTERNAL, FLOOR(SUM_TIMER_READ_NORMAL/1000), FLOOR(SUM_TIMER_READ_WITH_SHARED_LOCKS/1000)," +
		"FLOOR(SUM_TIMER_READ_HIGH_PRIORITY/1000), FLOOR(SUM_TIMER_READ_NO_INSERT/1000), FLOOR(SUM_TIMER_READ_EXTERNAL/1000)," +
		"FLOOR(SUM_TIMER_WRITE_ALLOW_WRITE/1000), FLOOR(SUM_TIMER_WRITE_CONCURRENT_INSERT/1000), FLOOR(SUM_TIMER_WRITE_LOW_PRIORITY/1000)," +
		"FLOOR(SUM_TIMER_WRITE_NORMAL/1000), FLOOR(SUM_TIMER_WRITE_EXTERNAL/1000) " +
		"FROM performance_schema.table_lock_waits_summary_by_table " +
		"WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema')"

	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []tableLockWaitEventStats
	for rows.Next() {
		var s tableLockWaitEventStats
		err := rows.Scan(&s.schema, &s.name,
			&s.countReadNormal, &s.countReadWithSharedLocks, &s.countReadHighPriority, &s.countReadNoInsert, &s.countReadExternal,
			&s.countWriteAllowWrite, &s.countWriteConcurrentInsert, &s.countWriteLowPriority, &s.countWriteNormal, &s.countWriteExternal,
			&s.sumTimerReadNormal, &s.sumTimerReadWithSharedLocks, &s.sumTimerReadHighPriority, &s.sumTimerReadNoInsert, &s.sumTimerReadExternal,
			&s.sumTimerWriteAllowWrite, &s.sumTimerWriteConcurrentInsert, &s.sumTimerWriteLowPriority, &s.sumTimerWriteNormal, &s.sumTimerWriteExternal)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}

func (c *mySQLClient) getReplicaStatusStats() ([]ReplicaStatusStats, error) {
	mysqlVersion, err := c.getVersion()
	if err != nil {
		return nil, err
	}

	query := "SHOW REPLICA STATUS"
	minMysqlVersion, _ := version.NewVersion("8.0.22")
	if strings.Contains(mysqlVersion.String(), "MariaDB") {
		query = "SHOW SLAVE STATUS"
	} else if mysqlVersion.LessThan(minMysqlVersion) {
		query = "SHOW SLAVE STATUS"
	}

	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var stats []ReplicaStatusStats
	for rows.Next() {
		var s ReplicaStatusStats
		dest := []any{}
		for _, col := range cols {
			switch strings.ToLower(col) {
			case "replica_io_state":
				dest = append(dest, &s.replicaIOState)
			case "slave_io_state":
				dest = append(dest, &s.replicaIOState)
			case "source_host":
				dest = append(dest, &s.sourceHost)
			case "master_host":
				dest = append(dest, &s.sourceHost)
			case "source_user":
				dest = append(dest, &s.sourceUser)
			case "master_user":
				dest = append(dest, &s.sourceUser)
			case "source_port":
				dest = append(dest, &s.sourcePort)
			case "master_port":
				dest = append(dest, &s.sourcePort)
			case "connect_retry":
				dest = append(dest, &s.connectRetry)
			case "source_log_file":
				dest = append(dest, &s.sourceLogFile)
			case "master_log_file":
				dest = append(dest, &s.sourceLogFile)
			case "read_source_log_pos":
				dest = append(dest, &s.readSourceLogPos)
			case "read_master_log_pos":
				dest = append(dest, &s.readSourceLogPos)
			case "relay_log_file":
				dest = append(dest, &s.relayLogFile)
			case "relay_log_pos":
				dest = append(dest, &s.relayLogPos)
			case "relay_source_log_file":
				dest = append(dest, &s.relaySourceLogFile)
			case "relay_master_log_file":
				dest = append(dest, &s.relaySourceLogFile)
			case "replica_io_running":
				dest = append(dest, &s.replicaIORunning)
			case "slave_io_running":
				dest = append(dest, &s.replicaIORunning)
			case "replica_sql_running":
				dest = append(dest, &s.replicaSQLRunning)
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
				dest = append(dest, &s.execSourceLogPos)
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
				dest = append(dest, &s.sourceSSLAllowed)
			case "master_ssl_allowed":
				dest = append(dest, &s.sourceSSLAllowed)
			case "source_ssl_ca_file":
				dest = append(dest, &s.sourceSSLCAFile)
			case "master_ssl_ca_file":
				dest = append(dest, &s.sourceSSLCAFile)
			case "source_ssl_ca_path":
				dest = append(dest, &s.sourceSSLCAPath)
			case "master_ssl_ca_path":
				dest = append(dest, &s.sourceSSLCAPath)
			case "source_ssl_cert":
				dest = append(dest, &s.sourceSSLCert)
			case "master_ssl_cert":
				dest = append(dest, &s.sourceSSLCert)
			case "source_ssl_cipher":
				dest = append(dest, &s.sourceSSLCipher)
			case "master_ssl_cipher":
				dest = append(dest, &s.sourceSSLCipher)
			case "source_ssl_key":
				dest = append(dest, &s.sourceSSLKey)
			case "master_ssl_key":
				dest = append(dest, &s.sourceSSLKey)
			case "seconds_behind_source":
				dest = append(dest, &s.secondsBehindSource)
			case "seconds_behind_master":
				dest = append(dest, &s.secondsBehindSource)
			case "source_ssl_verify_server_cert":
				dest = append(dest, &s.sourceSSLVerifyServerCert)
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
				dest = append(dest, &s.replicateIgnoreServerIDs)
			case "source_server_id":
				dest = append(dest, &s.sourceServerID)
			case "master_server_id":
				dest = append(dest, &s.sourceServerID)
			case "source_uuid":
				dest = append(dest, &s.sourceUUID)
			case "master_uuid":
				dest = append(dest, &s.sourceUUID)
			case "source_info_file":
				dest = append(dest, &s.sourceInfoFile)
			case "master_info_file":
				dest = append(dest, &s.sourceInfoFile)
			case "sql_delay":
				dest = append(dest, &s.sqlDelay)
			case "sql_remaining_delay":
				dest = append(dest, &s.sqlRemainingDelay)
			case "replica_sql_running_state":
				dest = append(dest, &s.replicaSQLRunningState)
			case "slave_sql_running_state":
				dest = append(dest, &s.replicaSQLRunningState)
			case "source_retry_count":
				dest = append(dest, &s.sourceRetryCount)
			case "master_retry_count":
				dest = append(dest, &s.sourceRetryCount)
			case "source_bind":
				dest = append(dest, &s.sourceBind)
			case "master_bind":
				dest = append(dest, &s.sourceBind)
			case "last_io_error_timestamp":
				dest = append(dest, &s.lastIOErrorTimestamp)
			case "last_sql_error_timestamp":
				dest = append(dest, &s.lastSQLErrorTimestamp)
			case "source_ssl_crl":
				dest = append(dest, &s.sourceSSLCrl)
			case "master_ssl_crl":
				dest = append(dest, &s.sourceSSLCrl)
			case "source_ssl_crlpath":
				dest = append(dest, &s.sourceSSLCrlpath)
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
				dest = append(dest, &s.sourceTLSVersion)
			case "master_tls_version":
				dest = append(dest, &s.sourceTLSVersion)
			case "source_public_key_path":
				dest = append(dest, &s.sourcePublicKeyPath)
			case "master_public_key_path":
				dest = append(dest, &s.sourcePublicKeyPath)
			case "get_source_public_key":
				dest = append(dest, &s.getSourcePublicKey)
			case "get_master_public_key":
				dest = append(dest, &s.getSourcePublicKey)
			case "network_namespace":
				dest = append(dest, &s.networkNamespace)
			case "using_gtid":
				dest = append(dest, &s.usingGtid)
			case "gtid_io_pos":
				dest = append(dest, &s.gtidIoPos)
			case "slave_ddl_groups":
				dest = append(dest, &s.slaveDdlGroups)
			case "slave_non_transactional_groups":
				dest = append(dest, &s.slaveNonTransactionalGroups)
			case "slave_transactional_groups":
				dest = append(dest, &s.slaveTransactionalGroups)
			case "retried_transactions":
				dest = append(dest, &s.retriedTransactions)
			case "max_relay_log_size":
				dest = append(dest, &s.maxRelayLogSize)
			case "executed_log_entries":
				dest = append(dest, &s.executedLogEntries)
			case "slave_received_heartbeats":
				dest = append(dest, &s.slaveReceivedHeartbeats)
			case "slave_heartbeat_period":
				dest = append(dest, &s.slaveHeartbeatPeriod)
			case "gtid_slave_pos":
				dest = append(dest, &s.gtidSlavePos)
			case "master_last_event_time":
				dest = append(dest, &s.masterLastEventTime)
			case "slave_last_event_time":
				dest = append(dest, &s.slaveLastEventTime)
			case "master_slave_time_diff":
				dest = append(dest, &s.masterSlaveTimeDiff)
			case "parallel_mode":
				dest = append(dest, &s.parallelMode)
			case "replicate_do_domain_ids":
				dest = append(dest, &s.replicateDoDomainIDs)
			case "replicate_ignore_domain_ids":
				dest = append(dest, &s.replicateIgnoreDomainIDs)
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

// TODO take a timestamp and restrict the timer_start < timestamp
func (c *mySQLClient) getQueryStats(since int64, topCount int) ([]QueryStats, error) {
	query := "SELECT A.digest_text AS query_text," +
		"A.digest AS hash," +
		"count(*) AS execution_count," +
		"JSON_ARRAYAGG(A.current_schema) AS schema_nm, " +
		"sum(A.lock_time)/1e12 AS lock_time, " +
		"sum(A.rows_examined) AS total_rows, " +
		"sum(A.cpu_time)/1e12 AS cpu_time, " +
		"sum(A.timer_wait)/1e12 AS duration, " +
		"sum(A.rows_sent) AS rows_returned, " +
		"sum(B.sum_timer_wait) AS total_wait, " +
		"sum(A.rows_affected) AS rows_affected, " +
		"sum(A.select_full_join) AS full_joins, " +
		"sum(A.select_full_range_join) AS full_range_joins, " +
		"sum(A.select_range) AS select_ranges, " +
		"sum(A.select_range_check) AS select_range_checks, " +
		"sum(A.select_scan) AS select_scans, " +
		"sum(A.sort_merge_passes) AS sort_merge_passes, " +
		"sum(A.sort_range) AS sort_ranges, " +
		"sum(A.sort_rows) AS sort_rows, " +
		"sum(A.sort_scan) AS sort_scans, " +
		"sum(A.no_index_used) AS no_index_used, " +
		"sum(A.no_good_index_used) AS no_good_index_used, " +
		"JSON_ARRAYAGG(C.processlist_user) AS users, " +
		"JSON_ARRAYAGG(C.processlist_host) AS hosts, " +
		"JSON_ARRAYAGG(C.processlist_db) AS dbs, " +
		"ANY_VALUE(A.SQL_TEXT) AS literal_query_sample " +
		"FROM performance_schema.events_statements_history AS A, " +
		"performance_schema.events_statements_summary_by_digest AS B, " +
		"performance_schema.threads AS C " +
		"WHERE A.event_name = 'statement/sql/select' " +
		"AND A.digest_text IS NOT NULL " +
		"AND A.digest_text NOT LIKE 'EXPLAIN%' " +
		"AND A.timer_start > " + strconv.FormatInt(since, 10) + " " +
		"AND A.digest = B.digest " +
		"AND A.thread_id = C.thread_id " +
		"GROUP BY hash, query_text " +
		"ORDER BY duration desc " +
		"LIMIT " + strconv.FormatInt(int64(topCount), 10) + ";"
	rows, err := c.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []QueryStats
	for rows.Next() {
		var s QueryStats
		var schemas sql.NullString
		var users sql.NullString
		var hosts sql.NullString
		var dbs sql.NullString
		err := rows.Scan(
			&s.queryText,
			&s.queryDigest,
			&s.count,
			&schemas,
			&s.lockTime,
			&s.rowsExamined,
			&s.cpuTime,
			&s.totalDuration,
			&s.rowsReturned,
			&s.totalWait,
			&s.rowsAffected,
			&s.fullJoins,
			&s.fullRangeJoins,
			&s.selectRanges,
			&s.selectRangesChecks,
			&s.selectScans,
			&s.sortMergePasses,
			&s.sortRanges,
			&s.sortRows,
			&s.sortScans,
			&s.noIndexUsed,
			&s.noGoodIndexUsed,
			&users,
			&hosts,
			&dbs,
			&s.querySample,
		)
		s.schema = stringifyJsonStringArray(schemas)
		s.users = stringifyJsonStringArray(users)
		s.hosts = stringifyJsonStringArray(hosts)
		s.dbs = stringifyJsonStringArray(dbs)

		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (c *mySQLClient) getExplainPlanAsJsonForDigestQuery(query string) (string, error) {
	// The EXPLAIN FORMAT=JSON statement is used to get the execution plan for a query in JSON format.
	// This is useful for analyzing how MySQL will execute the query and can help identify performance issues.
	explainQuery := fmt.Sprintf("EXPLAIN FORMAT=JSON %s", query)
	rows, err := c.client.Query(explainQuery)
	if err != nil {
		// errors are typically gathered and handled by the caller, but in this case we want to be sure we see the updated
		// query that caused the error in the logs.
		// NOTE
		c.logger.Warn("MySQL EXPLAIN returned an error for query", zap.String("query", query), zap.Error(err))
		return "", err
	}
	defer rows.Close()

	var explainPlan string
	if rows.Next() {
		var jsonPlan sql.NullString
		err := rows.Scan(&jsonPlan)
		if err != nil {
			return "", err
		}
		if jsonPlan.Valid {
			explainPlan = jsonPlan.String
		}
	}
	return explainPlan, nil
}

func (c *mySQLClient) checkPerformanceCollectionSettings() {
	// Check if the performance_schema is enabled
	var performanceSchemaEnabled string
	err := c.client.QueryRow("SELECT @@performance_schema").Scan(&performanceSchemaEnabled)
	if err != nil {
		c.logger.Error("Error checking performance_schema", zap.Error(err))
		return
	}

	if performanceSchemaEnabled == "0" {
		c.logger.Warn("Performance schema is not enabled. Some metrics may not be available.")
	}

	var setupConsumers struct {
		name    string
		enabled string
	}

	rows, err := c.client.Query("SELECT name, enabled FROM performance_schema.setup_consumers")
	if err != nil {
		c.logger.Error("Error checking setup_consumers", zap.Error(err))
		return
	}
	for rows.Next() {
		err := rows.Scan(&setupConsumers.name, &setupConsumers.enabled)
		if err != nil {
			c.logger.Error("Error checking setup_consumers", zap.Error(err))
			return
		}
		switch setupConsumers.name {
		case "events_statements_current", "events_statements_history", "events_statements_history_long":
			if setupConsumers.enabled != "YES" {
				c.logger.Warn(fmt.Sprintf("Performance schema consumer %s is not enabled. Some metrics may not be available.", setupConsumers.name))
			}
		}
	}
}

func stringifyJsonStringArray(input sql.NullString) string {
	if !input.Valid {
		return ""
	}
	// take out quotes, brackets, and spaces
	str := stringArrayRegex.ReplaceAllString(input.String, "")
	str = strings.ReplaceAll(str, "null", "")
	//split on commas
	strSlice := strings.Split(str, ",")
	// sort so that Compact works
	slices.Sort(strSlice)
	// remove duplicates
	strSlice = slices.Compact(strSlice)

	if len(strSlice) == 1 && strSlice[0] == "" {
		return "NONE"
	}
	return strings.Join(strSlice, ",")
}

func query(c mySQLClient, query string) (map[string]string, error) {
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
