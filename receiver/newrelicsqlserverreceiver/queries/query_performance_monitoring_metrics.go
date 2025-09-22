// // Copyright The OpenTelemetry Authors
// // SPDX-License-Identifier: Apache-2.0

// // Package queries provides SQL query definitions for performance monitoring.
// // This file contains all SQL queries related to SQL Server query performance monitoring.
// package queries

// // // Top N Slow Queries by Total Elapsed Time
// // var SlowQueriesByTotalTime = `
// // SELECT TOP (@topN)
// //     qs.query_hash,
// //     qs.query_plan_hash,
// //     qs.total_elapsed_time,
// //     qs.avg_elapsed_time,
// //     qs.execution_count,
// //     qs.total_worker_time AS total_cpu_time,
// //     qs.avg_worker_time AS avg_cpu_time,
// //     qs.total_logical_reads,
// //     qs.avg_logical_reads,
// //     qs.total_physical_reads,
// //     qs.avg_physical_reads,
// //     qs.total_logical_writes,
// //     qs.avg_logical_writes,
// //     qs.creation_time,
// //     qs.last_execution_time,
// //     SUBSTRING(st.text, (qs.statement_start_offset/2)+1,
// //         ((CASE qs.statement_end_offset
// //             WHEN -1 THEN DATALENGTH(st.text)
// //             ELSE qs.statement_end_offset
// //         END - qs.statement_start_offset)/2) + 1) AS query_text
// // FROM sys.dm_exec_query_stats qs
// // CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
// // ORDER BY qs.total_elapsed_time DESC`

// // // Top N Slow Queries by Average Elapsed Time
// // var SlowQueriesByAvgTime = `
// // SELECT TOP (@topN)
// //     qs.query_hash,
// //     qs.query_plan_hash,
// //     qs.total_elapsed_time,
// //     qs.avg_elapsed_time,
// //     qs.execution_count,
// //     qs.total_worker_time AS total_cpu_time,
// //     qs.avg_worker_time AS avg_cpu_time,
// //     qs.total_logical_reads,
// //     qs.avg_logical_reads,
// //     qs.total_physical_reads,
// //     qs.avg_physical_reads,
// //     qs.total_logical_writes,
// //     qs.avg_logical_writes,
// //     qs.creation_time,
// //     qs.last_execution_time,
// //     SUBSTRING(st.text, (qs.statement_start_offset/2)+1,
// //         ((CASE qs.statement_end_offset
// //             WHEN -1 THEN DATALENGTH(st.text)
// //             ELSE qs.statement_end_offset
// //         END - qs.statement_start_offset)/2) + 1) AS query_text
// // FROM sys.dm_exec_query_stats qs
// // CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
// // WHERE qs.execution_count > 5
// // ORDER BY qs.avg_elapsed_time DESC`

