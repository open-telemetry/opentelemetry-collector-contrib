// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

import (
	"fmt"
	"strings"
)

// Oracle SQL query for session count metric
const (
	SessionCountSQL = "SELECT COUNT(*) as SESSION_COUNT FROM v$session WHERE type = 'USER'"
)

// Oracle SQL query for system metrics from gv$sysmetric
const (
	SystemSysMetricsSQL = "SELECT INST_ID, METRIC_NAME, VALUE FROM gv$sysmetric"
)

// Oracle SQL query for tablespace metrics
const (
	TablespaceMetricsSQL = `
		SELECT a.TABLESPACE_NAME,
			a.USED_PERCENT,
			a.USED_SPACE * b.BLOCK_SIZE AS "USED",
			a.TABLESPACE_SIZE * b.BLOCK_SIZE AS "SIZE",
			b.TABLESPACE_OFFLINE AS "OFFLINE"
		FROM DBA_TABLESPACE_USAGE_METRICS a
		JOIN (
			SELECT
				TABLESPACE_NAME,
				BLOCK_SIZE,
				MAX( CASE WHEN status = 'OFFLINE' THEN 1 ELSE 0 END) AS "TABLESPACE_OFFLINE"
			FROM DBA_TABLESPACES
			GROUP BY TABLESPACE_NAME, BLOCK_SIZE
		) b
		ON a.TABLESPACE_NAME = b.TABLESPACE_NAME`

	GlobalNameTablespaceSQL = `
		SELECT
		t1.TABLESPACE_NAME,
		t2.GLOBAL_NAME
		FROM (SELECT TABLESPACE_NAME FROM DBA_TABLESPACES) t1,
		(SELECT GLOBAL_NAME FROM global_name) t2`

	DBIDTablespaceSQL = `
		SELECT
		t1.TABLESPACE_NAME,
		t2.DBID
		FROM (SELECT TABLESPACE_NAME FROM DBA_TABLESPACES) t1,
		(SELECT DBID FROM v$database) t2`

	CDBDatafilesOfflineTablespaceSQL = `
		SELECT
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE', 'SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS CDB_DATAFILES_OFFLINE,
			TABLESPACE_NAME
		FROM dba_data_files
		GROUP BY TABLESPACE_NAME`

	PDBDatafilesOfflineTablespaceSQL = `
		SELECT
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_DATAFILES_OFFLINE,
			a.TABLESPACE_NAME
		FROM cdb_data_files a, cdb_pdbs b
		WHERE a.con_id = b.con_id
		GROUP BY a.TABLESPACE_NAME`

	PDBNonWriteTablespaceSQL = `
		SELECT 
			TABLESPACE_NAME, 
			sum(CASE WHEN ONLINE_STATUS IN ('ONLINE','SYSTEM','RECOVER') THEN 0 ELSE 1 END) AS PDB_NON_WRITE_MODE
		FROM cdb_data_files a, cdb_pdbs b
		WHERE a.con_id = b.con_id
		GROUP BY TABLESPACE_NAME`

	// Core database metrics queries
	LockedAccountsSQL = `
		SELECT
			INST_ID, LOCKED_ACCOUNTS
		FROM
		(	SELECT count(1) AS LOCKED_ACCOUNTS
			FROM
				cdb_users a,
				cdb_pdbs b
			WHERE a.con_id = b.con_id
				AND a.account_status != 'OPEN'
		) l,
		gv$instance i`

	ReadWriteMetricsSQL = `
		SELECT
			INST_ID,
			SUM(PHYRDS) AS PhysicalReads,
			SUM(PHYWRTS) AS PhysicalWrites,
			SUM(PHYBLKRD) AS PhysicalBlockReads,
			SUM(PHYBLKWRT) AS PhysicalBlockWrites,
			SUM(READTIM) * 10 AS ReadTime,
			SUM(WRITETIM) * 10 AS WriteTime
		FROM gv$filestat
		GROUP BY INST_ID`

	PGAMetricsSQL = `
		SELECT INST_ID, NAME, VALUE 
		FROM gv$pgastat 
		WHERE NAME IN ('total PGA inuse', 'total PGA allocated', 'total freeable PGA memory', 'global memory bound')`

	GlobalNameInstanceSQL = `
		SELECT
			t1.INST_ID,
			t2.GLOBAL_NAME
		FROM
			(SELECT INST_ID FROM gv$instance) t1,
			(SELECT GLOBAL_NAME FROM global_name) t2`

	DBIDInstanceSQL = `
		SELECT
			t1.INST_ID,
			t2.DBID
		FROM (SELECT INST_ID FROM gv$instance) t1,
		(SELECT DBID FROM v$database) t2`

	LongRunningQueriesSQL = `
		SELECT inst_id, sum(num) AS total FROM ((
			SELECT i.inst_id, 1 AS num
			FROM gv$session s, gv$instance i
			WHERE i.inst_id=s.inst_id
			AND s.status='ACTIVE'
			AND s.type <>'BACKGROUND'
			AND s.last_call_et > 60
			GROUP BY i.inst_id
		) UNION (
			SELECT i.inst_id, 0 AS num
			FROM gv$session s, gv$instance i
			WHERE i.inst_id=s.inst_id
		))
		GROUP BY inst_id`

	SGAUGATotalMemorySQL = `
		SELECT SUM(value) AS sum, inst.inst_id
		FROM GV$sesstat, GV$statname, GV$INSTANCE inst
		WHERE name = 'session uga memory max'
		AND GV$sesstat.statistic#=GV$statname.statistic#
		AND GV$sesstat.inst_id=inst.inst_id
		AND GV$statname.inst_id=inst.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheShareableStatementSQL = `
		SELECT SUM(sqlarea.sharable_mem) AS sum, inst.inst_id
		FROM GV$sqlarea sqlarea, GV$INSTANCE inst
		WHERE sqlarea.executions > 5
		AND inst.inst_id=sqlarea.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheShareableUserSQL = `
		SELECT SUM(250 * sqlarea.users_opening) AS sum, inst.inst_id
		FROM GV$sqlarea sqlarea, GV$INSTANCE inst
		WHERE inst.inst_id=sqlarea.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheReloadRatioSQL = `
		SELECT (sum(libcache.reloads)/sum(libcache.pins)) AS ratio, inst.inst_id
		FROM GV$librarycache libcache, GV$INSTANCE inst
		WHERE inst.inst_id=libcache.inst_id
		GROUP BY inst.inst_id`

	SGASharedPoolLibraryCacheHitRatioSQL = `
		SELECT libcache.gethitratio as ratio, inst.inst_id
		FROM GV$librarycache libcache, GV$INSTANCE inst
		WHERE namespace='SQL AREA'
		AND inst.inst_id=libcache.inst_id`

	SGASharedPoolDictCacheMissRatioSQL = `
		SELECT (SUM(rcache.getmisses)/SUM(rcache.gets)) as ratio, inst.inst_id
		FROM GV$rowcache rcache, GV$INSTANCE inst
		WHERE inst.inst_id=rcache.inst_id
		GROUP BY inst.inst_id`

	SGALogBufferSpaceWaitsSQL = `
		SELECT count(wait.inst_id) as count, inst.inst_id
		FROM GV$SESSION_WAIT wait, GV$INSTANCE inst
		WHERE wait.event like 'log buffer space%'
		AND inst.inst_id=wait.inst_id
		GROUP BY inst.inst_id`

	SGALogAllocRetriesSQL = `
		SELECT (rbar.value/re.value) as ratio, inst.inst_id
		FROM GV$SYSSTAT rbar, GV$SYSSTAT re, GV$INSTANCE inst
		WHERE rbar.name like 'redo buffer allocation retries'
		AND re.name like 'redo entries'
		AND re.inst_id=inst.inst_id AND rbar.inst_id=inst.inst_id`

	SGAHitRatioSQL = `
		SELECT inst.inst_id,(1 - (phy.value - lob.value - dir.value)/ses.value) as ratio
		FROM GV$SYSSTAT ses, GV$SYSSTAT lob, GV$SYSSTAT dir, GV$SYSSTAT phy, GV$INSTANCE inst
		WHERE ses.name='session logical reads'
		AND dir.name='physical reads direct'
		AND lob.name='physical reads direct (lob)'
		AND phy.name='physical reads'
		AND ses.inst_id=inst.inst_id
		AND lob.inst_id=inst.inst_id
		AND dir.inst_id=inst.inst_id
		AND phy.inst_id=inst.inst_id`

	SysstatSQL = `
		SELECT inst.inst_id, sysstat.name, sysstat.value
		FROM GV$SYSSTAT sysstat, GV$INSTANCE inst
		WHERE sysstat.inst_id=inst.inst_id 
		AND sysstat.name IN ('redo buffer allocation retries', 'redo entries', 'sorts (memory)', 'sorts (disk)')`

	SGASQL = `
		SELECT inst.inst_id, sga.name, sga.value
		FROM GV$SGA sga, GV$INSTANCE inst
		WHERE sga.inst_id=inst.inst_id 
		AND sga.name IN ('Fixed Size', 'Redo Buffers')`

	RollbackSegmentsSQL = `
		SELECT
			SUM(stat.gets) AS gets,
			sum(stat.waits) AS waits,
			sum(stat.waits)/sum(stat.gets) AS ratio,
			inst.inst_id
		FROM GV$ROLLSTAT stat, GV$INSTANCE inst
		WHERE stat.inst_id=inst.inst_id
		GROUP BY inst.inst_id`

	RedoLogWaitsSQL = `
		SELECT
			sysevent.total_waits,
			inst.inst_id,
			sysevent.event
		FROM
			GV$SYSTEM_EVENT sysevent,
			GV$INSTANCE inst
		WHERE sysevent.inst_id=inst.inst_id
		AND (sysevent.event LIKE '%log file parallel write%'
		OR sysevent.event LIKE '%log file switch completion%'
		OR sysevent.event LIKE '%log file switch (check%'
		OR sysevent.event LIKE '%log file switch (arch%'
		OR sysevent.event LIKE '%buffer busy waits%'
		OR sysevent.event LIKE '%freeBufferWaits%'
		OR sysevent.event LIKE '%free buffer inspected%')`

	PDBSysMetricsSQL = `
		SELECT
			INST_ID,
			METRIC_NAME,
			VALUE
		FROM gv$con_sysmetric`

	// Individual query metrics SQL template
	IndividualQuerySQLBase = `
		SELECT
			sql_id,
			child_number,
			sql_fulltext,
			executions,
			elapsed_time / 1000 AS elapsed_time_ms
		FROM
			v$sql
		WHERE
			sql_fulltext LIKE '%%%s%%'
			AND parsing_schema_name NOT IN ('SYS', 'SYSTEM')
		ORDER BY
			elapsed_time DESC
		FETCH FIRST %d ROWS ONLY`
)

// IndividualQueryConfig represents individual query filtering configuration
type IndividualQueryConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	SearchText     string   `mapstructure:"search_text"`
	ExcludeSchemas []string `mapstructure:"exclude_schemas"`
	MaxQueries     int      `mapstructure:"max_queries"`
}

// BuildIndividualQuerySQL builds a dynamic individual query SQL based on configuration
func BuildIndividualQuerySQL(config IndividualQueryConfig) string {
	// Set default values
	searchText := config.SearchText
	if searchText == "" {
		searchText = "SELECT" // Default search text
	}

	maxQueries := config.MaxQueries
	if maxQueries <= 0 {
		maxQueries = 10 // Default
	}

	// Build the base query with search text and limit
	query := fmt.Sprintf(IndividualQuerySQLBase, searchText, maxQueries)

	// Add additional schema exclusions if specified
	if len(config.ExcludeSchemas) > 0 {
		additionalSchemas := strings.Join(config.ExcludeSchemas, "', '")
		query = strings.Replace(query, "NOT IN ('SYS', 'SYSTEM')",
			fmt.Sprintf("NOT IN ('SYS', 'SYSTEM', '%s')", additionalSchemas), 1)
	}

	return query
}
