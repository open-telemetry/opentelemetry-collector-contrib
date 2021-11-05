package postgresqlreceiver

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type client interface {
	Close() error
	getCommitsAndRollbacks(databases []string) ([]MetricStat, error)
	getBackends(databases []string) ([]MetricStat, error)
	getDatabaseSize(databases []string) ([]MetricStat, error)
	getDatabaseTableMetrics() ([]MetricStat, error)
	getBlocksReadByTable() ([]MetricStat, error)
	listDatabases() ([]string, error)
}

type postgreSQLClient struct {
	client   *sql.DB
	database string
}

var _ client = (*postgreSQLClient)(nil)

type postgreSQLConfig struct {
	username  string
	password  string
	database  string
	host      string
	port      int
	sslConfig SSLConfig
}

func newPostgreSQLClient(conf postgreSQLConfig) (*postgreSQLClient, error) {
	dbField := ""
	if conf.database != "" {
		dbField = fmt.Sprintf("dbname=%s", conf.database)
	}
	connStr := fmt.Sprintf("port=%d host=%s user=%s password=%s %s %s", conf.port, conf.host, conf.username, conf.password, dbField, conf.sslConfig.ConnString())

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

func (p *postgreSQLClient) getCommitsAndRollbacks(databases []string) ([]MetricStat, error) {
	query := filterQueryByDatabases("SELECT datname, xact_commit, xact_rollback FROM pg_stat_database", databases, false)

	return p.collectStatsFromQuery(query, []string{"xact_commit", "xact_rollback"}, true, false)
}

func (p *postgreSQLClient) getBackends(databases []string) ([]MetricStat, error) {
	query := filterQueryByDatabases("SELECT datname, count(*) as count from pg_stat_activity", databases, true)

	return p.collectStatsFromQuery(query, []string{"count"}, true, false)
}

func (p *postgreSQLClient) getDatabaseSize(databases []string) ([]MetricStat, error) {
	query := filterQueryByDatabases("SELECT datname, pg_database_size(datname) FROM pg_catalog.pg_database WHERE datistemplate = false", databases, false)

	return p.collectStatsFromQuery(query, []string{"db_size"}, true, false)
}

func (p *postgreSQLClient) getDatabaseTableMetrics() ([]MetricStat, error) {
	query := `SELECT schemaname || '.' || relname AS table,
	n_live_tup AS live,
	n_dead_tup AS dead,
	n_tup_ins AS ins,
	n_tup_upd AS upd,
	n_tup_del AS del,
	n_tup_hot_upd AS hot_upd
	FROM pg_stat_user_tables;`

	return p.collectStatsFromQuery(query, []string{"live", "dead", "ins", "upd", "del", "hot_upd"}, false, true)
}

func (p *postgreSQLClient) getBlocksReadByTable() ([]MetricStat, error) {
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

	return p.collectStatsFromQuery(query, []string{"heap_read", "heap_hit", "idx_read", "idx_hit", "toast_read", "toast_hit", "tidx_read", "tidx_hit"}, false, true)
}

func (p *postgreSQLClient) collectStatsFromQuery(query string, orderedFields []string, includeDatabase bool, includeTable bool) ([]MetricStat, error) {
	rows, err := p.client.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metricStats := []MetricStat{}
	for rows.Next() {
		rowFields := make([]interface{}, 0)

		// Build a list of addresses that rows.Scan will load column data into
		appendField := func(val string) {
			rowFields = append(rowFields, &val)
		}

		if includeDatabase {
			appendField("")
		}
		if includeTable {
			appendField("")
		}
		for range orderedFields {
			appendField("")
		}

		stats := map[string]string{}
		if err := rows.Scan(rowFields...); err != nil {
			return nil, err
		}

		convertInterfaceToString := func(input interface{}) string {
			if val, ok := input.(*string); ok {
				return *val
			}
			return ""
		}

		database := p.database
		if includeDatabase {
			database, rowFields = convertInterfaceToString(rowFields[0]), rowFields[1:]
		}
		table := ""
		if includeTable {
			table, rowFields = convertInterfaceToString(rowFields[0]), rowFields[1:]
		}
		for idx, val := range rowFields {
			stats[orderedFields[idx]] = convertInterfaceToString(val)
		}
		metricStats = append(metricStats, MetricStat{
			database: database,
			table:    table,
			stats:    stats,
		})
	}
	return metricStats, nil
}

func (p *postgreSQLClient) listDatabases() ([]string, error) {
	query := `SELECT datname FROM pg_database
	WHERE datistemplate = false;`
	rows, err := p.client.Query(query)
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