// // // Wait Statistics Query with Categorization
// // var WaitStatistics = `
// // SELECT 
// //     wait_type,
// //     CASE 
// //         WHEN wait_type LIKE 'PAGEIOLATCH%' OR wait_type LIKE 'WRITELOG%' OR wait_type LIKE 'IO_COMPLETION%' THEN 'I/O'
// //         WHEN wait_type LIKE 'LCK_%' OR wait_type LIKE 'LOCK_%' THEN 'Lock'
// //         WHEN wait_type LIKE 'SOS_SCHEDULER_YIELD%' OR wait_type LIKE 'THREADPOOL%' THEN 'CPU'
// //         WHEN wait_type LIKE 'NETWORK_%' OR wait_type LIKE 'ASYNC_NETWORK_%' THEN 'Network'
// //         WHEN wait_type LIKE 'RESOURCE_SEMAPHORE%' OR wait_type LIKE 'CMEMTHREAD%' THEN 'Memory'
// //         ELSE 'Other'
// //     END AS wait_category,
// //     waiting_tasks_count,
// //     wait_time_ms,
// //     max_wait_time_ms,
// //     signal_wait_time_ms,
// //     wait_time_ms - signal_wait_time_ms AS resource_wait_time_ms,
// //     CAST(100.0 * wait_time_ms / SUM(wait_time_ms) OVER() AS DECIMAL(5,2)) AS percentage_total
// // FROM sys.dm_os_wait_stats
// // WHERE wait_type NOT IN (
// //     'CLR_SEMAPHORE', 'LAZYWRITER_SLEEP', 'RESOURCE_QUEUE', 'SLEEP_TASK',
// //     'SLEEP_SYSTEMTASK', 'SQLTRACE_BUFFER_FLUSH', 'WAITFOR', 'LOGMGR_QUEUE',
// //     'CHECKPOINT_QUEUE', 'REQUEST_FOR_DEADLOCK_SEARCH', 'XE_TIMER_EVENT',
// //     'BROKER_TO_FLUSH', 'BROKER_TASK_STOP', 'CLR_MANUAL_EVENT', 'CLR_AUTO_EVENT',
// //     'DISPATCHER_QUEUE_SEMAPHORE', 'FT_IFTS_SCHEDULER_IDLE_WAIT', 'XE_DISPATCHER_WAIT'
// // )
// // AND wait_time_ms > 0
// // ORDER BY wait_time_ms DESC`

// // // Active Blocking Sessions Query
// // var BlockingSessions = `
// // SELECT 
// //     blocked.session_id AS blocked_session_id,
// //     blocked.blocking_session_id,
// //     blocked.wait_type,
// //     blocked.wait_resource,
// //     blocked.wait_time,
// //     blocked_session.login_name AS blocked_login_name,
// //     blocking_session.login_name AS blocking_login_name,
// //     blocked_session.host_name AS blocked_host_name,
// //     blocking_session.host_name AS blocking_host_name,
// //     blocked_session.program_name AS blocked_program_name,
// //     blocking_session.program_name AS blocking_program_name,
// //     blocked.command AS blocked_command,
// //     blocking.command AS blocking_command,
// //     blocked.status AS blocked_status,
// //     blocking.status AS blocking_status,
// //     CASE WHEN deadlock.session_id IS NOT NULL THEN 1 ELSE 0 END AS is_deadlock
// // FROM sys.dm_exec_requests blocked
// // LEFT JOIN sys.dm_exec_requests blocking ON blocked.blocking_session_id = blocking.session_id
// // LEFT JOIN sys.dm_exec_sessions blocked_session ON blocked.session_id = blocked_session.session_id
// // LEFT JOIN sys.dm_exec_sessions blocking_session ON blocking.session_id = blocking_session.session_id
// // LEFT JOIN sys.dm_exec_requests deadlock ON deadlock.blocking_session_id = blocked.session_id 
// //     AND blocked.blocking_session_id = deadlock.session_id
// // WHERE blocked.blocking_session_id <> 0
// // ORDER BY blocked.wait_time DESC`

// // // Execution Plan Cache Statistics
// // var ExecutionPlanCache = `
// // SELECT 
// //     cp.objtype AS plan_type,
// //     cp.cacheobjtype AS cache_object_type,
// //     cp.usecounts AS use_counts,
// //     cp.size_in_bytes,
// //     DATEDIFF(minute, cp.plan_generation_num, GETDATE()) AS plan_age_minutes,
// //     qs.creation_time,
// //     qs.last_execution_time,
// //     CASE 
// //         WHEN cp.objtype = 'Adhoc' AND cp.usecounts = 1 THEN 1 
// //         ELSE 0 
// //     END AS is_single_use_plan
// // FROM sys.dm_exec_cached_plans cp
// // LEFT JOIN sys.dm_exec_query_stats qs ON cp.plan_handle = qs.plan_handle
// // WHERE cp.cacheobjtype = 'Compiled Plan'
// // ORDER BY cp.size_in_bytes DESC`

