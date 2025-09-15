// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for performance monitoring.
// This file contains all SQL queries related to SQL Server query performance monitoring.
package queries

// Top N Slow Queries by Total Elapsed Time
var SlowQueriesByTotalTime = `
SELECT TOP (@topN)
    qs.query_hash,
    qs.query_plan_hash,
    qs.total_elapsed_time,
    qs.avg_elapsed_time,
    qs.execution_count,
    qs.total_worker_time AS total_cpu_time,
    qs.avg_worker_time AS avg_cpu_time,
    qs.total_logical_reads,
    qs.avg_logical_reads,
    qs.total_physical_reads,
    qs.avg_physical_reads,
    qs.total_logical_writes,
    qs.avg_logical_writes,
    qs.creation_time,
    qs.last_execution_time,
    SUBSTRING(st.text, (qs.statement_start_offset/2)+1,
        ((CASE qs.statement_end_offset
            WHEN -1 THEN DATALENGTH(st.text)
            ELSE qs.statement_end_offset
        END - qs.statement_start_offset)/2) + 1) AS query_text
FROM sys.dm_exec_query_stats qs
CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
ORDER BY qs.total_elapsed_time DESC`

// Top N Slow Queries by Average Elapsed Time
var SlowQueriesByAvgTime = `
SELECT TOP (@topN)
    qs.query_hash,
    qs.query_plan_hash,
    qs.total_elapsed_time,
    qs.avg_elapsed_time,
    qs.execution_count,
    qs.total_worker_time AS total_cpu_time,
    qs.avg_worker_time AS avg_cpu_time,
    qs.total_logical_reads,
    qs.avg_logical_reads,
    qs.total_physical_reads,
    qs.avg_physical_reads,
    qs.total_logical_writes,
    qs.avg_logical_writes,
    qs.creation_time,
    qs.last_execution_time,
    SUBSTRING(st.text, (qs.statement_start_offset/2)+1,
        ((CASE qs.statement_end_offset
            WHEN -1 THEN DATALENGTH(st.text)
            ELSE qs.statement_end_offset
        END - qs.statement_start_offset)/2) + 1) AS query_text
FROM sys.dm_exec_query_stats qs
CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
WHERE qs.execution_count > 5
ORDER BY qs.avg_elapsed_time DESC`

// Wait Statistics Query with Categorization
var WaitStatistics = `
SELECT 
    wait_type,
    CASE 
        WHEN wait_type LIKE 'PAGEIOLATCH%' OR wait_type LIKE 'WRITELOG%' OR wait_type LIKE 'IO_COMPLETION%' THEN 'I/O'
        WHEN wait_type LIKE 'LCK_%' OR wait_type LIKE 'LOCK_%' THEN 'Lock'
        WHEN wait_type LIKE 'SOS_SCHEDULER_YIELD%' OR wait_type LIKE 'THREADPOOL%' THEN 'CPU'
        WHEN wait_type LIKE 'NETWORK_%' OR wait_type LIKE 'ASYNC_NETWORK_%' THEN 'Network'
        WHEN wait_type LIKE 'RESOURCE_SEMAPHORE%' OR wait_type LIKE 'CMEMTHREAD%' THEN 'Memory'
        ELSE 'Other'
    END AS wait_category,
    waiting_tasks_count,
    wait_time_ms,
    max_wait_time_ms,
    signal_wait_time_ms,
    wait_time_ms - signal_wait_time_ms AS resource_wait_time_ms,
    CAST(100.0 * wait_time_ms / SUM(wait_time_ms) OVER() AS DECIMAL(5,2)) AS percentage_total
FROM sys.dm_os_wait_stats
WHERE wait_type NOT IN (
    'CLR_SEMAPHORE', 'LAZYWRITER_SLEEP', 'RESOURCE_QUEUE', 'SLEEP_TASK',
    'SLEEP_SYSTEMTASK', 'SQLTRACE_BUFFER_FLUSH', 'WAITFOR', 'LOGMGR_QUEUE',
    'CHECKPOINT_QUEUE', 'REQUEST_FOR_DEADLOCK_SEARCH', 'XE_TIMER_EVENT',
    'BROKER_TO_FLUSH', 'BROKER_TASK_STOP', 'CLR_MANUAL_EVENT', 'CLR_AUTO_EVENT',
    'DISPATCHER_QUEUE_SEMAPHORE', 'FT_IFTS_SCHEDULER_IDLE_WAIT', 'XE_DISPATCHER_WAIT'
)
AND wait_time_ms > 0
ORDER BY wait_time_ms DESC`

