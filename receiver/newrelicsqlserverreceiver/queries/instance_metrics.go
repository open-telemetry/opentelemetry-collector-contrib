// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for instance-level metrics.
// This file contains all SQL queries for collecting 32 instance-level SQL Server metrics.
//
// Instance Metrics Categories (32 total metrics):
//
// 1. Memory Metrics:
//   - Buffer pool size (sys.dm_os_buffer_descriptors)
//   - Memory usage by clerks (sys.dm_os_memory_clerks)
//   - Available physical memory (sys.dm_os_sys_memory)
//   - Page life expectancy (sys.dm_os_performance_counters)
//   - Memory grants pending/outstanding
//
// 2. CPU Metrics:
//   - CPU utilization percentage (sys.dm_os_ring_buffers)
//   - SQL Server CPU time vs system CPU time
//   - Context switches per second
//   - Work queue usage and parallel query metrics
//
// 3. I/O Metrics:
//   - Total I/O operations (reads/writes per second)
//   - I/O stall statistics (sys.dm_io_virtual_file_stats)
//   - Batch requests per second
//   - Page reads/writes per second
//
// 4. Connection/Session Metrics:
//   - Active connections count (sys.dm_exec_sessions)
//   - User connections vs system connections
//   - Logins per second and logouts per second
//   - Blocked processes count
//
// 5. Transaction/Lock Metrics:
//   - Active transactions count (sys.dm_tran_active_transactions)
//   - Lock waits per second and lock timeouts
//   - Deadlocks per second
//   - Distributed transaction coordinator metrics
//
// 6. Database Engine Metrics:
//   - Compilations per second and re-compilations
//   - Plan cache hit ratio and plan cache usage
//   - Backup/restore operations status
//   - Log flushes per second and log cache usage
//
// Engine Support:
// - Default: Full instance metrics from all DMVs
// - AzureSQLDatabase: Limited metrics (no OS-level DMVs)
// - AzureSQLManagedInstance: Most metrics available with some limitations
package queries

// InstanceBufferPoolQuery returns the SQL query for buffer pool size metrics
const InstanceBufferPoolQuery = `SELECT
	Count_big(*) * (8*1024) AS instance_buffer_pool_size
	FROM sys.dm_os_buffer_descriptors WITH (nolock)
	WHERE database_id <> 32767 -- ResourceDB`

const instanceMemoryDefinitions = `SELECT
		Max(sys_mem.total_physical_memory_kb * 1024.0) AS total_physical_memory,
		Max(sys_mem.available_physical_memory_kb * 1024.0) AS available_physical_memory,
		(Max(proc_mem.physical_memory_in_use_kb) / (Max(sys_mem.total_physical_memory_kb) * 1.0)) * 100 AS memory_utilization
		FROM sys.dm_os_process_memory proc_mem,
		  sys.dm_os_sys_memory sys_mem,
		  sys.dm_os_performance_counters perf_count WHERE object_name = 'SQLServer:Memory Manager'`
