// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sqlserverreceiver provides functionality for collecting metrics from SQL Server.
package sqlserverreceiver

import (
	"fmt"
	"strings"
)

// Direct access to queries is not recommended: The receiver allows filtering based on
// instance name, which means the query will change based on configuration.
// Please use getSQLServerDatabaseIOQuery

// sqlServerDatabaseIOQuery collects I/O statistics for each database file.
// This includes read/write latency, throughput, and stall metrics for data and log files.
const sqlServerDatabaseIOQuery = `
-- Set low priority to minimize impact on production workload
SET DEADLOCK_PRIORITY -10;

-- Check if this is a supported SQL Server edition (Standard, Enterprise, or Express)
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard,Enterprise,Express*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard,Enterprise or Express. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

-- Declare variables for version-specific features
DECLARE
	 @SqlStatement AS nvarchar(max)
	,@MajorMinorVersion AS int = CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),4) AS int) * 100 + CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),3) AS int)
	,@Columns AS nvarchar(max) = ''
	,@Tables AS nvarchar(max) = ''

-- Add Resource Governor metrics for SQL Server 2012 and later
IF @MajorMinorVersion > 1100 BEGIN
	SET @Columns += N'
	,vfs.[io_stall_queued_read_ms] AS [rg_read_stall_ms]    -- Read stalls from Resource Governor
	,vfs.[io_stall_queued_write_ms] AS [rg_write_stall_ms]  -- Write stalls from Resource Governor'
END

SET @SqlStatement = N'
SELECT
	''sqlserver_database_io'' AS [measurement]
	,REPLACE(@@SERVERNAME,''\'','':'') AS [sql_instance]     -- Instance name with standardized format
	,HOST_NAME() AS [computer_name]                          -- Host machine name
	,DB_NAME(vfs.[database_id]) AS [database_name]          -- Database name
	,COALESCE(mf.[physical_name],''RBPEX'') AS [physical_filename]  -- Physical file path or RBPEX for buffer pool extension
	,COALESCE(mf.[name],''RBPEX'') AS [logical_filename]    -- Logical file name or RBPEX for buffer pool extension
	,mf.[type_desc] AS [file_type]                          -- File type (ROWS, LOG, etc.)
	,vfs.[io_stall_read_ms] AS [read_latency_ms]           -- Total read latency
	,vfs.[num_of_reads] AS [reads]                          -- Number of read operations
	,vfs.[num_of_bytes_read] AS [read_bytes]                -- Total bytes read
	,vfs.[io_stall_write_ms] AS [write_latency_ms]         -- Total write latency
	,vfs.[num_of_writes] AS [writes]                        -- Number of write operations
	,vfs.[num_of_bytes_written] AS [write_bytes]'           -- Total bytes written
	+ @Columns + N'
FROM sys.dm_io_virtual_file_stats(NULL, NULL) AS vfs        -- DMV for file I/O statistics
INNER JOIN sys.master_files AS mf WITH (NOLOCK)             -- Join with file metadata
	ON vfs.[database_id] = mf.[database_id] AND vfs.[file_id] = mf.[file_id]
%s'                                                         -- Placeholder for instance filter
+ @Tables;

EXEC sp_executesql @SqlStatement
`

// getSQLServerDatabaseIOQuery returns a query to collect database I/O metrics.
// Parameters:
// - instanceName: SQL Server instance to monitor (empty string for default instance)
func getSQLServerDatabaseIOQuery(instanceName string) string {
	if instanceName != "" {
		whereClause := fmt.Sprintf("WHERE @@SERVERNAME = ''%s''", instanceName)
		return fmt.Sprintf(sqlServerDatabaseIOQuery, whereClause)
	}

	return fmt.Sprintf(sqlServerDatabaseIOQuery, "")
}

