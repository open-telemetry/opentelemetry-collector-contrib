// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for database-level metrics.
// This file contains all SQL queries for collecting 9 database-level SQL Server metrics.
//
// Database Metrics Categories (9 total metrics):
//
// 1. Database Size Metrics:
//    - Total database size in MB/GB (sys.database_files)
//    - Data file size and usage (allocated vs used space)
//    - Log file size and usage percentage
//
// 2. Database I/O Metrics:
//    - Database-specific read/write operations (sys.dm_io_virtual_file_stats)
//    - I/O stall time per database
//    - Average I/O response time per database
//
// 3. Database Activity Metrics:
//    - Active sessions per database (sys.dm_exec_sessions)
//    - Database-specific lock waits and timeouts
//    - Transaction log usage and growth events
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
