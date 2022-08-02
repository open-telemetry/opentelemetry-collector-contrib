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
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/multierr"
)

var errNoLastArchive = errors.New("no last archive found, not able to calculate oldest WAL age")

type client interface {
	Close() error
	getCommitsAndRollbacks(ctx context.Context, databases []string) ([]MetricStat, error)
	getBackends(ctx context.Context, databases []string) ([]MetricStat, error)
	getDatabaseSize(ctx context.Context, databases []string) ([]MetricStat, error)
	getDatabaseTableMetrics(ctx context.Context) ([]TableMetrics, error)
	getBlocksReadByTable(ctx context.Context) ([]MetricStat, error)
	getBackgroundWriterStats(ctx context.Context) ([]MetricStat, error)
	getIndexStats(ctx context.Context, database string) (*IndexStat, error)
	getQueryStats(ctx context.Context) ([]QueryStat, error)
	getReplicationDelay(ctx context.Context) (int64, error)
	getWALStats(ctx context.Context) (*walStats, error)
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

type MetricStat struct {
	database string
	table    string
	stats    map[string]string
}

func (c *postgreSQLClient) getCommitsAndRollbacks(ctx context.Context, databases []string) ([]MetricStat, error) {
	query := filterQueryByDatabases("SELECT datname, xact_commit, xact_rollback FROM pg_stat_database", databases, false)
	return c.collectStatsFromQuery(ctx, query, true, false, "xact_commit", "xact_rollback")
}

func (c *postgreSQLClient) getBackends(ctx context.Context, databases []string) ([]MetricStat, error) {
	query := filterQueryByDatabases("SELECT datname, count(*) as count from pg_stat_activity", databases, true)
	return c.collectStatsFromQuery(ctx, query, true, false, "count")
}

func (c *postgreSQLClient) getDatabaseSize(ctx context.Context, databases []string) ([]MetricStat, error) {
	query := filterQueryByDatabases("SELECT datname, pg_database_size(datname) FROM pg_catalog.pg_database WHERE datistemplate = false", databases, false)
	return c.collectStatsFromQuery(ctx, query, true, false, "db_size")
}

// TableMetrics contains a result for a row of the getDatabaseTableMetrics result
type TableMetrics struct {
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

func (c *postgreSQLClient) getDatabaseTableMetrics(ctx context.Context) ([]TableMetrics, error) {
	query := `SELECT schemaname || '.' || relname AS table,
	n_live_tup AS live,
	n_dead_tup AS dead,
	n_tup_ins AS ins,
	n_tup_upd AS upd,
	n_tup_del AS del,
	n_tup_hot_upd AS hot_upd,
	pg_relation_size(relid) AS table_size,
	vacuum_count AS vacuum_count
	FROM pg_stat_user_tables;`
	stats, err := c.collectStatsFromQuery(ctx, query, false, true, "live", "dead", "ins", "upd", "del", "hot_upd", "table_size", "vacuum_count")
	if err != nil {
		return nil, err
	}
	return c.mapTableMetrics(stats)
}

func (c *postgreSQLClient) mapTableMetrics(metricStats []MetricStat) ([]TableMetrics, error) {
	tms := []TableMetrics{}
	var errs error
	for _, ms := range metricStats {
		tm := TableMetrics{
			database: ms.database,
			table:    ms.table,
		}
		for mkey, v := range ms.stats {
			switch mkey {
			case "live":
				live, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.live = live
			case "dead":
				dead, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.dead = dead
			case "ins":
				ins, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.inserts = ins
			case "upd":
				upd, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.upd = upd
			case "del":
				deletes, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.del = deletes
			case "hot_upd":
				hotUpd, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.hotUpd = hotUpd
			case "table_size":
				tableSize, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.size = tableSize
			case "vacuum_count":
				vc, err := parseInt(v)
				errs = multierr.Append(errs, err)
				tm.vacuumCount = vc
			}
		}
		tms = append(tms, tm)
	}
	return tms, errs
}

func (c *postgreSQLClient) getBlocksReadByTable(ctx context.Context) ([]MetricStat, error) {
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

	return c.collectStatsFromQuery(ctx, query, false, true, "heap_read", "heap_hit", "idx_read", "idx_hit", "toast_read", "toast_hit", "tidx_read", "tidx_hit")
}

// IndexStat holds the statistics information for a particular index of a database
type IndexStat struct {
	database   string
	indexStats map[string]indexStatHolder
}

type indexStatHolder struct {
	size  int64
	scans int64
	index string
	table string
}

// getIndexStats requires a db client in
func (c *postgreSQLClient) getIndexStats(ctx context.Context, database string) (*IndexStat, error) {
	query := `SELECT relname, indexrelname,
	pg_relation_size(indexrelid) AS index_size,
	idx_scan
	FROM pg_stat_user_indexes;
	`

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var errs []error
	stats := IndexStat{
		database:   database,
		indexStats: make(map[string]indexStatHolder),
	}

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
		stats.indexStats[index] = indexStatHolder{
			size:  indexSize,
			index: index,
			table: table,
			scans: indexScans,
		}
	}
	return &stats, multierr.Combine(errs...)
}

func (c *postgreSQLClient) getReplicationDelay(ctx context.Context) (int64, error) {
	query := `SELECT
	CASE WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn()
	THEN 0 ELSE (coalesce(extract(epoch FROM now()) - extract(epoch FROM pg_last_xact_replay_timestamp()), 0))::int
	END AS estimated_replication_delay;
	`
	row := c.client.QueryRowContext(ctx, query)
	var replicationDelay int64
	err := row.Scan(&replicationDelay)
	if err != nil {
		return 0, err
	}
	return replicationDelay, nil
}

type replicationStats struct {
	client       string
	pendingBytes int64
	flushLag     int64
	replayLag    int64
	writeLag     int64
}

func (c *postgreSQLClient) getReplicationStats(ctx context.Context) ([]replicationStats, error) {
	query := `SELECT 
	client_addr,
	coalesce(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn), 0) AS replication_bytes_pending,
	write_lag,
	flush_lag,
	replay_lag
	FROM pg_stat_replication;
	`
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("unable to query pg_stat_replication: %w", err)
	}
	defer rows.Close()
	rs := []replicationStats{}
	for rows.Next() {
		var client string
		var replicationBytes, writeLag, flushLag, replayLag int64
		rows.Scan(&client, &replicationBytes, &writeLag, &flushLag, &replayLag)
		rs = append(rs, replicationStats{
			client:       client,
			pendingBytes: replicationBytes,
			replayLag:    replayLag,
			writeLag:     writeLag,
			flushLag:     flushLag,
		})
	}

	return rs, nil
}

