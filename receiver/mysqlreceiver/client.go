// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	// registers the mysql driver
	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
	"go.uber.org/zap"
)

// dbProduct identifies the database product (MySQL or MariaDB).
type dbProduct int

const (
	dbProductMySQL dbProduct = iota
	dbProductMariaDB
)

// minMySQLReplicaStatusVersion is the MySQL version at which SHOW REPLICA STATUS
// replaced SHOW SLAVE STATUS. Initialized at package load; panics on bad literal.
var minMySQLReplicaStatusVersion = version.Must(version.NewVersion("8.0.22"))

// dbVersion holds the parsed database version and product identity.
// Capability predicates keep version-specific branching out of callers.
type dbVersion struct {
	product dbProduct
	version *version.Version
}

// isValid reports whether a version was successfully detected.
func (v dbVersion) isValid() bool {
	return v.version != nil
}

// productString returns a human-readable product name for logging.
func (v dbVersion) productString() string {
	if v.product == dbProductMariaDB {
		return "MariaDB"
	}
	return "MySQL"
}

// supportsQuerySampleText reports whether the server's
// performance_schema.events_statements_summary_by_digest table includes the
// query_sample_text column, introduced in MySQL 8.0.3. Absent on MySQL <8.0.3
// and all MariaDB versions.
var minMySQLQuerySampleTextVersion = version.Must(version.NewVersion("8.0.3"))

func (v dbVersion) supportsQuerySampleText() bool {
	if !v.isValid() {
		return false
	}
	return v.product == dbProductMySQL && !v.version.LessThan(minMySQLQuerySampleTextVersion)
}

// supportsReplicaStatus reports whether the server uses SHOW REPLICA STATUS
// (MySQL 8.0.22+). Older MySQL versions and all MariaDB versions use the
// deprecated SHOW SLAVE STATUS syntax.
func (v dbVersion) supportsReplicaStatus() bool {
	if !v.isValid() {
		return false
	}
	return v.product == dbProductMySQL && !v.version.LessThan(minMySQLReplicaStatusVersion)
}

// supportsProcesslist reports whether performance_schema.processlist is available
// (MySQL 8.0.22+). This table exposes HOST as "host:port" for TCP/IP connections,
// enabling population of client.port and network.peer.port on query sample events.
//
// The only alternative source that exposes host:port is information_schema.PROCESSLIST,
// but it is not used: its implementation iterates active threads while holding a global
// mutex (same as SHOW PROCESSLIST), which has negative performance consequences on busy
// systems. It was also deprecated in MySQL 8.0, removed in MySQL 9.0, and was already
// removed from this receiver in a prior change. performance_schema.processlist is
// lock-free and reads directly from Performance Schema data structures.
//
// As a result, client.port and network.peer.port remain 0 on MariaDB (all versions)
// and MySQL < 8.0.22, where this table is not available.
func (v dbVersion) supportsProcesslist() bool {
	if !v.isValid() {
		return false
	}
	return v.product == dbProductMySQL && !v.version.LessThan(minMySQLReplicaStatusVersion)
}

type client interface {
	Connect() error
	getDBVersion() dbVersion
	getGlobalStats() (map[string]string, error)
	getInnodbStats() (map[string]string, error)
	getTableStats() ([]tableStats, error)
	getTableIoWaitsStats() ([]tableIoWaitsStats, error)
	getIndexIoWaitsStats() ([]indexIoWaitsStats, error)
	getStatementEventsStats() ([]statementEventStats, error)
	getTableLockWaitEventStats() ([]tableLockWaitEventStats, error)
	// supportsReplicaStatus controls whether SHOW REPLICA STATUS or the
	// deprecated SHOW SLAVE STATUS is used.
	getReplicaStatusStats(supportsReplicaStatus bool) ([]replicaStatusStats, error)
	// supportsSampleText controls top-query template selection (MySQL 8+ vs fallback).
	getTopQueries(topN, lookback uint64, supportsSampleText bool) ([]topQuery, error)
	// supportsProcesslist controls whether the performance_schema.processlist JOIN
	// is included to populate client.port and network.peer.port (MySQL 8.0.22+).
	getQuerySamples(limit uint64, supportsProcesslist bool) ([]querySample, error)
	explainQuery(digestText, sampleStatement, schema, digest string, logger *zap.Logger) string
	Close() error
}

type mySQLClient struct {
	connStr                        string
	client                         *sql.DB
	statementEventsDigestTextLimit int
	statementEventsLimit           int
	statementEventsTimeLimit       time.Duration
	dbVersion                      dbVersion
}

