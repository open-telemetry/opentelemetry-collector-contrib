// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/multierr"
)

const lagMetricsInSecondsFeatureGateID = "postgresqlreceiver.preciselagmetrics"

var preciseLagMetricsFg = featuregate.GlobalRegistry().MustRegister(
	lagMetricsInSecondsFeatureGateID,
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("Metric `postgresql.wal.lag` is replaced by more precise `postgresql.wal.delay`."),
	featuregate.WithRegisterFromVersion("0.89.0"),
)

// databaseName is a name that refers to a database so that it can be uniquely referred to later
// i.e. database1
type databaseName string

// tableIdentifier is an identifier that contains both the database and table separated by a "|"
// i.e. database1|table2
type tableIdentifier string

// indexIdentifier is a unique string that identifies a particular index and is separated by the "|" character
type indexIdentifer string

// errNoLastArchive is an error that occurs when there is no previous wal archive, so there is no way to compute the
// last archived point
var errNoLastArchive = errors.New("no last archive found, not able to calculate oldest WAL age")

type client interface {
	Close() error
	getDatabaseStats(ctx context.Context, databases []string) (map[databaseName]databaseStats, error)
	getDatabaseLocks(ctx context.Context) ([]databaseLocks, error)
	getBGWriterStats(ctx context.Context) (*bgStat, error)
	getBackends(ctx context.Context, databases []string) (map[databaseName]int64, error)
	getDatabaseSize(ctx context.Context, databases []string) (map[databaseName]int64, error)
	getDatabaseTableMetrics(ctx context.Context, db string) (map[tableIdentifier]tableStats, error)
	getBlocksReadByTable(ctx context.Context, db string) (map[tableIdentifier]tableIOStats, error)
	getReplicationStats(ctx context.Context) ([]replicationStats, error)
	getLatestWalAgeSeconds(ctx context.Context) (int64, error)
	getMaxConnections(ctx context.Context) (int64, error)
	getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error)
	listDatabases(ctx context.Context) ([]string, error)
}

type postgreSQLClient struct {
	client  *sql.DB
	closeFn func() error
}

var _ client = (*postgreSQLClient)(nil)

type postgreSQLConfig struct {
	username string
	password string
	database string
	address  confignet.AddrConfig
	tls      configtls.ClientConfig
}

func sslConnectionString(tls configtls.ClientConfig) string {
	if tls.Insecure {
		return "sslmode='disable'"
	}

	conn := ""

	if tls.InsecureSkipVerify {
		conn += "sslmode='require'"
	} else {
		conn += "sslmode='verify-full'"
	}

	if tls.CAFile != "" {
		conn += fmt.Sprintf(" sslrootcert='%s'", tls.CAFile)
	}

	if tls.KeyFile != "" {
		conn += fmt.Sprintf(" sslkey='%s'", tls.KeyFile)
	}

	if tls.CertFile != "" {
		conn += fmt.Sprintf(" sslcert='%s'", tls.CertFile)
	}

	return conn
}

func (c postgreSQLConfig) ConnectionString() (string, error) {
	// postgres will assume the supplied user as the database name if none is provided,
	// so we must specify a database name even when we are just collecting the list of databases.
	database := defaultPostgreSQLDatabase
	if c.database != "" {
		database = c.database
	}

	host, port, err := net.SplitHostPort(c.address.Endpoint)
	if err != nil {
		return "", err
	}

	if c.address.Transport == confignet.TransportTypeUnix {
		// lib/pg expects a unix socket host to start with a "/" and appends the appropriate .s.PGSQL.port internally
		host = fmt.Sprintf("/%s", host)
	}

	return fmt.Sprintf("port=%s host=%s user=%s password=%s dbname=%s %s", port, host, c.username, c.password, database, sslConnectionString(c.tls)), nil
}

func (c *postgreSQLClient) Close() error {
	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}

type databaseStats struct {
	transactionCommitted int64
	transactionRollback  int64
	deadlocks            int64
	tempFiles            int64
}

func (c *postgreSQLClient) getDatabaseStats(ctx context.Context, databases []string) (map[databaseName]databaseStats, error) {
	query := filterQueryByDatabases("SELECT datname, xact_commit, xact_rollback, deadlocks, temp_files FROM pg_stat_database", databases, false)
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	var errs error
	dbStats := map[databaseName]databaseStats{}
	for rows.Next() {
		var datname string
		var transactionCommitted, transactionRollback, deadlocks, tempFiles int64
		err = rows.Scan(&datname, &transactionCommitted, &transactionRollback, &deadlocks, &tempFiles)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		if datname != "" {
			dbStats[databaseName(datname)] = databaseStats{
				transactionCommitted: transactionCommitted,
				transactionRollback:  transactionRollback,
				deadlocks:            deadlocks,
				tempFiles:            tempFiles,
			}
		}
	}
	return dbStats, errs
}

