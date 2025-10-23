// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/common-utils"
)

// validateAndCorrectThresholds validates QPM thresholds and returns corrected values
func validateAndCorrectThresholds(responseTimeMs, count int) (int, int) {
	if responseTimeMs < commonutils.MinQueryMonitoringResponseTimeThreshold || responseTimeMs > commonutils.MaxQueryMonitoringResponseTimeThreshold {
		responseTimeMs = commonutils.DefaultQueryMonitoringResponseTimeThreshold
	}
	if count < commonutils.MinQueryMonitoringCountThreshold || count > commonutils.MaxQueryMonitoringCountThreshold {
		count = commonutils.DefaultQueryMonitoringCountThreshold
	}
	return responseTimeMs, count
}

// Oracle SQL queries for performance metrics
const (
	slowQueriesBaseSQL = `
		WITH full_scans AS (
			SELECT DISTINCT sql_id
			FROM   v$sql_plan
			WHERE  operation = 'TABLE ACCESS' AND options = 'FULL'
		)
		SELECT
			d.name AS database_name,
			sa.sql_id AS query_id,
			sa.parsing_schema_name AS schema_name,
			au.username AS user_name,
			TO_CHAR(sa.last_load_time, 'YYYY-MM-DD HH24:MI:SS') AS last_load_time,
			sa.sharable_mem AS sharable_memory_bytes,
			sa.persistent_mem AS persistent_memory_bytes,
			sa.runtime_mem AS runtime_memory_bytes,
			COALESCE(sa.module,
				CASE
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'SELECT%' THEN 'SELECT'
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'INSERT%' THEN 'INSERT'
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'UPDATE%' THEN 'UPDATE'
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'DELETE%' THEN 'DELETE'
					ELSE 'OTHER'
				END
			) AS statement_type,
			sa.executions AS execution_count,
			sa.sql_text AS query_text,
			sa.cpu_time / DECODE(sa.executions, 0, 1, sa.executions) / 1000 AS avg_cpu_time_ms,
			sa.disk_reads / DECODE(sa.executions, 0, 1, sa.executions) AS avg_disk_reads,
			sa.direct_writes / DECODE(sa.executions, 0, 1, sa.executions) AS avg_disk_writes,
			sa.elapsed_time / DECODE(sa.executions, 0, 1, sa.executions) / 1000 AS avg_elapsed_time_ms,
			CASE WHEN fs.sql_id IS NOT NULL THEN 'Yes' ELSE 'No' END AS has_full_table_scan
		FROM
			v$sqlarea sa
		INNER JOIN
			ALL_USERS au ON sa.parsing_user_id = au.user_id
		CROSS JOIN
			v$database d
		LEFT JOIN
			full_scans fs ON sa.sql_id = fs.sql_id
		WHERE
			sa.executions > 0
			AND sa.sql_text NOT LIKE '%full_scans AS%'
			AND sa.sql_text NOT LIKE '%ALL_USERS%'
		ORDER BY
			avg_elapsed_time_ms DESC
		FETCH FIRST %d ROWS ONLY`

	blockingQueriesBaseSQL = `
		SELECT
			s2.sid AS blocked_sid,
			s2.serial# AS blocked_serial,
			s2.username AS blocked_user,
			s2.seconds_in_wait AS blocked_wait_sec,
			s2.sql_id AS blocked_sql_id,
			blocked_sql.sql_text AS blocked_query_text,
			s1.sid AS blocking_sid,
			s1.serial# AS blocking_serial,
			s1.username AS blocking_user,
			d.name AS database_name
		FROM
			v$session s2
		JOIN
			v$session s1 ON s2.blocking_session = s1.sid
		LEFT JOIN
			v$sql blocked_sql ON s2.sql_id = blocked_sql.sql_id
		CROSS JOIN
			v$database d
		WHERE
			s2.blocking_session IS NOT NULL
		ORDER BY
			s2.seconds_in_wait DESC
		FETCH FIRST %d ROWS ONLY`

	waitEventQueriesBaseSQL = `
		SELECT
			d.name AS database_name,
			ash.sql_id AS query_id,
			ash.wait_class AS wait_category,
			ash.event AS wait_event_name,
			SYSTIMESTAMP AS collection_timestamp,
			COUNT(DISTINCT ash.session_id || ',' || ash.session_serial#) AS waiting_tasks_count,
			ROUND(
				(SUM(ash.time_waited) / 1000) +
				(SUM(CASE WHEN ash.time_waited = 0 THEN 1 ELSE 0 END) * 1000)
			) AS total_wait_time_ms,
			ROUND(
				SUM(ash.time_waited) / NULLIF(COUNT(*), 0) / 1000, 2
			) AS avg_wait_time_ms
		FROM
			v$active_session_history ash
		CROSS JOIN
			v$database d
		WHERE
			ash.sql_id IS NOT NULL
			AND ash.wait_class <> 'Idle'
			AND ash.event IS NOT NULL
			AND ash.sample_time >= SYSDATE - INTERVAL '5' MINUTE
		GROUP BY
			d.name,
			ash.sql_id,
			ash.wait_class,
			ash.event
		ORDER BY
			total_wait_time_ms DESC
		FETCH FIRST %d ROWS ONLY`
)