// // // Query to get execution plans for slow queries
// // var SlowQueryExecutionPlans = `
// // SELECT 
// //     qs.query_hash,
// //     qs.query_plan_hash,
// //     qs.total_elapsed_time,
// //     qs.execution_count,
// //     qp.query_plan,
// //     SUBSTRING(st.text, (qs.statement_start_offset/2)+1,
// //         ((CASE qs.statement_end_offset
// //             WHEN -1 THEN DATALENGTH(st.text)
// //             ELSE qs.statement_end_offset
// //         END - qs.statement_start_offset)/2) + 1) AS query_text
// // FROM sys.dm_exec_query_stats qs
// // CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
// // CROSS APPLY sys.dm_exec_query_plan(qs.plan_handle) qp
// // WHERE qs.total_elapsed_time > @threshold_ms * 1000
// // ORDER BY qs.total_elapsed_time DESC`

// // // Currently executing queries with wait information
// // var CurrentlyExecutingQueries = `
// // SELECT 
// //     r.session_id,
// //     r.request_id,
// //     r.start_time,
// //     r.status,
// //     r.command,
// //     r.wait_type,
// //     r.wait_time,
// //     r.last_wait_type,
// //     r.wait_resource,
// //     r.cpu_time,
// //     r.total_elapsed_time,
// //     r.reads,
// //     r.writes,
// //     r.logical_reads,
// //     s.login_name,
// //     s.host_name,
// //     s.program_name,
// //     SUBSTRING(st.text, (r.statement_start_offset/2)+1,
// //         ((CASE r.statement_end_offset
// //             WHEN -1 THEN DATALENGTH(st.text)
// //             ELSE r.statement_end_offset
// //         END - r.statement_start_offset)/2) + 1) AS current_query
// // FROM sys.dm_exec_requests r
// // LEFT JOIN sys.dm_exec_sessions s ON r.session_id = s.session_id
// // CROSS APPLY sys.dm_exec_sql_text(r.sql_handle) st
// // WHERE r.session_id <> @@SPID
// // ORDER BY r.total_elapsed_time DESC`


// const (
//     SlowQuery = `DECLARE @IntervalSeconds INT = %d; 		-- Define the interval in seconds
// 				DECLARE @TopN INT = %d; 				-- Number of top queries to retrieve
// 				DECLARE @ElapsedTimeThreshold INT = %d; -- Elapsed time threshold in milliseconds
// 				DECLARE @TextTruncateLimit INT = %d; 	-- Truncate limit for query_text
				
