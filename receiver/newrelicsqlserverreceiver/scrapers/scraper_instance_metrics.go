// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the instance-level metrics scraper for SQL Server.
// This file implements comprehensive collection of 32 instance-level metrics
// covering memory management, CPU utilization, I/O operations, connections, and transactions.
//
// Instance-Level Metrics Categories (32 total):
//
// 1. Memory Metrics (8 metrics):
//    - Total Server Memory (KB): Current memory used by SQL Server
//    - Target Server Memory (KB): Target memory SQL Server wants to achieve
//    - Buffer Cache Hit Ratio: Percentage of pages found in buffer cache
//    - Page Life Expectancy: Average time pages stay in buffer cache
//    - Buffer Pool Data Pages: Number of data pages in buffer pool
//    - Buffer Pool Free Pages: Number of free pages in buffer pool
//    - Memory Grants Pending: Number of processes waiting for memory grants
//    - SQL Compilation Rate: SQL compilations per second
//
// 2. CPU Metrics (4 metrics):
//    - CPU Usage Percentage: SQL Server CPU utilization
//    - CPU Usage (ms): Total CPU time consumed
//    - Scheduler Runnable Tasks: Tasks waiting in runnable queues
//    - Context Switches per Second: OS context switches per second
//
// 3. I/O Metrics (8 metrics):
//    - Disk Read I/O per Second: Physical disk reads per second
//    - Disk Write I/O per Second: Physical disk writes per second
//    - Disk Read Bytes per Second: Physical bytes read per second
//    - Disk Write Bytes per Second: Physical bytes written per second
//    - Log Flush Wait Time (ms): Average log flush wait time
//    - Log Flush Waits per Second: Log flush waits per second
//    - Checkpoint Pages per Second: Pages flushed by checkpoint per second
//    - Lazy Writes per Second: Lazy writer writes per second
//
// 4. Connection Metrics (4 metrics):
//    - User Connections: Current number of user connections
//    - Active Connections: Number of active connections
//    - Login Attempts per Second: Successful login attempts per second
//    - Login Failures per Second: Failed login attempts per second
//
// 5. Transaction Metrics (4 metrics):
//    - Transactions per Second: Number of transactions per second
//    - Active Transactions: Currently active transactions
//    - Blocked Processes: Number of currently blocked processes
//    - Deadlocks per Second: Number of deadlocks per second
//
// 6. Database Engine Metrics (4 metrics):
//    - Batch Requests per Second: SQL batch requests per second
//    - SQL Re-Compilations per Second: SQL statement recompiles per second
//    - Lock Waits per Second: Lock requests that had to wait
//    - Lock Wait Time (ms): Total wait time for locks
//
// Scraper Structure:
// type InstanceScraper struct {
//     config   *Config
//     mb       *metadata.MetricsBuilder
//     queries  *queries.InstanceQueries
//     logger   *zap.Logger
// }
//
// Data Sources:
// - sys.dm_os_performance_counters: Performance counter data
// - sys.dm_os_sys_memory: System memory information
// - sys.dm_exec_sessions: Session and connection data
// - sys.dm_os_schedulers: CPU scheduler information
// - sys.dm_io_virtual_file_stats: I/O statistics
//
// Engine-Specific Considerations:
// - Azure SQL Database: Limited OS-level metrics, focus on database-scoped counters
// - Azure SQL Managed Instance: Most instance metrics available
// - Standard SQL Server: Full access to all OS and instance-level performance counters
package scrapers