// Active Blocking Sessions Query
var BlockingSessions = `
SELECT 
    blocked.session_id AS blocked_session_id,
    blocked.blocking_session_id,
    blocked.wait_type,
    blocked.wait_resource,
    blocked.wait_time,
    blocked_session.login_name AS blocked_login_name,
    blocking_session.login_name AS blocking_login_name,
    blocked_session.host_name AS blocked_host_name,
    blocking_session.host_name AS blocking_host_name,
    blocked_session.program_name AS blocked_program_name,
    blocking_session.program_name AS blocking_program_name,
    blocked.command AS blocked_command,
    blocking.command AS blocking_command,
    blocked.status AS blocked_status,
    blocking.status AS blocking_status,
    CASE WHEN deadlock.session_id IS NOT NULL THEN 1 ELSE 0 END AS is_deadlock
FROM sys.dm_exec_requests blocked
LEFT JOIN sys.dm_exec_requests blocking ON blocked.blocking_session_id = blocking.session_id
LEFT JOIN sys.dm_exec_sessions blocked_session ON blocked.session_id = blocked_session.session_id
LEFT JOIN sys.dm_exec_sessions blocking_session ON blocking.session_id = blocking_session.session_id
LEFT JOIN sys.dm_exec_requests deadlock ON deadlock.blocking_session_id = blocked.session_id 
    AND blocked.blocking_session_id = deadlock.session_id
WHERE blocked.blocking_session_id <> 0
ORDER BY blocked.wait_time DESC`

// Execution Plan Cache Statistics
var ExecutionPlanCache = `
SELECT 
    cp.objtype AS plan_type,
    cp.cacheobjtype AS cache_object_type,
    cp.usecounts AS use_counts,
    cp.size_in_bytes,
    DATEDIFF(minute, cp.plan_generation_num, GETDATE()) AS plan_age_minutes,
    qs.creation_time,
    qs.last_execution_time,
    CASE 
        WHEN cp.objtype = 'Adhoc' AND cp.usecounts = 1 THEN 1 
        ELSE 0 
    END AS is_single_use_plan
FROM sys.dm_exec_cached_plans cp
LEFT JOIN sys.dm_exec_query_stats qs ON cp.plan_handle = qs.plan_handle
WHERE cp.cacheobjtype = 'Compiled Plan'
ORDER BY cp.size_in_bytes DESC`

// Query to get execution plans for slow queries
var SlowQueryExecutionPlans = `
SELECT 
    qs.query_hash,
    qs.query_plan_hash,
    qs.total_elapsed_time,
    qs.execution_count,
    qp.query_plan,
    SUBSTRING(st.text, (qs.statement_start_offset/2)+1,
        ((CASE qs.statement_end_offset
            WHEN -1 THEN DATALENGTH(st.text)
            ELSE qs.statement_end_offset
        END - qs.statement_start_offset)/2) + 1) AS query_text
FROM sys.dm_exec_query_stats qs
CROSS APPLY sys.dm_exec_sql_text(qs.sql_handle) st
CROSS APPLY sys.dm_exec_query_plan(qs.plan_handle) qp
WHERE qs.total_elapsed_time > @threshold_ms * 1000
ORDER BY qs.total_elapsed_time DESC`

// Currently executing queries with wait information
var CurrentlyExecutingQueries = `
SELECT 
    r.session_id,
    r.request_id,
    r.start_time,
    r.status,
    r.command,
    r.wait_type,
    r.wait_time,
    r.last_wait_type,
    r.wait_resource,
    r.cpu_time,
    r.total_elapsed_time,
    r.reads,
    r.writes,
    r.logical_reads,
    s.login_name,
    s.host_name,
    s.program_name,
    SUBSTRING(st.text, (r.statement_start_offset/2)+1,
        ((CASE r.statement_end_offset
            WHEN -1 THEN DATALENGTH(st.text)
            ELSE r.statement_end_offset
        END - r.statement_start_offset)/2) + 1) AS current_query
FROM sys.dm_exec_requests r
LEFT JOIN sys.dm_exec_sessions s ON r.session_id = s.session_id
CROSS APPLY sys.dm_exec_sql_text(r.sql_handle) st
WHERE r.session_id <> @@SPID
ORDER BY r.total_elapsed_time DESC`