// 				WITH RecentQueryIds AS (
// 					SELECT  
// 						qs.query_hash as query_id
// 					FROM 
// 						sys.dm_exec_query_stats qs
// 					WHERE 
// 						qs.execution_count > 0
// 						AND qs.last_execution_time >= DATEADD(SECOND, -@IntervalSeconds, GETUTCDATE())
// 						AND qs.sql_handle IS NOT NULL
// 				),
// 				QueryStats AS (
// 					SELECT
// 						qs.plan_handle,
// 						qs.sql_handle,
// 						LEFT(SUBSTRING(
// 							qt.text,
// 							(qs.statement_start_offset / 2) + 1,
// 							(
// 								CASE
// 									qs.statement_end_offset
// 									WHEN -1 THEN DATALENGTH(qt.text)
// 									ELSE qs.statement_end_offset
// 								END - qs.statement_start_offset
// 							) / 2 + 1
// 						), @TextTruncateLimit) AS query_text, 
// 						qs.query_hash AS query_id,
// 						qs.last_execution_time,
// 						qs.execution_count,
// 						(qs.total_worker_time / qs.execution_count) / 1000.0 AS avg_cpu_time_ms,
// 						(qs.total_elapsed_time / qs.execution_count) / 1000.0 AS avg_elapsed_time_ms,
// 						(qs.total_logical_reads / qs.execution_count) AS avg_disk_reads,
// 						(qs.total_logical_writes / qs.execution_count) AS avg_disk_writes,
// 						CASE
// 							WHEN UPPER(
// 								LTRIM(
// 									SUBSTRING(qt.text, (qs.statement_start_offset / 2) + 1, 6)
// 								)
// 							) LIKE 'SELECT' THEN 'SELECT'
// 							WHEN UPPER(
// 								LTRIM(
// 									SUBSTRING(qt.text, (qs.statement_start_offset / 2) + 1, 6)
// 								)
// 							) LIKE 'INSERT' THEN 'INSERT'
// 							WHEN UPPER(
// 								LTRIM(
// 									SUBSTRING(qt.text, (qs.statement_start_offset / 2) + 1, 6)
// 								)
// 							) LIKE 'UPDATE' THEN 'UPDATE'
// 							WHEN UPPER(
// 								LTRIM(
// 									SUBSTRING(qt.text, (qs.statement_start_offset / 2) + 1, 6)
// 								)
// 							) LIKE 'DELETE' THEN 'DELETE'
// 							ELSE 'OTHER'
// 						END AS statement_type,
// 						CONVERT(INT, pa.value) AS database_id,
// 						qt.objectid
// 					FROM
// 						sys.dm_exec_query_stats qs
// 						CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) AS qt
// 						JOIN sys.dm_exec_cached_plans cp ON qs.plan_handle = cp.plan_handle
// 						CROSS APPLY sys.dm_exec_plan_attributes(cp.plan_handle) AS pa
// 					WHERE
// 						qs.query_hash IN (SELECT DISTINCT(query_id) FROM RecentQueryIds)
// 						AND qs.execution_count > 0
// 						AND pa.attribute = 'dbid'
// 						AND DB_NAME(CONVERT(INT, pa.value)) NOT IN ('master', 'model', 'msdb', 'tempdb')
// 						AND qt.text NOT LIKE '%%sys.%%'
// 						AND qt.text NOT LIKE '%%INFORMATION_SCHEMA%%'
// 						AND qt.text NOT LIKE '%%schema_name()%%'
// 						AND qt.text IS NOT NULL
// 						AND LTRIM(RTRIM(qt.text)) <> ''
// 						AND EXISTS (
// 							SELECT 1
// 							FROM sys.databases d
// 							WHERE d.database_id = CONVERT(INT, pa.value) AND d.is_query_store_on = 1
// 						)
// 				)
// 				SELECT
// 					TOP (@TopN) qs.query_id,
// 					MIN(qs.query_text) AS query_text,
// 					DB_NAME(MIN(qs.database_id)) AS database_name,
// 					COALESCE(
// 						OBJECT_SCHEMA_NAME(MIN(qs.objectid), MIN(qs.database_id)),
// 						'N/A'
// 					) AS schema_name,
// 					FORMAT(
// 						MAX(qs.last_execution_time) AT TIME ZONE 'UTC',
// 						'yyyy-MM-ddTHH:mm:ssZ'
// 					) AS last_execution_timestamp,
// 					SUM(qs.execution_count) AS execution_count,
// 					AVG(qs.avg_cpu_time_ms) AS avg_cpu_time_ms,
// 					AVG(qs.avg_elapsed_time_ms) AS avg_elapsed_time_ms,
// 					AVG(qs.avg_disk_reads) AS avg_disk_reads,
// 					AVG(qs.avg_disk_writes) AS avg_disk_writes,
// 					 MAX(qs.statement_type) AS statement_type,
// 					FORMAT(
// 						SYSDATETIMEOFFSET() AT TIME ZONE 'UTC',
// 						'yyyy-MM-ddTHH:mm:ssZ'
// 					) AS collection_timestamp
// 				FROM
// 					QueryStats qs
// 				GROUP BY
// 					qs.query_id
// 				HAVING
// 					AVG(qs.avg_elapsed_time_ms) > @ElapsedTimeThreshold
// 				ORDER BY
// 					avg_elapsed_time_ms DESC;
//     `
// )


package queries