type databaseLocks struct {
	relation string
	mode     string
	lockType string
	locks    int64
}

func (c *postgreSQLClient) getDatabaseLocks(ctx context.Context) ([]databaseLocks, error) {
	query := `SELECT relname AS relation, mode, locktype,COUNT(pid)
	AS locks FROM pg_locks
	JOIN pg_class ON pg_locks.relation = pg_class.oid
	GROUP BY relname, mode, locktype;`

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_locks and pg_locks.relation: %w", err)
	}
	defer rows.Close()
	var dl []databaseLocks
	var errs []error
	for rows.Next() {
		var relation, mode, lockType string
		var locks int64
		err = rows.Scan(&relation, &mode, &lockType, &locks)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		dl = append(dl, databaseLocks{
			relation: relation,
			mode:     mode,
			lockType: lockType,
			locks:    locks,
		})
	}
	return dl, multierr.Combine(errs...)
}

// getBackends returns a map of database names to the number of active connections
func (c *postgreSQLClient) getBackends(ctx context.Context, databases []string) (map[databaseName]int64, error) {
	query := filterQueryByDatabases("SELECT datname, count(*) as count from pg_stat_activity", databases, true)
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ars := map[databaseName]int64{}
	var errors error
	for rows.Next() {
		var datname string
		var count int64
		err = rows.Scan(&datname, &count)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		if datname != "" {
			ars[databaseName(datname)] = count
		}
	}
	return ars, errors
}

func (c *postgreSQLClient) getDatabaseSize(ctx context.Context, databases []string) (map[databaseName]int64, error) {
	query := filterQueryByDatabases("SELECT datname, pg_database_size(datname) FROM pg_catalog.pg_database WHERE datistemplate = false", databases, false)
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sizes := map[databaseName]int64{}
	var errors error
	for rows.Next() {
		var datname string
		var size int64
		err = rows.Scan(&datname, &size)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		if datname != "" {
			sizes[databaseName(datname)] = size
		}
	}
	return sizes, errors
}

// tableStats contains a result for a row of the getDatabaseTableMetrics result
type tableStats struct {
	database    string
	schema      string
	table       string
	live        int64
	dead        int64
	inserts     int64
	upd         int64
	del         int64
	hotUpd      int64
	seqScans    int64
	size        int64
	vacuumCount int64
}

func (c *postgreSQLClient) getDatabaseTableMetrics(ctx context.Context, db string) (map[tableIdentifier]tableStats, error) {
	query := `SELECT schemaname as schema, relname AS table,
	n_live_tup AS live,
	n_dead_tup AS dead,
	n_tup_ins AS ins,
	n_tup_upd AS upd,
	n_tup_del AS del,
	n_tup_hot_upd AS hot_upd,
	seq_scan AS seq_scans,
	pg_relation_size(relid) AS table_size,
	vacuum_count
	FROM pg_stat_user_tables;`

	ts := map[tableIdentifier]tableStats{}
	var errors error
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var schema, table string
		var live, dead, ins, upd, del, hotUpd, seqScans, tableSize, vacuumCount int64
		err = rows.Scan(&schema, &table, &live, &dead, &ins, &upd, &del, &hotUpd, &seqScans, &tableSize, &vacuumCount)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		ts[tableKey(db, schema, table)] = tableStats{
			database:    db,
			schema:      schema,
			table:       table,
			live:        live,
			dead:        dead,
			inserts:     ins,
			upd:         upd,
			del:         del,
			hotUpd:      hotUpd,
			seqScans:    seqScans,
			size:        tableSize,
			vacuumCount: vacuumCount,
		}
	}
	return ts, errors
}

type tableIOStats struct {
	database  string
	schema    string
	table     string
	heapRead  int64
	heapHit   int64
	idxRead   int64
	idxHit    int64
	toastRead int64
	toastHit  int64
	tidxRead  int64
	tidxHit   int64
}