// sqlServerPerformanceCountersQuery collects various performance counter metrics from SQL Server.
// This includes metrics for memory usage, cache hits, transaction throughput, and more.
const sqlServerPerformanceCountersQuery string = `
-- Set low priority to minimize impact on production workload
SET DEADLOCK_PRIORITY -10;

-- Check if this is a supported SQL Server edition
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard,Enterprise,Express*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard, Enterprise or Express. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

-- Declare variables for version detection
DECLARE
	 @SqlStatement AS nvarchar(max)
	,@MajorMinorVersion AS int = CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),4) AS int)*100 + CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),3) AS int)

-- Create temporary table to store counter values
DECLARE @PCounters TABLE
(
	 [object_name] nvarchar(128)    -- Counter object (e.g., SQLServer:Buffer Manager)
	,[counter_name] nvarchar(128)   -- Counter name (e.g., Buffer cache hit ratio)
	,[instance_name] nvarchar(128)  -- Instance name for specific counters
	,[cntr_value] bigint           -- Current counter value
	,[cntr_type] int               -- Counter type (e.g., rate, ratio)
	PRIMARY KEY([object_name], [counter_name], [instance_name])
);

-- Collect performance counter values
WITH PerfCounters AS (
SELECT DISTINCT
	 RTRIM(spi.[object_name]) [object_name]      -- Remove trailing spaces from names
	,RTRIM(spi.[counter_name]) [counter_name]
	,RTRIM(spi.[instance_name]) AS [instance_name]
	,CAST(spi.[cntr_value] AS bigint) AS [cntr_value]
	,spi.[cntr_type]
	FROM sys.dm_os_performance_counters AS spi
	WHERE
		counter_name IN (
			-- Memory counters
			 'Buffer cache hit ratio'
			,'Page life expectancy'
			-- Query counters
			,'SQL Compilations/sec'
			,'SQL Re-Compilations/sec'
			-- Connection counters
			,'User Connections'
			,'Batch Requests/sec'
			,'Logouts/sec'
			,'Logins/sec'
			-- ... (remaining counter names)
		) OR (
			-- Include user-settable counters and error stats
			spi.[object_name] LIKE '%User Settable%'
			OR spi.[object_name] LIKE '%SQL Errors%'
			OR spi.[object_name] LIKE '%Batch Resp Statistics%'
		) OR (
			-- Include total instance counters for specific metrics
			spi.[instance_name] IN ('_Total')
			AND spi.[counter_name] IN (
				 'Lock Timeouts/sec'
				,'Lock Timeouts (timeout > 0)/sec'
				,'Number of Deadlocks/sec'
				,'Lock Waits/sec'
				,'Latch Waits/sec'
			)
		)
)

-- Store counter values for processing
INSERT INTO @PCounters SELECT * FROM PerfCounters;

-- Calculate and return counter values
SELECT
	 'sqlserver_performance' AS [measurement]
	,REPLACE(@@SERVERNAME,'\',':') AS [sql_instance]
	,HOST_NAME() AS [computer_name]
	,pc.[object_name] AS [object]
	,pc.[counter_name] AS [counter]
	,CASE pc.[instance_name] WHEN '_Total' THEN 'Total' ELSE ISNULL(pc.[instance_name],'') END AS [instance]
	-- Calculate actual counter value based on counter type
	,CAST(CASE WHEN pc.[cntr_type] = 537003264 AND pc1.[cntr_value] > 0 THEN (pc.[cntr_value] * 1.0) / (pc1.[cntr_value] * 1.0) * 100 ELSE pc.[cntr_value] END AS float(10)) AS [value]
	,CAST(pc.[cntr_type] AS varchar(25)) AS [counter_type]
FROM @PCounters AS pc
LEFT OUTER JOIN @PCounters AS pc1
	ON (
		-- Join with base counter values for ratio calculations
		pc.[counter_name] = REPLACE(pc1.[counter_name],' base','')
		OR pc.[counter_name] = REPLACE(pc1.[counter_name],' base',' (ms)')
	)
	AND pc.[object_name] = pc1.[object_name]
	AND pc.[instance_name] = pc1.[instance_name]
	AND pc1.[counter_name] LIKE '%base'
WHERE
	pc.[counter_name] NOT LIKE '% base'
{filter_instance_name}
OPTION(RECOMPILE)  -- Always generate fresh execution plan
`

// getSQLServerPerformanceCounterQuery returns a query to collect performance counter metrics.
// Parameters:
// - instanceName: SQL Server instance to monitor (empty string for default instance)
func getSQLServerPerformanceCounterQuery(instanceName string) string {
	if instanceName != "" {
		whereClause := fmt.Sprintf("\tAND @@SERVERNAME = '%s'", instanceName)
		r := strings.NewReplacer("{filter_instance_name}", whereClause)
		return r.Replace(sqlServerPerformanceCountersQuery)
	}

	r := strings.NewReplacer("{filter_instance_name}", "")
	return r.Replace(sqlServerPerformanceCountersQuery)
}