// GetSlowQueriesSQL returns parameterized SQL for slow queries with configurable thresholds
func GetSlowQueriesSQL(responseTimeThresholdMs, countThreshold int) (string, []interface{}) {
	responseTimeThresholdMs, countThreshold = validateAndCorrectThresholds(responseTimeThresholdMs, countThreshold)

	var params []interface{}
	baseQuery := `
		WITH full_scans AS (
			SELECT DISTINCT sql_id
			FROM   v$sql_plan
			WHERE  operation = 'TABLE ACCESS' AND options = 'FULL'
		)
		SELECT
			d.name AS database_name,
			sa.sql_id AS query_id,
			sa.parsing_schema_name AS schema_name,
			au.username AS user_name,
			TO_CHAR(sa.last_load_time, 'YYYY-MM-DD HH24:MI:SS') AS last_load_time,
			sa.sharable_mem AS sharable_memory_bytes,
			sa.persistent_mem AS persistent_memory_bytes,
			sa.runtime_mem AS runtime_memory_bytes,
			COALESCE(sa.module,
				CASE
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'SELECT%' THEN 'SELECT'
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'INSERT%' THEN 'INSERT'
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'UPDATE%' THEN 'UPDATE'
					WHEN UPPER(LTRIM(sa.sql_text)) LIKE 'DELETE%' THEN 'DELETE'
					ELSE 'OTHER'
				END
			) AS statement_type,
			sa.executions AS execution_count,
			sa.sql_text AS query_text,
			sa.cpu_time / DECODE(sa.executions, 0, 1, sa.executions) / 1000 AS avg_cpu_time_ms,
			sa.disk_reads / DECODE(sa.executions, 0, 1, sa.executions) AS avg_disk_reads,
			sa.direct_writes / DECODE(sa.executions, 0, 1, sa.executions) AS avg_disk_writes,
			sa.elapsed_time / DECODE(sa.executions, 0, 1, sa.executions) / 1000 AS avg_elapsed_time_ms,
			CASE WHEN fs.sql_id IS NOT NULL THEN 'Yes' ELSE 'No' END AS has_full_table_scan
		FROM
			v$sqlarea sa
		INNER JOIN
			ALL_USERS au ON sa.parsing_user_id = au.user_id
		CROSS JOIN
			v$database d
		LEFT JOIN
			full_scans fs ON sa.sql_id = fs.sql_id
		WHERE
			sa.executions > 0
			AND sa.sql_text NOT LIKE '%full_scans AS%'
			AND sa.sql_text NOT LIKE '%ALL_USERS%'`

	if responseTimeThresholdMs > 0 {
		baseQuery += `
			AND sa.elapsed_time / DECODE(sa.executions, 0, 1, sa.executions) / 1000 >= ?`
		params = append(params, responseTimeThresholdMs)
	}

	baseQuery += fmt.Sprintf(`
		ORDER BY
			avg_elapsed_time_ms DESC
		FETCH FIRST %d ROWS ONLY`, countThreshold)

	return baseQuery, params
}

