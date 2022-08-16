// Copyright  The OpenTelemetry Authors
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

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/lib/pq"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
)

// databaseName is a name that refers to a database so that it can be uniquely referred to later
// i.e. database1
type databaseName string

// tableIdentifier is an identifier that contains both the database and table separated by a "|"
// i.e. database1|table2
type tableIdentifier string

// indexIdentifier is a unique string that identifies a particular index and is separated by the "|" character
type indexIdentifer string

type client interface {
	Close() error
	getDatabaseStats(ctx context.Context, databases []string) (map[databaseName]databaseStats, error)
	getBGWriterStats(ctx context.Context) (*bgStat, error)
	getBackends(ctx context.Context, databases []string) (map[databaseName]int64, error)
	getDatabaseSize(ctx context.Context, databases []string) (map[databaseName]int64, error)
	getDatabaseTableMetrics(ctx context.Context, db string) (map[tableIdentifier]tableStats, error)
	getBlocksReadByTable(ctx context.Context, db string) (map[tableIdentifier]tableIOStats, error)
	getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error)
	getQueryStats(ctx context.Context, pgVersion *version.Version) ([]queryStat, error)
	getVersion(ctx context.Context) (*version.Version, error)
	listDatabases(ctx context.Context) ([]string, error)
}

type postgreSQLClient struct {
	client   *sql.DB
	database string
}

var _ client = (*postgreSQLClient)(nil)

type postgreSQLConfig struct {
	username string
	password string
	database string
	address  confignet.NetAddr
	tls      configtls.TLSClientSetting
}