type walStats struct {
	age int64
	lag walLag
}

type walLag struct {
	flushLag  int64
	replayLag int64
	writeLag  int64
}

func (c *postgreSQLClient) getWALStats(ctx context.Context) (*walStats, error) {
	query := `SELECT
	coalesce(last_archived_time, CURRENT_TIMESTAMP) AS last_archived_wal,
	CURRENT_TIMESTAMP,
	FROM pg_stat_archiver;
	`
	row := c.client.QueryRowContext(ctx, query)
	var lastArchivedWal, currentInstanceTime time.Time
	err := row.Scan(lastArchivedWal, currentInstanceTime)
	if err != nil {
		return nil, err
	}

	if lastArchivedWal.Equal(currentInstanceTime) {
		return nil, errNoLastArchive
	}

	age := int64(currentInstanceTime.Sub(lastArchivedWal).Seconds())
	return &walStats{
		age: age,
	}, nil
}

// QueryStat contain statistics about a particular query sourced from
type QueryStat struct {
	query           string
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

func (c *postgreSQLClient) getQueryStats(ctx context.Context) ([]QueryStat, error) {
	query := `SELECT
	"query",
	mean_exec_time,
	total_exec_time,
	calls,
	shared_blks_read, shared_blks_written, shared_blks_dirtied,
	local_blks_read, local_blks_written, local_blks_dirtied,
	temp_blks_read, temp_blks_written
	FROM pg_stat_statements(true);
	`

	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var errs []error
	qs := []QueryStat{}

	for rows.Next() {
		var (
			query                                              string
			meanExecTimeMs, totalExecTimeMs                    float64
			calls                                              int64
			sharedBlksRead, sharedBlksDirty, sharedBlksWritten int64
			localBlksRead, localBlksDirty, localBlksWritten    int64
			tempBlksRead, tempBlksWritten                      int64
		)
		err := rows.Scan(
			&query,
			&meanExecTimeMs,
			&totalExecTimeMs,
			&calls,
			&sharedBlksRead,
			&sharedBlksDirty,
			&sharedBlksWritten,
			&localBlksRead,
			&localBlksDirty,
			&localBlksWritten,
			&tempBlksRead,
			&tempBlksWritten,
		)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		qs = append(qs, QueryStat{
			query:               query,
			meanExecTimeMs:      meanExecTimeMs,
			totalExecTimeMs:     totalExecTimeMs,
			calls:               calls,
			sharedBlocksRead:    sharedBlksRead,
			sharedBlocksWritten: sharedBlksWritten,
			sharedBlocksDirtied: sharedBlksDirty,
			localBlocksRead:     localBlksRead,
			localBlocksWritten:  localBlksWritten,
			localBlocksDirtied:  localBlksDirty,
			tempBlocksRead:      tempBlksRead,
			tempBlocksWritten:   tempBlksWritten,
		})
	}
	return qs, multierr.Combine(errs...)
}

func (c *postgreSQLClient) getBackgroundWriterStats(ctx context.Context) ([]MetricStat, error) {
	// the pg_stat_bgwriter will have a single row containing global stats for the cluster
	query := `SELECT 
	coalesce(checkpoints_req, 0) AS checkpoint_req,
	coalesce(checkpoints_timed, 0) AS checkpoint_scheduled,
	coalesce(checkpoint_write_time, 0) AS checkpoint_duration_write,
	coalesce(checkpoint_sync_time, 0) AS checkpoint_duration_sync,
	coalesce(buffers_clean, 0) AS bg_writes,
	coalesce(buffers_backend, 0) AS backend_writes,
	coalesce(buffers_backend_fsync, 0) AS buffers_written_fsync,
	coalesce(buffers_checkpoint, 0) AS buffers_checkpoints,
	coalesce(buffers_alloc, 0) AS buffers_allocated,
	coalesce(maxwritten_clean, 0) AS maxwritten_count
	FROM pg_stat_bgwriter;`

	return c.collectStatsFromQuery(ctx, query, false, false,
		"checkpoint_req",
		"checkpoint_scheduled",
		"checkpoint_duration_write",
		"checkpoint_duration_sync",
		"bg_writes",
		"backend_writes",
		"buffers_written_fsync",
		"buffers_checkpoints",
		"buffers_allocated",
		"maxwritten_count",
	)
}

func (c *postgreSQLClient) collectStatsFromQuery(ctx context.Context, query string, includeDatabase bool, includeTable bool, orderedFields ...string) ([]MetricStat, error) {
	rows, err := c.client.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	errors := scrapererror.ScrapeErrors{}
	metricStats := []MetricStat{}
	for rows.Next() {
		rowFields := make([]interface{}, 0)

		// Build a list of addresses that rows.Scan will load column data into
		if includeDatabase {
			var val string
			rowFields = append(rowFields, &val)
		}
		if includeTable {
			var val string
			rowFields = append(rowFields, &val)
		}
		for range orderedFields {
			var val string
			rowFields = append(rowFields, &val)
		}

		if err := rows.Scan(rowFields...); err != nil {
			return nil, err
		}

		database := c.database
		if includeDatabase {
			v, err := convertInterfaceToString(rowFields[0])
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			database = v
			rowFields = rowFields[1:]
		}
		table := ""
		if includeTable {
			v, err := convertInterfaceToString(rowFields[0])
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			table = v
			rowFields = rowFields[1:]
		}

		stats := map[string]string{}
		for idx, val := range rowFields {
			v, err := convertInterfaceToString(val)
			if err != nil {
				errors.AddPartial(0, err)
				continue
			}
			stats[orderedFields[idx]] = v
		}

		metricStats = append(metricStats, MetricStat{
			database: database,
			table:    table,
			stats:    stats,
		})
	}
	return metricStats, errors.Combine()
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

func convertInterfaceToString(input interface{}) (string, error) {
	if val, ok := input.(*string); ok {
		return *val, nil
	}
	return "", errors.New("issue converting interface into string")
}