// GetBlockingQueriesSQL returns parameterized SQL for blocking queries
func GetBlockingQueriesSQL(countThreshold int) (string, []interface{}) {
	_, countThreshold = validateAndCorrectThresholds(0, countThreshold)

	query := `
		SELECT
			s2.sid AS blocked_sid,
			s2.serial# AS blocked_serial,
			s2.username AS blocked_user,
			s2.seconds_in_wait AS blocked_wait_sec,
			s2.sql_id AS blocked_sql_id,
			blocked_sql.sql_text AS blocked_query_text,
			s1.sid AS blocking_sid,
			s1.serial# AS blocking_serial,
			s1.username AS blocking_user,
			d.name AS database_name
		FROM
			v$session s2
		JOIN
			v$session s1 ON s2.blocking_session = s1.sid
		LEFT JOIN
			v$sql blocked_sql ON s2.sql_id = blocked_sql.sql_id
		CROSS JOIN
			v$database d
		WHERE
			s2.blocking_session IS NOT NULL
		ORDER BY
			s2.seconds_in_wait DESC
		FETCH FIRST %d ROWS ONLY`

	return fmt.Sprintf(query, countThreshold), []interface{}{}
}

// GetWaitEventQueriesSQL returns parameterized SQL for wait events
func GetWaitEventQueriesSQL(countThreshold int) (string, []interface{}) {
	_, countThreshold = validateAndCorrectThresholds(0, countThreshold)

	query := `
		SELECT
			d.name AS database_name,
			ash.sql_id AS query_id,
			ash.wait_class AS wait_category,
			ash.event AS wait_event_name,
			SYSTIMESTAMP AS collection_timestamp,
			COUNT(DISTINCT ash.session_id || ',' || ash.session_serial#) AS waiting_tasks_count,
			ROUND(
				(SUM(ash.time_waited) / 1000) +
				(SUM(CASE WHEN ash.time_waited = 0 THEN 1 ELSE 0 END) * 1000)
			) AS total_wait_time_ms,
			ROUND(
				SUM(ash.time_waited) / NULLIF(COUNT(*), 0) / 1000, 2
			) AS avg_wait_time_ms
		FROM
			v$active_session_history ash
		CROSS JOIN
			v$database d
		WHERE
			ash.sql_id IS NOT NULL
			AND ash.wait_class <> 'Idle'
			AND ash.event IS NOT NULL
			AND ash.sample_time >= SYSDATE - INTERVAL '5' MINUTE
		GROUP BY
			d.name,
			ash.sql_id,
			ash.wait_class,
			ash.event
		ORDER BY
			total_wait_time_ms DESC
		FETCH FIRST %d ROWS ONLY`

	return fmt.Sprintf(query, countThreshold), []interface{}{}
}