type ioWaitsStats struct {
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

type tableIoWaitsStats struct {
	ioWaitsStats
}

type indexIoWaitsStats struct {
	ioWaitsStats
	index string
}

type tableStats struct {
	schema           string
	name             string
	rows             int64
	averageRowLength int64
	dataLength       int64
	indexLength      int64
}

type statementEventStats struct {
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

type replicaStatusStats struct {
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

type querySample struct {
	sessionID          int64
	threadID           int64
	processlistUser    string
	processlistHost    string
	clientPort         uint64
	processlistDB      string
	processlistCommand string
	processlistState   string
	sqlText            string
	digest             string
	eventID            int64
	sessionStatus      string
	waitEvent          string
	waitTime           float64
	statementTimerWait float64
	traceparent        string
}

type topQuery struct {
	schemaName                string
	digest                    string
	digestText                string
	countStar                 int64
	sumTimerWaitInPicoSeconds int64
	querySampleText           string
}

var _ client = (*mySQLClient)(nil)

func newMySQLClient(conf *Config) (client, error) {
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
	}, nil
}

func (c *mySQLClient) Connect() error {
	clientDB, err := sql.Open("mysql", c.connStr)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	c.client = clientDB

	// Version detection runs exactly once during Connect and is non-fatal.
	// If the query fails (e.g. no database reachable at startup) or the
	// version string cannot be parsed, dbVersion stays at its zero value,
	// which selects the safe fallback template (MySQL <8 / MariaDB behavior)
	// for the entire lifetime of this receiver instance. Any connection error
	// encountered here will cause the receiver to operate with incorrect
	// version information; it will not be retried.
	//
	// This is intentional: sql.Open is lazy and the component lifecycle test
	// calls start() against 127.0.0.1:3306 with no live database. A hard
	// failure here would break that test with no way to inject a mock client
	// before Connect runs. Real connection errors surface on the first scrape.
	if dbVer, verErr := c.fetchDBVersion(); verErr == nil {
		c.dbVersion = dbVer
	}
	return nil
}

// fetchDBVersion queries the database for its version string and parses it
// into a dbVersion. Called once during Connect. A short context timeout
// prevents a blackholed or slow endpoint from stalling collector startup.
func (c *mySQLClient) fetchDBVersion() (dbVersion, error) {
	const versionDetectTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), versionDetectTimeout)
	defer cancel()

	var versionStr string
	if err := c.client.QueryRowContext(ctx, "SELECT VERSION();").Scan(&versionStr); err != nil {
		return dbVersion{}, err
	}
	return parseDBVersion(versionStr)
}

// mariaDBVersionRe extracts the leading semver triplet from a MariaDB VERSION()
// string after the optional MySQL-compat "5.5.5-" prefix has been stripped.
// Examples handled:
//   - "10.11.6-MariaDB"                          → "10.11.6"
//   - "5.5.5-10.11.6-MariaDB"                    → "10.11.6"
//   - "10.6.14-MariaDB-1:10.6.14+maria~ubu2204"  → "10.6.14"
//   - "10.11.6-MariaDB-log"                      → "10.11.6"
var mariaDBVersionRe = regexp.MustCompile(`^(\d+\.\d+\.\d+)`)

// mysqlCompatPrefix is prepended by older MariaDB 10.x builds to fool
// MySQL clients that require a version >= 5.5.5. Strip it before parsing.
const mysqlCompatPrefix = "5.5.5-"

// parseDBVersion parses a raw VERSION() string into a dbVersion.
//
// MySQL strings (no "MariaDB" substring): the semver is everything before the
// first "-" (handles suffixes like "-log").
//
// MariaDB strings: strip the optional MySQL-compat "5.5.5-" prefix, then
// extract the leading dotted-decimal triplet via regex.
func parseDBVersion(versionStr string) (dbVersion, error) {
	if strings.Contains(versionStr, "MariaDB") {
		s := strings.TrimPrefix(versionStr, mysqlCompatPrefix)
		m := mariaDBVersionRe.FindStringSubmatch(s)
		if m == nil {
			return dbVersion{}, fmt.Errorf("failed to parse MariaDB version %q: no semver found", versionStr)
		}
		v, err := version.NewVersion(m[1])
		if err != nil {
			return dbVersion{}, fmt.Errorf("failed to parse MariaDB version %q: %w", versionStr, err)
		}
		return dbVersion{product: dbProductMariaDB, version: v}, nil
	}

	// MySQL: strip any suffix after the first "-"
	semverStr := strings.SplitN(versionStr, "-", 2)[0]
	v, err := version.NewVersion(semverStr)
	if err != nil {
		return dbVersion{}, fmt.Errorf("failed to parse db version %q: %w", versionStr, err)
	}
	return dbVersion{product: dbProductMySQL, version: v}, nil
}

