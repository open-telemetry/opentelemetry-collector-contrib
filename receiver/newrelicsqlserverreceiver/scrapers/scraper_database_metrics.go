// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the database-level metrics scraper for SQL Server.
// This file implements comprehensive collection of 9 database-level metrics
// covering database size, I/O operations, and database activity statistics.
//
// Database-Level Metrics (9 total):
//
// 1. Database Size Metrics (3 metrics):
//    - Database Size (MB): Total size of the database including log files
//    - Data File Size (MB): Size of all data files in the database
//    - Log File Size (MB): Size of all transaction log files in the database
//
// 2. Database I/O Metrics (3 metrics):
//    - Database Read I/O per Second: Database-specific read operations per second
//    - Database Write I/O per Second: Database-specific write operations per second
//    - Database I/O Stall Time (ms): Total I/O stall time for database files
//
// 3. Database Activity Metrics (3 metrics):
//    - Active Transactions: Number of active transactions in the database
//    - Database Sessions: Number of sessions connected to the database
//    - Log Flush Rate per Second: Transaction log flush operations per second
//
// Detailed Metric Descriptions:
//
// Database Size (MB):
// - Query: sys.database_files and FILEPROPERTY functions
// - Includes: Data files, log files, full-text catalog files
// - Calculation: Sum of (size * 8 / 1024) for all database files
//
// Data File Size (MB):
// - Query: sys.database_files WHERE type = 0 (data files)
// - Includes: Primary data file (.mdf) and secondary data files (.ndf)
// - Excludes: Transaction log files and full-text files
//
// Log File Size (MB):
// - Query: sys.database_files WHERE type = 1 (log files)
// - Includes: All transaction log files (.ldf)
// - Used for monitoring log file growth and space usage
//
// Database Read I/O per Second:
// - Query: sys.dm_io_virtual_file_stats() for read operations
// - Metric: num_of_reads per second for database files
// - Scope: All read I/O operations for the specific database
//
// Database Write I/O per Second:
// - Query: sys.dm_io_virtual_file_stats() for write operations  
// - Metric: num_of_writes per second for database files
// - Scope: All write I/O operations for the specific database
//
// Database I/O Stall Time (ms):
// - Query: sys.dm_io_virtual_file_stats() for I/O stall statistics
// - Metric: io_stall_read_ms + io_stall_write_ms
// - Purpose: Identify I/O bottlenecks at database level
//
// Active Transactions:
// - Query: sys.dm_tran_active_transactions and sys.dm_tran_session_transactions
// - Count: Active transactions scoped to specific database
// - Includes: Both user and system transactions
//
// Database Sessions:
// - Query: sys.dm_exec_sessions WHERE database_id = DB_ID()
// - Count: Sessions currently connected to the database
// - Excludes: System sessions and background tasks
//
// Log Flush Rate per Second:
// - Query: sys.dm_os_performance_counters for log flush waits
// - Metric: Log flush operations per second for the database
// - Critical for: Transaction throughput monitoring
//
// Scraper Structure:
// type DatabaseScraper struct {
//     config   *Config
//     mb       *metadata.MetricsBuilder
//     queries  *queries.DatabaseQueries  
//     logger   *zap.Logger
// }
//
// Data Sources:
// - sys.database_files: Database file information and sizes
// - sys.dm_io_virtual_file_stats(): Database I/O statistics
// - sys.dm_tran_active_transactions: Active transaction information
// - sys.dm_exec_sessions: Session and connection data
// - sys.dm_os_performance_counters: Database-specific performance counters
//
// Engine-Specific Considerations:
// - Azure SQL Database: All 9 metrics fully supported, database-scoped views
// - Azure SQL Managed Instance: All 9 metrics available with full functionality
// - Standard SQL Server: Complete access to all database-level metrics and DMVs
package scrapers