// sqlServerProperties collects server-level properties and configuration settings.
// This includes hardware information, SQL Server version, uptime, and database states.
const sqlServerProperties = `
-- Set low priority to minimize impact on production workload
SET DEADLOCK_PRIORITY -10;

-- Check if this is a supported SQL Server edition
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard, Enterprise, Express*/
	DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard, Enterprise or Express. This query is only supported on these editions.';
	RAISERROR (@ErrorMessage,11,1)
	RETURN
END

-- Declare variables for dynamic SQL and version detection
DECLARE
	 @SqlStatement AS nvarchar(max) = ''
	,@MajorMinorVersion AS int = CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),4) AS int)*100 + CAST(PARSENAME(CAST(SERVERPROPERTY('ProductVersion') AS nvarchar),3) AS int)
	,@Columns AS nvarchar(MAX) = ''

-- Add virtualization info for SQL Server 2008 R2 SP1 and later
IF CAST(SERVERPROPERTY('ProductVersion') AS varchar(50)) >= '10.50.2500.0'
	SET @Columns = N'
	,CASE [virtual_machine_type_desc]
		WHEN ''NONE'' THEN ''PHYSICAL Machine''
		ELSE [virtual_machine_type_desc]
	END AS [hardware_type]'

SET @SqlStatement = '
-- Declare variables for registry access
DECLARE @ForceEncryption INT
DECLARE @DynamicportNo NVARCHAR(50);
DECLARE @StaticportNo NVARCHAR(50);

-- Read encryption settings from registry
EXEC [xp_instance_regread]
	 @rootkey = ''HKEY_LOCAL_MACHINE''
	,@key = ''SOFTWARE\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib''
	,@value_name = ''ForceEncryption''
	,@value = @ForceEncryption OUTPUT;

-- Read dynamic port settings
EXEC [xp_instance_regread]
	 @rootkey = ''HKEY_LOCAL_MACHINE''
	,@key = ''Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib\Tcp\IpAll''
	,@value_name = ''TcpDynamicPorts''
	,@value = @DynamicportNo OUTPUT

-- Read static port settings
EXEC [xp_instance_regread]
	  @rootkey = ''HKEY_LOCAL_MACHINE''
     ,@key = ''Software\Microsoft\Microsoft SQL Server\MSSQLServer\SuperSocketNetLib\Tcp\IpAll''
     ,@value_name = ''TcpPort''
     ,@value = @StaticportNo OUTPUT

-- Collect server properties
SELECT
	 ''sqlserver_server_properties'' AS [measurement]
	,REPLACE(@@SERVERNAME,''\'','':'') AS [sql_instance]     -- Instance name
	,HOST_NAME() AS [computer_name]                          -- Host machine name
	,@@SERVICENAME AS [service_name]                         -- SQL Server service name
	,si.[cpu_count]                                          -- Number of CPUs
	,(SELECT [total_physical_memory_kb] FROM sys.[dm_os_sys_memory]) AS [server_memory]           -- Total server memory
	,(SELECT [available_physical_memory_kb] FROM sys.[dm_os_sys_memory]) AS [available_server_memory]  -- Available memory
	,SERVERPROPERTY(''Edition'') AS [sku]                    -- SQL Server edition
	,CAST(SERVERPROPERTY(''EngineEdition'') AS int) AS [engine_edition]  -- Engine edition number
	,DATEDIFF(MINUTE,si.[sqlserver_start_time],GETDATE()) AS [uptime]    -- Server uptime in minutes
	,SERVERPROPERTY(''ProductVersion'') AS [sql_version]     -- SQL Server version
	,SERVERPROPERTY(''IsClustered'') AS [instance_type]      -- Clustered or standalone
	,SERVERPROPERTY(''IsHadrEnabled'') AS [is_hadr_enabled]  -- AlwaysOn availability groups enabled
	,LEFT(@@VERSION,CHARINDEX('' - '',@@VERSION)) AS [sql_version_desc]  -- Full version description
	,@ForceEncryption AS [ForceEncryption]                   -- Encryption required flag
	,COALESCE(@DynamicportNo,@StaticportNo) AS [Port]       -- TCP port in use
	,IIF(@DynamicportNo IS NULL, ''Static'', ''Dynamic'') AS [PortType]  -- Port allocation type
	,dbs.[db_online]                                         -- Count of online databases
	,dbs.[db_restoring]                                      -- Count of restoring databases
	,dbs.[db_recovering]                                     -- Count of recovering databases
	,dbs.[db_recoveryPending]                                -- Count of recovery pending databases
	,dbs.[db_suspect]                                        -- Count of suspect databases
	,dbs.[db_offline]'                                       -- Count of offline databases
	+ @Columns + N'
	FROM sys.[dm_os_sys_info] AS si
	CROSS APPLY (
		-- Get database state counts
		SELECT
			 SUM(CASE WHEN [state] = 0 THEN 1 ELSE 0 END) AS [db_online]
			,SUM(CASE WHEN [state] = 1 THEN 1 ELSE 0 END) AS [db_restoring]
			,SUM(CASE WHEN [state] = 2 THEN 1 ELSE 0 END) AS [db_recovering]
			,SUM(CASE WHEN [state] = 3 THEN 1 ELSE 0 END) AS [db_recoveryPending]
			,SUM(CASE WHEN [state] = 4 THEN 1 ELSE 0 END) AS [db_suspect]
			,SUM(CASE WHEN [state] IN (6,10) THEN 1 ELSE 0 END) AS [db_offline]
		FROM sys.databases
	) AS dbs
%s'  -- Placeholder for instance filter

EXEC sp_executesql @SqlStatement
`