// GetExecutionPlanQuery returns optimized SQL query to fetch execution plans for given SQL IDs
// Performance optimizations: limits plan steps, adds execution filters, batch processing support
func GetExecutionPlanQuery(sqlIDs []string) (string, []interface{}) {
	if len(sqlIDs) == 0 {
		return "", []interface{}{}
	}

	// Performance optimization: Limit to reasonable batch size
	const maxBatchSize = 5
	if len(sqlIDs) > maxBatchSize {
		sqlIDs = sqlIDs[:maxBatchSize]
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(sqlIDs))
	args := make([]interface{}, len(sqlIDs))
	for i, sqlID := range sqlIDs {
		placeholders[i] = "?"
		args[i] = sqlID
	}

	query := fmt.Sprintf(`
		SELECT
			d.name AS database_name,
			sa.sql_id AS query_id,
			sa.plan_hash_value AS plan_hash_value,
			XMLSERIALIZE(CONTENT 
				XMLELEMENT("execution_plan",
					XMLATTRIBUTES(
						sa.sql_id AS "sql_id",
						sa.plan_hash_value AS "plan_hash_value",
						sa.parsing_schema_name AS "parsing_schema",
						sa.parsing_user_id AS "parsing_user_id",
						TO_CHAR(sa.first_load_time, 'YYYY-MM-DD HH24:MI:SS') AS "first_load_time",
						TO_CHAR(sa.last_load_time, 'YYYY-MM-DD HH24:MI:SS') AS "last_load_time",
						sa.executions AS "total_executions",
						ROUND(sa.elapsed_time / 1000000, 3) AS "total_elapsed_seconds",
						ROUND(sa.elapsed_time / DECODE(sa.executions, 0, 1, sa.executions) / 1000, 2) AS "avg_elapsed_ms",
						sa.optimizer_mode AS "optimizer_mode",
						sa.optimizer_cost AS "optimizer_cost",
						sa.optimizer_env_hash_value AS "optimizer_env_hash",
						COALESCE(sa.outline_category, '') AS "outline_category",
						sa.is_replicable_plan AS "is_replicable",
						sa.is_adaptive_plan AS "is_adaptive_plan",
						sa.is_shareable AS "is_shareable"
					),
					XMLELEMENT("sql_text", sa.sql_text),
					XMLELEMENT("sql_fulltext", sa.sql_fulltext),
					XMLELEMENT("plan_statistics",
						XMLELEMENT("buffer_gets", sa.buffer_gets),
						XMLELEMENT("disk_reads", sa.disk_reads),
						XMLELEMENT("direct_writes", sa.direct_writes),
						XMLELEMENT("application_wait_time", sa.application_wait_time),
						XMLELEMENT("concurrency_wait_time", sa.concurrency_wait_time),
						XMLELEMENT("cluster_wait_time", sa.cluster_wait_time),
						XMLELEMENT("user_io_wait_time", sa.user_io_wait_time),
						XMLELEMENT("plsql_exec_time", sa.plsql_exec_time),
						XMLELEMENT("java_exec_time", sa.java_exec_time),
						XMLELEMENT("sorts", sa.sorts),
						XMLELEMENT("loads", sa.loads),
						XMLELEMENT("invalidations", sa.invalidations),
						XMLELEMENT("parse_calls", sa.parse_calls),
						XMLELEMENT("fetches", sa.fetches),
						XMLELEMENT("rows_processed", sa.rows_processed)
					),
					XMLELEMENT("memory_usage",
						XMLELEMENT("sharable_mem", sa.sharable_mem),
						XMLELEMENT("persistent_mem", sa.persistent_mem),
						XMLELEMENT("runtime_mem", sa.runtime_mem),
						XMLELEMENT("loaded_versions", sa.loaded_versions),
						XMLELEMENT("open_versions", sa.open_versions),
						XMLELEMENT("users_opening", sa.users_opening),
						XMLELEMENT("users_executing", sa.users_executing)
					),
					XMLELEMENT("bind_variables",
						XMLELEMENT("bind_aware", sa.bind_aware),
						XMLELEMENT("bind_sensitive", sa.bind_sensitive),
						XMLELEMENT("bind_equivalent", sa.bind_equivalent)
					),
					XMLELEMENT("plan_steps",
						XMLAGG(
							XMLELEMENT("step",
								XMLATTRIBUTES(
									sp.id AS "step_id",
									NVL(sp.parent_id, '') AS "parent_step_id",
									sp.depth AS "depth_level",
									sp.position AS "position"
								),
								XMLELEMENT("operation", sp.operation),
								XMLELEMENT("options", NVL(sp.options, '')),
								XMLELEMENT("object_node", NVL(sp.object_node, '')),
								XMLELEMENT("object",
									XMLATTRIBUTES(
										NVL(sp.object_owner, '') AS "owner",
										NVL(sp.object_name, '') AS "name",
										NVL(sp.object_alias, '') AS "alias",
										NVL(sp.object_instance, '') AS "instance",
										NVL(sp.object_type, '') AS "type"
									)
								),
								XMLELEMENT("optimizer", NVL(sp.optimizer, '')),
								XMLELEMENT("search_columns", NVL(sp.search_columns, 0)),
								XMLELEMENT("cost_info",
									XMLELEMENT("cost", NVL(sp.cost, 0)),
									XMLELEMENT("cardinality", NVL(sp.cardinality, 0)),
									XMLELEMENT("bytes", NVL(sp.bytes, 0)),
									XMLELEMENT("other_tag", NVL(sp.other_tag, '')),
									XMLELEMENT("partition_start", NVL(sp.partition_start, '')),
									XMLELEMENT("partition_stop", NVL(sp.partition_stop, '')),
									XMLELEMENT("partition_id", NVL(sp.partition_id, 0)),
									XMLELEMENT("other", NVL(sp.other, '')),
									XMLELEMENT("distribution", NVL(sp.distribution, '')),
									XMLELEMENT("cpu_cost", NVL(sp.cpu_cost, 0)),
									XMLELEMENT("io_cost", NVL(sp.io_cost, 0)),
									XMLELEMENT("temp_space", NVL(sp.temp_space, 0))
								),
								XMLELEMENT("predicates",
									XMLELEMENT("access_predicates", NVL(sp.access_predicates, '')),
									XMLELEMENT("filter_predicates", NVL(sp.filter_predicates, ''))
								),
								XMLELEMENT("projection", NVL(sp.projection, '')),
								XMLELEMENT("time", NVL(sp.time, 0)),
								XMLELEMENT("qblock_name", NVL(sp.qblock_name, '')),
								XMLELEMENT("remarks", NVL(sp.remarks, '')),
								XMLELEMENT("formatted_operation", 
									CASE 
										WHEN sp.depth = 0 THEN sp.operation || COALESCE(' (' || sp.options || ')', '')
										ELSE LPAD(' ', sp.depth * 2, ' ') || sp.operation || COALESCE(' (' || sp.options || ')', '') || 
											CASE WHEN sp.object_name IS NOT NULL THEN ' ON ' || NVL(sp.object_owner || '.', '') || sp.object_name END
									END
								)
							) ORDER BY sp.id
						)
					)
				)
			) AS execution_plan_xml
		FROM
			v$sqlarea sa
		CROSS JOIN
			v$database d
		LEFT JOIN
			v$sql_plan sp ON sa.sql_id = sp.sql_id AND sa.plan_hash_value = sp.plan_hash_value
		WHERE
			sa.sql_id IN (%s)
			AND sa.executions > 1          -- Only executed queries
			AND (sp.id IS NULL OR sp.id <= 15)  -- Limit plan steps for performance
			AND sa.last_load_time >= SYSDATE - INTERVAL '7' DAY  -- Recent queries only
		GROUP BY
			d.name,
			sa.sql_id,
			sa.plan_hash_value,
			sa.parsing_schema_name,
			sa.parsing_user_id,
			sa.first_load_time,
			sa.last_load_time,
			sa.executions,
			sa.elapsed_time,
			sa.optimizer_mode,
			sa.optimizer_cost,
			sa.optimizer_env_hash_value,
			sa.outline_category,
			sa.is_replicable_plan,
			sa.is_adaptive_plan,
			sa.is_shareable,
			sa.sql_text,
			sa.sql_fulltext,
			sa.buffer_gets,
			sa.disk_reads,
			sa.direct_writes,
			sa.application_wait_time,
			sa.concurrency_wait_time,
			sa.cluster_wait_time,
			sa.user_io_wait_time,
			sa.plsql_exec_time,
			sa.java_exec_time,
			sa.sorts,
			sa.loads,
			sa.invalidations,
			sa.parse_calls,
			sa.fetches,
			sa.rows_processed,
			sa.sharable_mem,
			sa.persistent_mem,
			sa.runtime_mem,
			sa.loaded_versions,
			sa.open_versions,
			sa.users_opening,
			sa.users_executing,
			sa.bind_aware,
			sa.bind_sensitive,
			sa.bind_equivalent
		ORDER BY
			sa.sql_id,
			sa.plan_hash_value`, strings.Join(placeholders, ","))

	return query, args
}