func sslConnectionString(tls configtls.TLSClientSetting) string {
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

func newPostgreSQLClient(conf postgreSQLConfig) (*postgreSQLClient, error) {
	dbField := ""
	if conf.database != "" {
		dbField = fmt.Sprintf("dbname=%s ", conf.database)
	}

	host, port, err := net.SplitHostPort(conf.address.Endpoint)
	if err != nil {
		return nil, err
	}

	if conf.address.Transport == "unix" {
		// lib/pg expects a unix socket host to start with a "/" and appends the appropriate .s.PGSQL.port internally
		host = fmt.Sprintf("/%s", host)
	}

	connStr := fmt.Sprintf("port=%s host=%s user=%s password=%s %s%s", port, host, conf.username, conf.password, dbField, sslConnectionString(conf.tls))

	conn, err := pq.NewConnector(connStr)
	if err != nil {
		return nil, err
	}

	db := sql.OpenDB(conn)

	return &postgreSQLClient{
		client:   db,
		database: conf.database,
	}, nil
}

func (c *postgreSQLClient) Close() error {
	return c.client.Close()
}

type databaseStats struct {
	transactionCommitted int64
	transactionRollback  int64
}

func (c *postgreSQLClient) getVersion(ctx context.Context) (*version.Version, error) {
	query := `SHOW server_version;`
	row := c.client.QueryRowContext(ctx, query)
	var versionString string
	err := row.Scan(&versionString)
	if err != nil {
		return nil, err
	}
	// some results of show semver_version suffix with a clarifier such as "Debian" dependent on OS
	// so just grab the parseable semver ex) 13.2 Debian
	semverString := strings.Split(versionString, " ")[0]
	return version.NewVersion(semverString)
}

func (c *postgreSQLClient) getDatabaseStats(ctx context.Context, databases []string) (map[databaseName]databaseStats, error) {
	query := filterQueryByDatabases("SELECT datname, xact_commit, xact_rollback FROM pg_stat_database", databases, false)
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	var errs error
	dbStats := map[databaseName]databaseStats{}
	for rows.Next() {
		var datname string
		var transactionCommitted, transactionRollback int64
		err = rows.Scan(&datname, &transactionCommitted, &transactionRollback)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		if datname != "" {
			dbStats[databaseName(datname)] = databaseStats{
				transactionCommitted: transactionCommitted,
				transactionRollback:  transactionRollback,
			}
		}
	}
	return dbStats, errs
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
	table       string
	live        int64
	dead        int64
	inserts     int64
	upd         int64
	del         int64
	hotUpd      int64
	size        int64
	vacuumCount int64
}

func (c *postgreSQLClient) getDatabaseTableMetrics(ctx context.Context, db string) (map[tableIdentifier]tableStats, error) {
	query := `SELECT schemaname || '.' || relname AS table,
	n_live_tup AS live,
	n_dead_tup AS dead,
	n_tup_ins AS ins,
	n_tup_upd AS upd,
	n_tup_del AS del,
	n_tup_hot_upd AS hot_upd,
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
		var table string
		var live, dead, ins, upd, del, hotUpd, tableSize, vacuumCount int64
		err = rows.Scan(&table, &live, &dead, &ins, &upd, &del, &hotUpd, &tableSize, &vacuumCount)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		ts[tableKey(db, table)] = tableStats{
			database:    db,
			table:       table,
			live:        live,
			inserts:     ins,
			upd:         upd,
			del:         del,
			hotUpd:      hotUpd,
			size:        tableSize,
			vacuumCount: vacuumCount,
		}
	}
	return ts, errors
}

type tableIOStats struct {
	database  string
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
	query := `SELECT schemaname || '.' || relname AS table, 
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
		var table string
		var heapRead, heapHit, idxRead, idxHit, toastRead, toastHit, tidxRead, tidxHit int64
		err = rows.Scan(&table, &heapRead, &heapHit, &idxRead, &idxHit, &toastRead, &toastHit, &tidxRead, &tidxHit)
		if err != nil {
			errors = multierr.Append(errors, err)
			continue
		}
		tios[tableKey(db, table)] = tableIOStats{
			database:  db,
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
	database string
	size     int64
	scans    int64
}

func (c *postgreSQLClient) getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error) {
	query := `SELECT relname, indexrelname,
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
			table, index          string
			indexSize, indexScans int64
		)
		err := rows.Scan(&table, &index, &indexSize, &indexScans)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		stats[indexKey(database, table, index)] = indexStat{
			index:    index,
			table:    table,
			database: database,
			size:     indexSize,
			scans:    indexScans,
		}
	}
	return stats, multierr.Combine(errs...)
}

// queryStat is a representation of a row for the pg_stat_statements table,
// more info can be found here https://www.postgresql.org/docs/current/pgstatstatements.html
type queryStat struct {
	queryID   string
	queryText string

	meanExecTimeMs  float64
	totalExecTimeMs float64
	calls           int64

	sharedBlocksRead    int64
	sharedBlocksWritten int64
	sharedBlocksDirtied int64

	localBlocksRead    int64
	localBlocksWritten int64
	localBlocksDirtied int64

	tempBlocksRead    int64
	tempBlocksWritten int64
}

func (c *postgreSQLClient) getQueryStats(ctx context.Context, pgVersion *version.Version) ([]queryStat, error) {
	// negative queryid's are indicative of internal queries, which users most likely do not care about.
	query, err := c.perfQueryStatement(pgVersion)
	if err != nil {
		return []queryStat{}, err
	}
	qs := []queryStat{}
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return qs, err
	}

	var errors []error
	for rows.Next() {
		var (
			queryID, query                                             string
			meanExecTime, totalExecTime                                float64
			callCount                                                  int64
			sharedBlocksRead, sharedBlocksWritten, sharedBlocksDirtied int64
			localBlocksRead, localBlocksWritten, localBlocksDirtied    int64
			tempBlocksRead, tempBlocksWritten                          int64
		)
		err := rows.Scan(
			&queryID, &query, &meanExecTime, &totalExecTime, &callCount,
			&sharedBlocksRead, &sharedBlocksWritten, &sharedBlocksDirtied,
			&localBlocksRead, &localBlocksWritten, &localBlocksDirtied,
			&tempBlocksRead, &tempBlocksWritten,
		)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		qs = append(qs, queryStat{
			queryID:             queryID,
			queryText:           query,
			meanExecTimeMs:      meanExecTime,
			totalExecTimeMs:     totalExecTime,
			calls:               callCount,
			sharedBlocksRead:    sharedBlocksRead,
			sharedBlocksWritten: sharedBlocksWritten,
			sharedBlocksDirtied: sharedBlocksDirtied,
			localBlocksRead:     localBlocksRead,
			localBlocksWritten:  localBlocksWritten,
			localBlocksDirtied:  localBlocksDirtied,
			tempBlocksRead:      tempBlocksRead,
			tempBlocksWritten:   tempBlocksWritten,
		})
	}
	return qs, multierr.Combine(errors...)
}

func (c *postgreSQLClient) perfQueryStatement(pgVersion *version.Version) (string, error) {
	// version definitions defined here https://www.postgresql.org/docs/current/pgstatstatements.html
	pg10, _ := version.NewVersion("10.0")
	pg13, _ := version.NewVersion("13.0")

	var query string
	switch {
	// unsupported operation so just return here
	case pgVersion.LessThan(pg10):
		return "", fmt.Errorf("gathering performance data on queries is not supported for postgres version: %s", pgVersion.Original())
	// main difference is the change of mean_time => mean_exec_time from pg 10,11,12 => 13
	case pgVersion.GreaterThanOrEqual(pg10) && pgVersion.LessThan(pg13):
		query = `
		SELECT
			userid,
			queryid,
			query,
			mean_time,
			total_time,
			calls,
			shared_blks_read, shared_blks_written, shared_blks_dirtied,
			local_blks_read, local_blks_written, local_blks_dirtied,
			temp_blks_read, temp_blks_written
		FROM pg_stat_statements;
	`
	// default is a postgres instance version of > 13. Something to be conscious of if ends up changing.
	default:
		query = `SELECT
			userid,
			query,
			mean_exec_time,
			total_exec_time,
			calls,
			shared_blks_read, shared_blks_written, shared_blks_dirtied,
			local_blks_read, local_blks_written, local_blks_dirtied,
			temp_blks_read, temp_blks_written
		FROM pg_stat_statements;
	`
	}
	return query, nil
}

type bgStat struct {
	checkpointsReq       int64
	checkpointsScheduled int64
	checkpointWriteTime  int64
	checkpointSyncTime   int64
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
		checkpointSyncTime, checkpointWriteTime            int64
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

func (c *postgreSQLClient) listDatabases(ctx context.Context) ([]string, error) {
	query := `SELECT datname FROM pg_database
	WHERE datistemplate = false;`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	databases := []string{}
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
		queryDatabases := []string{}
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

func tableKey(database, table string) tableIdentifier {
	return tableIdentifier(fmt.Sprintf("%s|%s", database, table))
}

func indexKey(database, table, index string) indexIdentifer {
	return indexIdentifer(fmt.Sprintf("%s|%s|%s", database, table, index))
}