const SlowQueriesQuery = `
SELECT TOP 50
    CONVERT(varchar(100), qs.query_hash, 1) AS query_hash,
    CONVERT(varchar(100), qs.query_plan_hash, 1) AS query_plan_hash,
    qs.execution_count,
    qs.total_elapsed_time,
    CASE 
        WHEN qs.execution_count > 0 
        THEN qs.total_elapsed_time / qs.execution_count 
        ELSE 0 
    END AS avg_elapsed_time,
    qs.total_worker_time AS total_cpu_time,
    CASE 
        WHEN qs.execution_count > 0 
        THEN qs.total_worker_time / qs.execution_count 
        ELSE 0 
    END AS avg_cpu_time,
    qs.total_logical_reads,
    CASE 
        WHEN qs.execution_count > 0 
        THEN qs.total_logical_reads / qs.execution_count 
        ELSE 0 
    END AS avg_logical_reads,
    qs.total_physical_reads,
    CASE 
        WHEN qs.execution_count > 0 
        THEN qs.total_physical_reads / qs.execution_count 
        ELSE 0 
    END AS avg_physical_reads,
    qs.total_logical_writes AS total_writes,
    CASE 
        WHEN qs.execution_count > 0 
        THEN qs.total_logical_writes / qs.execution_count 
        ELSE 0 
    END AS avg_writes,
    SUBSTRING(qt.text, 1, 100) AS query_text_truncated
FROM sys.dm_exec_query_stats qs
CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) qt
WHERE qs.execution_count > 0
ORDER BY qs.total_elapsed_time DESC;`

const BlockingSessionsQuery = `
DECLARE @Limit INT = 50; -- Define the limit for the number of rows returned
DECLARE @TextTruncateLimit INT = 1000; -- Define the truncate limit for the query text
WITH blocking_info AS (
    SELECT
        req.blocking_session_id AS blocking_spid,
        req.session_id AS blocked_spid,
        req.wait_type AS wait_type,
        req.wait_time / 1000.0 AS wait_time_in_seconds,
        req.start_time AS start_time,
        sess.status AS status,
        req.command AS command_type,
        req.database_id AS database_id,
        req.sql_handle AS blocked_sql_handle,
        blocking_req.sql_handle AS blocking_sql_handle,
        blocking_req.start_time AS blocking_start_time
    FROM
        sys.dm_exec_requests AS req
    LEFT JOIN sys.dm_exec_requests AS blocking_req ON blocking_req.session_id = req.blocking_session_id
    LEFT JOIN sys.dm_exec_sessions AS sess ON sess.session_id = req.session_id
    WHERE
        req.blocking_session_id != 0
)
SELECT TOP (@Limit)
    blocking_info.blocking_spid,
    blocking_sessions.status AS blocking_status,
    blocking_info.blocked_spid,
    blocked_sessions.status AS blocked_status,
    blocking_info.wait_type,
    blocking_info.wait_time_in_seconds,
    blocking_info.command_type,
    blocking_info.start_time AS blocked_query_start_time,
    DB_NAME(blocking_info.database_id) AS database_name,
    CASE
        WHEN blocking_sql.text IS NULL THEN LEFT(input_buffer.event_info, @TextTruncateLimit)
        ELSE LEFT(blocking_sql.text, @TextTruncateLimit)
    END AS blocking_query_text,
    LEFT(blocked_sql.text, @TextTruncateLimit) AS blocked_query_text -- Truncate blocked query text
FROM
    blocking_info
JOIN sys.dm_exec_sessions AS blocking_sessions ON blocking_sessions.session_id = blocking_info.blocking_spid
JOIN sys.dm_exec_sessions AS blocked_sessions ON blocked_sessions.session_id = blocking_info.blocked_spid
OUTER APPLY sys.dm_exec_sql_text(blocking_info.blocking_sql_handle) AS blocking_sql
OUTER APPLY sys.dm_exec_sql_text(blocking_info.blocked_sql_handle) AS blocked_sql
OUTER APPLY sys.dm_exec_input_buffer(blocking_info.blocking_spid, NULL) AS input_buffer
JOIN sys.databases AS db ON db.database_id = blocking_info.database_id
WHERE db.is_query_store_on = 1
ORDER BY
    blocking_info.start_time;`