// getDBVersion returns the cached database version established during Connect.
func (c *mySQLClient) getDBVersion() dbVersion {
	return c.dbVersion
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
func (c *mySQLClient) getTableStats() ([]tableStats, error) {
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
	var stats []tableStats
	for rows.Next() {
		var s tableStats
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
func (c *mySQLClient) getTableIoWaitsStats() ([]tableIoWaitsStats, error) {
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
	var stats []tableIoWaitsStats
	for rows.Next() {
		var s tableIoWaitsStats
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
func (c *mySQLClient) getIndexIoWaitsStats() ([]indexIoWaitsStats, error) {
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
	var stats []indexIoWaitsStats
	for rows.Next() {
		var s indexIoWaitsStats
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

func (c *mySQLClient) getStatementEventsStats() ([]statementEventStats, error) {
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

	var stats []statementEventStats
	for rows.Next() {
		var s statementEventStats
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

func (c *mySQLClient) getReplicaStatusStats(supportsReplicaStatus bool) ([]replicaStatusStats, error) {
	query := "SHOW REPLICA STATUS"
	if !supportsReplicaStatus {
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

	var stats []replicaStatusStats
	for rows.Next() {
		var s replicaStatusStats
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

//go:embed templates/topQuery.tmpl
var topQueryTemplate string

//go:embed templates/topQueryNoSampleText.tmpl
var topQueryNoSampleTextTemplate string

func (c *mySQLClient) getTopQueries(topNValue, lookbackTime uint64, supportsSampleText bool) ([]topQuery, error) {
	// Select the appropriate template based on version support.
	// MySQL <8 and all MariaDB versions lack query_sample_text in
	// events_statements_summary_by_digest, so we use the 5-column fallback.
	tmplSrc := topQueryTemplate
	if !supportsSampleText {
		tmplSrc = topQueryNoSampleTextTemplate
	}

	tmpl, tmplErr := template.New("topQuery").Option("missingkey=error").Parse(tmplSrc)
	if tmplErr != nil {
		return nil, fmt.Errorf("failed to parse top query template: %w", tmplErr)
	}
	buf := bytes.Buffer{}

	if tmplErr = tmpl.Execute(&buf, map[string]any{
		"topNValue":    topNValue,
		"lookbackTime": lookbackTime,
	}); tmplErr != nil {
		return nil, fmt.Errorf("failed to execute template: %w", tmplErr)
	}

	rows, err := c.client.Query(buf.String())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	// scanRow is defined once outside the loop to avoid re-evaluating
	// supportsSampleText on every iteration.
	scanRow := func(tq *topQuery) error {
		if supportsSampleText {
			return rows.Scan(
				&tq.schemaName,
				&tq.digest,
				&tq.digestText,
				&tq.countStar,
				&tq.sumTimerWaitInPicoSeconds,
				&tq.querySampleText,
			)
		}
		// querySampleText stays "" — sentinel for "no sample available"
		return rows.Scan(
			&tq.schemaName,
			&tq.digest,
			&tq.digestText,
			&tq.countStar,
			&tq.sumTimerWaitInPicoSeconds,
		)
	}

	var topQueries []topQuery
	for rows.Next() {
		var tq topQuery
		if err := scanRow(&tq); err != nil {
			return nil, err
		}
		topQueries = append(topQueries, tq)
	}
	return topQueries, nil
}

//go:embed templates/querySample.tmpl
var querySampleTemplate string

func (c *mySQLClient) getQuerySamples(limit uint64, supportsProcesslist bool) ([]querySample, error) {
	tmpl := template.Must(template.New("querySample").Option("missingkey=error").Parse(querySampleTemplate))
	buf := bytes.Buffer{}

	if err := tmpl.Execute(&buf, map[string]any{
		"limit":               limit,
		"supportsProcesslist": supportsProcesslist,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	rows, err := c.client.Query(buf.String())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var samples []querySample
	for rows.Next() {
		var s querySample
		dest := make([]any, 0, len(cols))
		for _, col := range cols {
			switch strings.ToLower(col) {
			case "session_id":
				dest = append(dest, &s.sessionID)
			case "internal_thread_id":
				dest = append(dest, &s.threadID)
			case "session_user":
				dest = append(dest, &s.processlistUser)
			case "client_address":
				dest = append(dest, &s.processlistHost)
			case "client_port":
				dest = append(dest, &s.clientPort)
			case "current_database":
				dest = append(dest, &s.processlistDB)
			case "session_command":
				dest = append(dest, &s.processlistCommand)
			case "session_state":
				dest = append(dest, &s.processlistState)
			case "sql_text":
				dest = append(dest, &s.sqlText)
			case "fingerprint":
				dest = append(dest, &s.digest)
			case "activity_event_id":
				dest = append(dest, &s.eventID)
			case "session_status":
				dest = append(dest, &s.sessionStatus)
			case "wait_event":
				dest = append(dest, &s.waitEvent)
			case "wait_time_seconds":
				dest = append(dest, &s.waitTime)
			case "statement_timer_wait_seconds":
				dest = append(dest, &s.statementTimerWait)
			case "traceparent":
				dest = append(dest, &s.traceparent)
			default:
				return nil, fmt.Errorf("unknown column name %q for query samples", col)
			}
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		// pl.HOST from performance_schema.processlist uses "host:port" for IPv4 and
		// "[::1]:port" for IPv6, both parseable by net.SplitHostPort. When processlist
		// is not joined (supportsProcesslist=false), processlistHost is a bare hostname
		// from thread.processlist_host and SplitHostPort will return an error — in that
		// case we leave processlistHost as-is and clientPort as 0.
		if host, portStr, splitErr := net.SplitHostPort(s.processlistHost); splitErr == nil {
			if port, parseErr := strconv.ParseUint(portStr, 10, 64); parseErr == nil {
				s.processlistHost = host
				s.clientPort = port
			}
		}

		samples = append(samples, s)
	}

	return samples, nil
}

func (c *mySQLClient) explainQuery(digestText, sampleStatement, schema, digest string, logger *zap.Logger) string {
	if strings.HasSuffix(sampleStatement, "...") {
		logger.Debug("statement is truncated, skipping explain", zap.String("digest_text", digestText))
		return ""
	}
	if !isQueryExplainable(digestText) {
		logger.Debug("statement is not explainable, skipping explain query", zap.String("digest", digest))
		return ""
	}

	if schema != "" {
		if _, err := c.client.Exec(fmt.Sprintf("/* otel-collector-ignore */ USE `%s`;", strings.ReplaceAll(schema, "`", "``"))); err != nil {
			logger.Warn(fmt.Sprintf("unable to use schema: %s", schema), zap.String("digest", digest), zap.Error(err))
			return ""
		}
	}

	var plan string
	err := c.client.QueryRow("EXPLAIN FORMAT=json " + strings.TrimSpace(sampleStatement)).Scan(&plan)
	if err != nil {
		logger.Warn("unable to execute explain statement", zap.String("digest", digest), zap.Error(err))
		return ""
	}
	if plan == "" {
		logger.Warn("explain query returned empty plan", zap.String("digest", digest))
	}
	return plan
}

// This function filters out queries that are unsupported by 'EXPLAIN'
// ref: https://dev.mysql.com/doc/refman/8.4/en/using-explain.html
func isQueryExplainable(query string) bool {
	sqlStartingKeywords := []string{
		"select",
		"delete",
		"insert",
		"replace",
		"update",
	}

	trimmedQuery := strings.TrimSpace(query)
	lowerQuery := strings.ToLower(stripLeadingSQLComments(trimmedQuery))

	for _, keyword := range sqlStartingKeywords {
		if strings.HasPrefix(lowerQuery, keyword) {
			return true
		}
	}
	return false
}

// stripLeadingSQLComments removes leading block (/* ... */)
// and line comments (-- ... and # ...) from a query.
func stripLeadingSQLComments(query string) string {
	for {
		switch {
		case strings.HasPrefix(query, "/*"):
			end := strings.Index(query, "*/")
			if end == -1 {
				return query
			}
			query = strings.TrimSpace(query[end+2:])
		case strings.HasPrefix(query, "--"), strings.HasPrefix(query, "#"):
			end := strings.Index(query, "\n")
			if end == -1 {
				// remaining text is a comment
				return ""
			}
			query = strings.TrimSpace(query[end+1:])
		default:
			return query
		}
	}
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
