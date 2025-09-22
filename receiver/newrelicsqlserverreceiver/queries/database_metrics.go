// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for database-level metrics.
// This file contains all SQL queries for collecting 9 database-level SQL Server metrics.
//
// Database Metrics Categories (9 total metrics):
//
// 1. Database Size Metrics:
//   - Total database size in MB/GB (sys.database_files)
//   - Data file size and usage (allocated vs used space)
//   - Log file size and usage percentage
//
// 2. Database I/O Metrics:
//   - Database-specific read/write operations (sys.dm_io_virtual_file_stats)
//   - I/O stall time per database
//   - Average I/O response time per database
//
// 3. Database Activity Metrics:
//   - Active sessions per database (sys.dm_exec_sessions)
//   - Database-specific lock waits and timeouts
//   - Transaction log usage and growth events
//
// Query Sources:
// - sys.databases: Database state, collation, and basic properties
// - sys.database_files: File sizes, growth settings, and space usage
// - sys.dm_io_virtual_file_stats: I/O statistics per database file
// - sys.dm_exec_sessions: Active connections and sessions per database
// - sys.dm_db_log_space_usage: Transaction log space utilization
// - sys.dm_db_file_space_usage: Data file space utilization
// - sys.dm_os_performance_counters: Database-specific performance counters
//
// Metric Collection Strategy:
// - Iterate through all online databases (sys.databases WHERE state = 0)
// - Collect metrics per database with database_name as dimension
// - Aggregate metrics where appropriate (total vs per-database)
// - Handle system databases separately (master, model, msdb, tempdb)
//
// Engine Support:
// - Default: Full database metrics for all user and system databases
// - AzureSQLDatabase: Single database scope, limited system database access
// - AzureSQLManagedInstance: Multiple databases, full access to system databases
package queries

// DatabaseBufferPoolQuery returns the SQL query for buffer pool size per database
// This query retrieves buffer pool usage for each database based on the New Relic implementation
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/database_metric_definitions.go
const DatabaseBufferPoolQuery = `SELECT 
	DB_NAME(database_id) AS db_name, 
	buffer_pool_size * (8*1024) AS buffer_pool_size
FROM ( 
	SELECT database_id, COUNT_BIG(*) AS buffer_pool_size 
	FROM sys.dm_os_buffer_descriptors a WITH (NOLOCK)
	INNER JOIN sys.sysdatabases b WITH (NOLOCK) ON b.dbid=a.database_id 
	WHERE b.dbid in (
		SELECT dbid FROM sys.sysdatabases WITH (NOLOCK)
		WHERE name NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
		UNION ALL SELECT 32767
	) 
	GROUP BY database_id
) a`

// DatabaseBufferPoolQueryAzureSQL returns the Azure SQL Database specific buffer pool query
const DatabaseBufferPoolQueryAzureSQL = `SELECT 
	DB_NAME() AS db_name, 
	COUNT_BIG(*) * (8 * 1024) AS buffer_pool_size
FROM sys.dm_os_buffer_descriptors WITH (NOLOCK) 
WHERE database_id = DB_ID()`

// DatabaseMaxDiskSizeQuery returns the SQL query for maximum database disk size
// For regular SQL Server instances, we'll return the current database size since MaxSizeInBytes
// is only applicable to Azure SQL Database managed service
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/database_metric_definitions.go
const DatabaseMaxDiskSizeQuery = `SELECT 
	name AS db_name, 
	CAST(COALESCE(DATABASEPROPERTYEX(name, 'MaxSizeInBytes'), 
		(SELECT SUM(CAST(size AS BIGINT) * 8 * 1024) 
		 FROM sys.master_files 
		 WHERE database_id = d.database_id)) AS BIGINT) AS max_disk_space
FROM sys.databases d
WHERE name NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
	AND state = 0`

// DatabaseMaxDiskSizeQueryAzureSQL returns the Azure SQL Database specific max disk size query
const DatabaseMaxDiskSizeQueryAzureSQL = `SELECT 
	DB_NAME() AS db_name, 
	CAST(DATABASEPROPERTYEX(DB_NAME(), 'MaxSizeInBytes') AS BIGINT) AS max_disk_space`

// DatabaseIOStallQuery returns the SQL query for database IO stall metrics
// This query gets the total IO stall time in milliseconds per database
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/database_metric_definitions.go
const DatabaseIOStallQuery = `SELECT
	DB_NAME(database_id) AS db_name,
	SUM(io_stall) AS io_stalls
FROM sys.dm_io_virtual_file_stats(null,null)
WHERE DB_NAME(database_id) NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
GROUP BY database_id`

// DatabaseIOStallQueryAzureSQL returns the Azure SQL Database specific IO stall query
const DatabaseIOStallQueryAzureSQL = `SELECT
	DB_NAME() AS db_name,
	SUM(io_stall) AS io_stalls
FROM sys.dm_io_virtual_file_stats(NULL, NULL)
WHERE database_id = DB_ID()`

// DatabaseIOStallQueryAzureMI returns the Azure SQL Managed Instance specific IO stall query
const DatabaseIOStallQueryAzureMI = `SELECT
	DB_NAME(database_id) AS db_name,
	SUM(io_stall) AS io_stalls
FROM sys.dm_io_virtual_file_stats(null,null)
WHERE DB_NAME(database_id) NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
GROUP BY database_id`