func (c *postgreSQLClient) getBlocksReadByTable(ctx context.Context, db string) (map[tableIdentifier]tableIOStats, error) {
	query := `SELECT schemaname as schema, relname AS table,
	coalesce(heap_blks_read, 0) AS heap_read,
	coalesce(heap_blks_hit, 0) AS heap_hit,
	coalesce(idx_blks_read, 0) AS idx_read,
	coalesce(idx_blks_hit, 0) AS idx_hit,
	coalesce(toast_blks_read, 0) AS toast_read,
	coalesce(toast_blks_hit, 0) AS toast_hit,
	coalesce(tidx_blks_read, 0) AS tidx_read,
	coalesce(tidx_blks_hit, 0) AS tidx_hit
	FROM pg_statio_user_tables;`

	tios := map[tableIdentifier]tableIOStats{}
	var errors error
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var schema, table string
		var heapRead, heapHit, idxRead, idxHit, toastRead, toastHit, tidxRead, tidxHit int64
		err = rows.Scan(&schema, &table, &heapRead, &heapHit, &idxRead, &idxHit, &toastRead, &toastHit, &tidxRead, &tidxHit)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		tios[tableKey(db, schema, table)] = tableIOStats{
			database:  db,
			schema:    schema,
			table:     table,
			heapRead:  heapRead,
			heapHit:   heapHit,
			idxRead:   idxRead,
			idxHit:    idxHit,
			toastRead: toastRead,
			toastHit:  toastHit,
			tidxRead:  tidxRead,
			tidxHit:   tidxHit,
		}
	}
	return tios, errors
}

type indexStat struct {
	index    string
	table    string
	schema   string
	database string
	size     int64
	scans    int64
}

func (c *postgreSQLClient) getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error) {
	query := `SELECT schemaname, relname, indexrelname,
	pg_relation_size(indexrelid) AS index_size,
	idx_scan
	FROM pg_stat_user_indexes;`

	stats := map[indexIdentifer]indexStat{}

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errs []error
	for rows.Next() {
		var (
			schema, table, index  string
			indexSize, indexScans int64
		)
		err := rows.Scan(&schema, &table, &index, &indexSize, &indexScans)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		stats[indexKey(database, schema, table, index)] = indexStat{
			index:    index,
			table:    table,
			schema:   schema,
			database: database,
			size:     indexSize,
			scans:    indexScans,
		}
	}
	return stats, multierr.Combine(errs...)
}

type bgStat struct {
	checkpointsReq       int64
	checkpointsScheduled int64
	checkpointWriteTime  float64
	checkpointSyncTime   float64
	bgWrites             int64
	backendWrites        int64
	bufferBackendWrites  int64
	bufferFsyncWrites    int64
	bufferCheckpoints    int64
	buffersAllocated     int64
	maxWritten           int64
}

func (c *postgreSQLClient) getBGWriterStats(ctx context.Context) (*bgStat, error) {
	query := `SELECT
	checkpoints_req AS checkpoint_req,
	checkpoints_timed AS checkpoint_scheduled,
	checkpoint_write_time AS checkpoint_duration_write,
	checkpoint_sync_time AS checkpoint_duration_sync,
	buffers_clean AS bg_writes,
	buffers_backend AS backend_writes,
	buffers_backend_fsync AS buffers_written_fsync,
	buffers_checkpoint AS buffers_checkpoints,
	buffers_alloc AS buffers_allocated,
	maxwritten_clean AS maxwritten_count
	FROM pg_stat_bgwriter;`

	row := c.client.QueryRowContext(ctx, query)
	var (
		checkpointsReq, checkpointsScheduled               int64
		checkpointSyncTime, checkpointWriteTime            float64
		bgWrites, bufferCheckpoints, bufferAllocated       int64
		bufferBackendWrites, bufferFsyncWrites, maxWritten int64
	)
	err := row.Scan(
		&checkpointsReq,
		&checkpointsScheduled,
		&checkpointWriteTime,
		&checkpointSyncTime,
		&bgWrites,
		&bufferBackendWrites,
		&bufferFsyncWrites,
		&bufferCheckpoints,
		&bufferAllocated,
		&maxWritten,
	)
	if err != nil {
		return nil, err
	}
	return &bgStat{
		checkpointsReq:       checkpointsReq,
		checkpointsScheduled: checkpointsScheduled,
		checkpointWriteTime:  checkpointWriteTime,
		checkpointSyncTime:   checkpointSyncTime,
		bgWrites:             bgWrites,
		backendWrites:        bufferBackendWrites,
		bufferBackendWrites:  bufferBackendWrites,
		bufferFsyncWrites:    bufferFsyncWrites,
		bufferCheckpoints:    bufferCheckpoints,
		buffersAllocated:     bufferAllocated,
		maxWritten:           maxWritten,
	}, nil
}

func (c *postgreSQLClient) getMaxConnections(ctx context.Context) (int64, error) {
	query := `SHOW max_connections;`
	row := c.client.QueryRowContext(ctx, query)
	var maxConns int64
	err := row.Scan(&maxConns)
	return maxConns, err
}

type replicationStats struct {
	clientAddr   string
	pendingBytes int64
	flushLagInt  int64 // Deprecated
	replayLagInt int64 // Deprecated
	writeLagInt  int64 // Deprecated
	flushLag     float64
	replayLag    float64
	writeLag     float64
}

