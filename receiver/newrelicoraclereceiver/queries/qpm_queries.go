// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package queries

// Oracle SQL queries for performance metrics
const (
	SlowQueriesSQL = `
		WITH full_scans AS (
			-- Get a distinct list of SQL_IDs that have a full table scan in their plan
			SELECT DISTINCT sql_id
			FROM   v$sql_plan
			WHERE  operation = 'TABLE ACCESS' AND options = 'FULL'
		)
		SELECT
			d.name AS database_name,
			sa.sql_id AS query_id,
			sa.parsing_schema_name AS schema_name,
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
		CROSS JOIN
			v$database d
		LEFT JOIN
			full_scans fs ON sa.sql_id = fs.sql_id
		WHERE
			sa.executions > 0
			AND sa.sql_text NOT LIKE '%full_scans AS%'
		ORDER BY
			avg_elapsed_time_ms DESC
		FETCH FIRST 10 ROWS ONLY`

	// Oracle SQL query for individual queries metrics with user information
	IndividualQueriesFilteredSQL = `
		SELECT
		    a.sql_id AS query_id,
		    a.parsing_user_id AS user_id,
		    u.username AS username,
		    a.sql_text AS query_text,
		    a.cpu_time / 1000 AS cpu_time_ms,
		    a.elapsed_time / 1000 AS elapsed_time_ms,
		    'Multiple' AS hostname,
		    d.name AS database_name
		FROM
		    v$sql a
		JOIN
		    dba_users u ON a.parsing_user_id = u.user_id
		CROSS JOIN
		    v$database d
		WHERE
		    a.sql_id IN (%s)
		ORDER BY
		    elapsed_time_ms DESC`
	BlockingQueriesSQL = `
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
		FETCH FIRST 100 ROWS ONLY`
	// Oracle SQL query for wait event queries metrics
	WaitEventQueriesSQL = `
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
		FETCH FIRST 10 ROWS ONLY`
)