// DatabaseLogGrowthQuery returns the SQL query for database log growth metrics
// This query retrieves log growth events per database from SQL Server performance counters
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/database_metric_definitions.go
const DatabaseLogGrowthQuery = `SELECT
	RTRIM(t1.instance_name) AS db_name,
	t1.cntr_value AS log_growth
FROM (
	SELECT * FROM sys.dm_os_performance_counters WITH (NOLOCK)
	WHERE object_name = 'SQLServer:Databases'
		AND counter_name = 'Log Growths'
		AND RTRIM(instance_name) NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
		AND instance_name NOT IN ('_Total', 'mssqlsystemresource', 'master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
) t1`

// DatabaseLogGrowthQueryAzureSQL returns the Azure SQL Database specific log growth query
const DatabaseLogGrowthQueryAzureSQL = `SELECT 
	sd.name AS db_name,
	spc.cntr_value AS log_growth
FROM sys.dm_os_performance_counters spc
INNER JOIN sys.databases sd 
	ON sd.physical_database_name = spc.instance_name
WHERE spc.counter_name = 'Log Growths'
	AND spc.object_name LIKE '%:Databases%'
	AND sd.database_id = DB_ID()`

// DatabaseLogGrowthQueryAzureMI returns the Azure SQL Managed Instance specific log growth query
const DatabaseLogGrowthQueryAzureMI = `SELECT 
	sd.name AS db_name,
	spc.cntr_value AS log_growth 
FROM sys.dm_os_performance_counters spc WITH (NOLOCK)
INNER JOIN sys.databases sd 
	ON sd.physical_database_name = spc.instance_name
WHERE spc.object_name LIKE '%:Databases%'
	AND spc.counter_name = 'Log Growths'
	AND sd.name NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
	AND spc.instance_name NOT IN ('_Total', 'mssqlsystemresource', 'master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')`

// DatabasePageFileQuery returns the SQL query for page file available metrics
// This query needs to be executed per database to get accurate page file information
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/database_metric_definitions.go
const DatabasePageFileQuery = `SELECT TOP 1
	'%DATABASE%' AS db_name,
	(SUM(a.total_pages) * 8.0 - SUM(a.used_pages) * 8.0) * 1024 AS reserved_space_not_used
FROM [%DATABASE%].sys.partitions p WITH (NOLOCK)
INNER JOIN [%DATABASE%].sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
LEFT JOIN [%DATABASE%].sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`

// DatabasePageFileTotalQuery returns the SQL query for page file total metrics
// This query retrieves the total reserved space (page file total) for the database
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/database_metric_definitions.go
const DatabasePageFileTotalQuery = `SELECT TOP 1
	'%DATABASE%' AS db_name,
	SUM(a.total_pages) * 8.0 * 1024 AS reserved_space
FROM [%DATABASE%].sys.partitions p WITH (NOLOCK)
INNER JOIN [%DATABASE%].sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
LEFT JOIN [%DATABASE%].sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`

// DatabasePageFileQueryAzureSQL returns the Azure SQL Database specific page file query
const DatabasePageFileQueryAzureSQL = `SELECT
	DB_NAME() AS db_name,
	(SUM(a.total_pages) * 8.0 - SUM(a.used_pages) * 8.0) * 1024 AS reserved_space_not_used
FROM sys.partitions p WITH (NOLOCK)
INNER JOIN sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
LEFT JOIN sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`

// DatabasePageFileTotalQueryAzureSQL returns the Azure SQL Database specific page file total query
const DatabasePageFileTotalQueryAzureSQL = `SELECT
	DB_NAME() AS db_name,
	SUM(a.total_pages) * 8.0 * 1024 AS reserved_space
FROM sys.partitions p WITH (NOLOCK)
INNER JOIN sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
LEFT JOIN sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`

// DatabasePageFileQueryAzureMI returns the Azure SQL Managed Instance specific page file query
const DatabasePageFileQueryAzureMI = `SELECT 
	DB_NAME() AS db_name,
	(SUM(a.total_pages) * 8.0 - SUM(a.used_pages) * 8.0) * 1024 AS reserved_space_not_used
FROM sys.partitions p WITH (NOLOCK)
INNER JOIN sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
LEFT JOIN sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`

// DatabasePageFileTotalQueryAzureMI returns the Azure SQL Managed Instance specific page file total query
const DatabasePageFileTotalQueryAzureMI = `SELECT 
	DB_NAME() AS db_name,
	SUM(a.total_pages) * 8.0 * 1024 AS reserved_space
FROM sys.partitions p WITH (NOLOCK)
INNER JOIN sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
LEFT JOIN sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`

// DatabaseMemoryQuery returns the SQL query for memory metrics (total, available, utilization)
// This is an instance-level metric that provides comprehensive system memory information
// This query works for all SQL Server engine types (Standard, Azure SQL Database, Azure Managed Instance)
// Source: https://github.com/newrelic/nri-mssql/blob/main/src/metrics/instance_metric_definitions.go
const DatabaseMemoryQuery = `SELECT 
	MAX(sys_mem.total_physical_memory_kb * 1024.0) AS total_physical_memory,
	MAX(sys_mem.available_physical_memory_kb * 1024.0) AS available_physical_memory,
	(MAX(proc_mem.physical_memory_in_use_kb) / (MAX(sys_mem.total_physical_memory_kb) * 1.0)) * 100 AS memory_utilization
FROM sys.dm_os_process_memory proc_mem,
	sys.dm_os_sys_memory sys_mem,
	sys.dm_os_performance_counters perf_count 
WHERE object_name = 'SQLServer:Memory Manager'`