// getSQLServerPropertiesQuery returns a query to collect server properties.
// Parameters:
// - instanceName: SQL Server instance to monitor (empty string for default instance)
func getSQLServerPropertiesQuery(instanceName string) string {
	if instanceName != "" {
		whereClause := fmt.Sprintf("WHERE @@SERVERNAME = ''%s''", instanceName)
		return fmt.Sprintf(sqlServerProperties, whereClause)
	}

	return fmt.Sprintf(sqlServerProperties, "")
}

// sqlServerQueryMetrics collects performance metrics for the most resource-intensive queries
// from SQL Server's query stats DMV (sys.dm_exec_query_stats).
// The query returns aggregated statistics about query execution, I/O, and memory usage
// for queries that have executed within a specified time window.
const sqlServerQueryMetrics = `
-- Set low priority to minimize impact on production workload
SET DEADLOCK_PRIORITY -10;

-- Check if this is a supported SQL Server edition (Standard, Enterprise, or Express)
IF SERVERPROPERTY('EngineEdition') NOT IN (2,3,4) BEGIN /*NOT IN Standard, Enterprise, Express*/
    DECLARE @ErrorMessage AS nvarchar(500) = 'Connection string Server:'+ @@ServerName + ',Database:' + DB_NAME() +' is not a SQL Server Standard, Enterprise or Express. This query is only supported on these editions.';
    RAISERROR (@ErrorMessage,11,1)
    RETURN
END

-- Parameters injected by the collector:
-- First %s = granularity declaration (e.g. "DECLARE @granularity INT = -3600" for 1 hour)
-- Second %s = top N value declaration (e.g. "DECLARE @topNValue INT = 20" to get top 20 queries)
%s
%s

-- Get the top N most resource-intensive queries from the query stats DMV
SELECT TOP(@topNValue)
    -- Server identification
    REPLACE(@@SERVERNAME,'\',':') AS [sql_instance],  -- Replace backslash with colon in instance name
    HOST_NAME() AS [computer_name],                    -- Name of the host machine
    
    -- Query identification - used to track unique queries across executions
    qs.query_hash AS query_hash,                       -- Hash of the query (normalized SQL text)
    qs.query_plan_hash AS query_plan_hash,            -- Hash of the execution plan
    
    -- Execution statistics (aggregated across all executions in the time window)
    SUM(qs.execution_count) AS execution_count,       -- Number of times the query was executed
    SUM(qs.total_elapsed_time) AS total_elapsed_time, -- Total time spent executing (includes waiting)
    SUM(qs.total_worker_time) AS total_worker_time,   -- Total CPU time used
    
    -- I/O statistics - measure buffer pool and disk activity
    SUM(qs.total_logical_reads) AS total_logical_reads,   -- Number of pages read from buffer cache
    SUM(qs.total_physical_reads) AS total_physical_reads, -- Number of pages read from disk
    SUM(qs.total_logical_writes) AS total_logical_writes, -- Number of pages modified in buffer cache
    
    -- Result and memory statistics
    SUM(qs.total_rows) AS total_rows,                 -- Total number of rows returned to client
    SUM(qs.total_grant_kb) as total_grant_kb         -- Total memory grant in kilobytes
FROM 
    sys.dm_exec_query_stats AS qs  -- DMV containing execution statistics for cached query plans
WHERE 
    -- Filter for queries executed within the specified time window
    qs.last_execution_time BETWEEN DATEADD(SECOND, @granularity, GETDATE()) AND GETDATE() %s
GROUP BY
    -- Group by query identifiers to aggregate metrics for each unique query
    qs.query_hash,
    qs.query_plan_hash
ORDER BY 
    -- Order by CPU time to focus on the most resource-intensive queries
    SUM(qs.total_worker_time) DESC
OPTION(RECOMPILE);  -- Always generate a new execution plan to handle varying data sizes
`