func (c *postgreSQLClient) getDeprecatedReplicationStats(ctx context.Context) ([]replicationStats, error) {
	query := `SELECT
	coalesce(cast(client_addr as varchar), 'unix') AS client_addr,
	coalesce(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), -1) AS replication_bytes_pending,
	extract('epoch' from coalesce(write_lag, '-1 seconds'))::integer,
	extract('epoch' from coalesce(flush_lag, '-1 seconds'))::integer,
	extract('epoch' from coalesce(replay_lag, '-1 seconds'))::integer
	FROM pg_stat_replication;
	`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_stat_replication: %w", err)
	}
	defer rows.Close()
	var rs []replicationStats
	var errors error
	for rows.Next() {
		var client string
		var replicationBytes int64
		var writeLagInt, flushLagInt, replayLagInt int64
		err = rows.Scan(&client, &replicationBytes,
			&writeLagInt, &flushLagInt, &replayLagInt)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		rs = append(rs, replicationStats{
			clientAddr:   client,
			pendingBytes: replicationBytes,
			replayLagInt: replayLagInt,
			writeLagInt:  writeLagInt,
			flushLagInt:  flushLagInt,
		})
	}

	return rs, errors
}

func (c *postgreSQLClient) getReplicationStats(ctx context.Context) ([]replicationStats, error) {
	if !preciseLagMetricsFg.IsEnabled() {
		return c.getDeprecatedReplicationStats(ctx)
	}

	query := `SELECT
	coalesce(cast(client_addr as varchar), 'unix') AS client_addr,
	coalesce(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), -1) AS replication_bytes_pending,
	extract('epoch' from coalesce(write_lag, '-1 seconds'))::decimal AS write_lag_fractional,
	extract('epoch' from coalesce(flush_lag, '-1 seconds'))::decimal AS flush_lag_fractional,
	extract('epoch' from coalesce(replay_lag, '-1 seconds'))::decimal AS replay_lag_fractional
	FROM pg_stat_replication;
	`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_stat_replication: %w", err)
	}
	defer rows.Close()
	var rs []replicationStats
	var errors error
	for rows.Next() {
		var client string
		var replicationBytes int64
		var writeLag, flushLag, replayLag float64
		err = rows.Scan(&client, &replicationBytes, &writeLag, &flushLag, &replayLag)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		rs = append(rs, replicationStats{
			clientAddr:   client,
			pendingBytes: replicationBytes,
			replayLag:    replayLag,
			writeLag:     writeLag,
			flushLag:     flushLag,
		})
	}

	return rs, errors
}

func (c *postgreSQLClient) getLatestWalAgeSeconds(ctx context.Context) (int64, error) {
	query := `SELECT
	coalesce(last_archived_time, CURRENT_TIMESTAMP) AS last_archived_wal,
	CURRENT_TIMESTAMP
	FROM pg_stat_archiver;
	`
	row := c.client.QueryRowContext(ctx, query)
	var lastArchivedWal, currentInstanceTime time.Time
	err := row.Scan(&lastArchivedWal, &currentInstanceTime)
	if err != nil {
		return 0, err
	}

	if lastArchivedWal.Equal(currentInstanceTime) {
		return 0, errNoLastArchive
	}

	age := int64(currentInstanceTime.Sub(lastArchivedWal).Seconds())
	return age, nil
}

func (c *postgreSQLClient) listDatabases(ctx context.Context) ([]string, error) {
	query := `SELECT datname FROM pg_database
	WHERE datistemplate = false;`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}

		databases = append(databases, database)
	}
	return databases, nil
}

func filterQueryByDatabases(baseQuery string, databases []string, groupBy bool) string {
	if len(databases) > 0 {
		var queryDatabases []string
		for _, db := range databases {
			queryDatabases = append(queryDatabases, fmt.Sprintf("'%s'", db))
		}
		if strings.Contains(baseQuery, "WHERE") {
			baseQuery += fmt.Sprintf(" AND datname IN (%s)", strings.Join(queryDatabases, ","))
		} else {
			baseQuery += fmt.Sprintf(" WHERE datname IN (%s)", strings.Join(queryDatabases, ","))
		}
	}
	if groupBy {
		baseQuery += " GROUP BY datname"
	}

	return baseQuery + ";"
}

func tableKey(database, schema, table string) tableIdentifier {
	return tableIdentifier(fmt.Sprintf("%s|%s|%s", database, schema, table))
}

func indexKey(database, schema, table, index string) indexIdentifer {
	return indexIdentifer(fmt.Sprintf("%s|%s|%s|%s", database, schema, table, index))
}