const (
	// granularityDeclaration defines how far back in time to look for query metrics
	// Format: "DECLARE @granularity INT = -%d" where %d is seconds
	granularityDeclaration = `DECLARE @granularity INT = -%d;`

	// topNValueDeclaration defines how many top queries to return
	// Format: "DECLARE @topNValue INT = %d" where %d is the limit
	topNValueDeclaration = `DECLARE @topNValue INT = %d;`
)

// getSQLServerQueryMetricsQuery returns a query to collect query performance metrics
// Parameters:
// - instanceName: SQL Server instance to monitor (empty string for default)
// - maxQuerySampleCount: number of top queries to collect
// - granularity: time window in seconds to look back for query metrics
func getSQLServerQueryMetricsQuery(instanceName string, maxQuerySampleCount uint, granularity uint) string {
	var topQueryCountStatement string
	var granularityStatement string
	var instanceNameClause string

	topQueryCountStatement = fmt.Sprintf(topNValueDeclaration, maxQuerySampleCount)
	granularityStatement = fmt.Sprintf(granularityDeclaration, granularity)

	if instanceName != "" {
		instanceNameClause = fmt.Sprintf("AND @@SERVERNAME = '%s'", instanceName)
	} else {
		instanceNameClause = ""
	}

	return fmt.Sprintf(sqlServerQueryMetrics, granularityStatement, topQueryCountStatement, instanceNameClause)
}

// sqlServerQueryTextAndPlan collects the actual SQL text and execution plans for the most resource-intensive queries.
// This provides detailed information about query content and execution strategy for analysis and optimization.
const sqlServerQueryTextAndPlan = `
%s  -- Granularity declaration
%s  -- Top N value declaration

-- Common Table Expression (CTE) to get query statistics
with qstats as (
SELECT TOP(@topNValue)
    -- Server identification
    REPLACE(@@SERVERNAME,'\',':') AS [sql_instance],
    HOST_NAME() AS [computer_name],
    
    -- Plan identification
    MAX(qs.plan_handle) AS query_plan_handle,         -- Handle to get the execution plan
    qs.query_hash AS query_hash,                      -- Query hash for correlation
    qs.query_plan_hash AS query_plan_hash,           -- Plan hash for correlation
    
    -- Execution metrics
    SUM(qs.execution_count) AS execution_count,      -- Total executions
    SUM(qs.total_elapsed_time) AS total_elapsed_time, -- Total duration
    SUM(qs.total_worker_time) AS total_worker_time,  -- CPU time
    
    -- I/O metrics
    SUM(qs.total_logical_reads) AS total_logical_reads,
    SUM(qs.total_physical_reads) AS total_physical_reads,
    SUM(qs.total_logical_writes) AS total_logical_writes,
    
    -- Result metrics
    SUM(qs.total_rows) AS total_rows,
    SUM(qs.total_grant_kb) as total_grant_kb
FROM sys.dm_exec_query_stats AS qs
WHERE qs.last_execution_time BETWEEN DATEADD(SECOND, @granularity, GETDATE()) AND GETDATE() %s
GROUP BY
    qs.query_hash,
    qs.query_plan_hash
)
-- Get the SQL text and execution plan for each query
SELECT qs.*,
    -- Extract the specific statement from the batch
    SUBSTRING(st.text, (stats.statement_start_offset / 2) + 1,
             ((CASE statement_end_offset
                   WHEN -1 THEN DATALENGTH(st.text)
                   ELSE stats.statement_end_offset END - stats.statement_start_offset) / 2) + 1) AS text,
    ISNULL(qp.query_plan, '') AS query_plan          -- XML execution plan
FROM qstats AS qs
INNER JOIN sys.dm_exec_query_stats AS stats on qs.query_plan_handle = stats.plan_handle
CROSS APPLY sys.dm_exec_query_plan(qs.query_plan_handle) AS qp  -- Get execution plan
CROSS APPLY sys.dm_exec_sql_text(qs.query_plan_handle) AS st;   -- Get SQL text
`

// getSQLServerQueryTextAndPlanQuery returns a query to collect SQL text and execution plans
// Parameters:
// - instanceName: SQL Server instance to monitor (empty string for default)
// - maxQuerySampleCount: number of top queries to collect
// - granularity: time window in seconds to look back
func getSQLServerQueryTextAndPlanQuery(instanceName string, maxQuerySampleCount uint, granularity uint) string {
	var topQueryCountStatement string
	var granularityStatement string
	var instanceNameClause string

	topQueryCountStatement = fmt.Sprintf(topNValueDeclaration, maxQuerySampleCount)
	granularityStatement = fmt.Sprintf(granularityDeclaration, granularity)

	if instanceName != "" {
		instanceNameClause = fmt.Sprintf("AND @@SERVERNAME = '%s'", instanceName)
	} else {
		instanceNameClause = ""
	}

	return fmt.Sprintf(sqlServerQueryTextAndPlan, granularityStatement, topQueryCountStatement, instanceNameClause)
}

// sqlServerQuerySamples collects information about currently executing queries.
// This provides real-time visibility into active workload and blocking situations.
const sqlServerQuerySamples = `
-- Get information about currently executing requests
SELECT 
    -- User and connection information
    USER_NAME(r.user_id) AS user_name,               -- Name of the user running the query
    DB_NAME(r.database_id) AS db_name,               -- Database being accessed
    ISNULL(c.client_tcp_port, '') AS client_port,    -- Client connection port
    
    -- Query timing
    CONVERT(NVARCHAR, TODATETIMEOFFSET(r.start_time, 
        DATEPART(TZOFFSET, SYSDATETIMEOFFSET())), 126) AS query_start,  -- Query start time with timezone
    
    -- Session information
    s.session_id,                                    -- Session ID for correlation
    s.STATUS AS session_status,                      -- Current session status
    ISNULL(s.host_name, '') AS host_name,           -- Client hostname
    
    -- Query information
    r.command,                                       -- Command type
    SUBSTRING(o.TEXT, (r.statement_start_offset / 2) + 1,  -- Currently executing statement
        ((CASE r.statement_end_offset
            WHEN - 1 THEN DATALENGTH(o.TEXT)
            ELSE r.statement_end_offset END 
            - r.statement_start_offset) / 2) + 1) AS statement_text,
    
    -- Blocking information
    r.blocking_session_id,                           -- ID of session blocking this request
    ISNULL(r.wait_type, '') AS wait_type,           -- Resource being waited on
    r.wait_time,                                     -- How long the request has been waiting
    r.wait_resource,                                 -- Specific resource being waited on
    
    -- Transaction information
    r.open_transaction_count,                        -- Number of open transactions
    r.transaction_id,                                -- Transaction ID
    r.percent_complete,                              -- Percent complete for certain operations
    r.estimated_completion_time,                     -- Estimated time to completion
    
    -- Resource usage
    r.cpu_time,                                      -- CPU time used
    r.total_elapsed_time,                            -- Total time elapsed
    r.reads,                                         -- Physical reads
    r.writes,                                        -- Physical writes
    r.logical_reads,                                 -- Logical reads
    
    -- Query context
    r.transaction_isolation_level,                   -- Transaction isolation level
    r.LOCK_TIMEOUT,                                  -- Lock timeout setting
    r.DEADLOCK_PRIORITY,                             -- Deadlock priority
    r.row_count,                                     -- Rows processed
    r.query_hash,                                    -- Query hash for correlation
    r.query_plan_hash,                               -- Plan hash for correlation
    ISNULL(r.context_info, CONVERT(VARBINARY, '')) AS context_info,  -- Context info
    
    -- Login information
    s.login_name,                                    -- SQL Server login name
    s.original_login_name,                           -- Original login name if impersonating
    
    -- Object information
    CASE
        WHEN o.objectid IS NULL THEN ''              -- Ad-hoc or dynamic SQL
        ELSE CONCAT (                                -- Named object
            DB_NAME(o.dbid),
            '.',
            OBJECT_NAME(o.objectid, o.dbid)
        )
    END AS object_name
FROM sys.dm_exec_requests r
INNER JOIN sys.dm_exec_sessions s ON r.session_id = s.session_id  -- Get session information
INNER JOIN sys.dm_exec_connections c ON s.session_id = c.session_id  -- Get connection information
CROSS APPLY sys.dm_exec_sql_text(r.plan_handle) AS o;  -- Get SQL text
`

// getSQLServerQuerySamplesQuery returns the query to collect information about currently executing queries
func getSQLServerQuerySamplesQuery() string {
	return sqlServerQuerySamples
}
