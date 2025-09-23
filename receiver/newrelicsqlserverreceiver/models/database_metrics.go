// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides data structures for database-level metrics and information.
// This file defines the data models used to represent SQL Server database-level performance data
// including database size, I/O operations, and database activity statistics.
//
// Database-Level Data Structures:
//
// 1. Database Size Metrics:
//
//	type DatabaseSizeMetrics struct {
//	    DatabaseSizeMB     float64   // Total size of the database including log files (MB)
//	    DataFileSizeMB     float64   // Size of all data files in the database (MB)
//	    LogFileSizeMB      float64   // Size of all transaction log files in the database (MB)
//	    DataFileUsedMB     float64   // Used space in data files (MB)
//	    LogFileUsedMB      float64   // Used space in log files (MB)
//	    DataFileFreeMB     float64   // Free space in data files (MB)
//	    LogFileFreeMB      float64   // Free space in log files (MB)
//	    DataFileCount      int       // Number of data files
//	    LogFileCount       int       // Number of log files
//	}
//
// 2. Database I/O Metrics:
//
//	type DatabaseIOMetrics struct {
//	    ReadIOPerSecond    float64   // Database-specific read operations per second
//	    WriteIOPerSecond   float64   // Database-specific write operations per second
//	    IOStallTimeMs      float64   // Total I/O stall time for database files (milliseconds)
//	    ReadStallTimeMs    float64   // Read I/O stall time (milliseconds)
//	    WriteStallTimeMs   float64   // Write I/O stall time (milliseconds)
//	    ReadBytesPerSecond float64   // Bytes read per second for database
//	    WriteBytesPerSecond float64  // Bytes written per second for database
//	    AvgReadLatencyMs   float64   // Average read latency (milliseconds)
//	    AvgWriteLatencyMs  float64   // Average write latency (milliseconds)
//	}
//
// 3. Database Activity Metrics:
//
//	type DatabaseActivityMetrics struct {
//	    ActiveTransactions     int64   // Number of active transactions in the database
//	    DatabaseSessions       int64   // Number of sessions connected to the database
//	    LogFlushRatePerSecond  float64 // Transaction log flush operations per second
//	    LogFlushWaitTimeMs     float64 // Average log flush wait time (milliseconds)
//	    LogFlushes             int64   // Total number of log flushes
//	    LogBytesUsed           int64   // Transaction log bytes used
//	    LogBytesReserved       int64   // Transaction log bytes reserved
//	}
//
// 4. Database File Information:
//
//	type DatabaseFile struct {
//	    FileID         int       // Database file ID
//	    FileName       string    // Logical file name
//	    PhysicalName   string    // Physical file path
//	    FileType       string    // File type (DATA, LOG)
//	    FileSizeMB     float64   // File size in MB
//	    UsedSpaceMB    float64   // Used space in MB
//	    FreeSpaceMB    float64   // Free space in MB
//	    GrowthMB       float64   // Growth increment in MB
//	    IsPercentGrowth bool     // Whether growth is percentage-based
//	    MaxSizeMB      float64   // Maximum file size in MB (-1 for unlimited)
//	    IsReadOnly     bool      // Whether file is read-only
//	    IsOffline      bool      // Whether file is offline
//	}
//
// 5. Comprehensive Database Metrics:
//
//	type DatabaseMetrics struct {
//	    DatabaseName    string                   // Name of the database
//	    DatabaseID      int                      // Database ID
//	    Size            DatabaseSizeMetrics      // Database size metrics (3 primary metrics)
//	    IO              DatabaseIOMetrics        // Database I/O metrics (3 primary metrics)
//	    Activity        DatabaseActivityMetrics  // Database activity metrics (3 primary metrics)
//	    Files           []DatabaseFile           // Individual file information
//	    CollectionTime  time.Time                // When metrics were collected
//	    ServerName      string                   // SQL Server instance name
//	    State           string                   // Database state (ONLINE, OFFLINE, etc.)
//	    Owner           string                   // Database owner
//	    CreateDate      time.Time                // Database creation date
//	    Collation       string                   // Database collation
//	    CompatibilityLevel int                   // Database compatibility level
//	    RecoveryModel   string                   // Recovery model (FULL, SIMPLE, BULK_LOGGED)
//	}
//
// 6. Database Summary Information:
//
//	type DatabaseSummary struct {
//	    DatabaseName       string    // Database name
//	    DatabaseID         int       // Database ID
//	    State              string    // Database state
//	    StateDesc          string    // Database state description
//	    CreateDate         time.Time // Creation date
//	    Owner              string    // Database owner
//	    Collation          string    // Database collation
//	    CompatibilityLevel int       // Compatibility level
//	    RecoveryModel      string    // Recovery model
//	    PageVerifyOption   string    // Page verify option
//	    IsAutoCloseOn      bool      // Auto close setting
//	    IsAutoShrinkOn     bool      // Auto shrink setting
//	    IsReadOnly         bool      // Read-only setting
//	    IsInStandby        bool      // Standby mode
//	    IsCleanlyShutdown  bool      // Clean shutdown status
//	}
//
// 7. Database Performance Counters:
//
//	type DatabasePerformanceCounter struct {
//	    DatabaseName    string    // Database name
//	    CounterName     string    // Performance counter name
//	    InstanceName    string    // Counter instance name
//	    CounterValue    int64     // Current counter value
//	    CounterType     int       // Counter type
//	    BaseValue       int64     // Base value for ratio counters
//	    Timestamp       time.Time // Collection timestamp
//	}
//
// Data Source Mappings:
// - sys.database_files: Database file information and sizes
// - sys.dm_io_virtual_file_stats(): Database I/O statistics
// - sys.dm_tran_active_transactions: Active transaction information
// - sys.dm_exec_sessions: Session and connection data
// - sys.dm_os_performance_counters: Database-specific performance counters
// - sys.databases: Database metadata and configuration
// - DATABASEPROPERTYEX(): Database property information
//
// Metric Calculations:
//
// Database Size (MB):
// - Query: SELECT SUM(size * 8.0 / 1024) FROM sys.database_files
// - Includes all file types (data and log files)
//
// Data File Size (MB):
// - Query: SELECT SUM(size * 8.0 / 1024) FROM sys.database_files WHERE type = 0
// - Only includes data files (.mdf, .ndf)
//
// Log File Size (MB):
// - Query: SELECT SUM(size * 8.0 / 1024) FROM sys.database_files WHERE type = 1
// - Only includes transaction log files (.ldf)
//
// Database Read I/O per Second:
// - Query: sys.dm_io_virtual_file_stats() - num_of_reads delta
// - Calculated as: (current_reads - previous_reads) / time_interval
//
// Database Write I/O per Second:
// - Query: sys.dm_io_virtual_file_stats() - num_of_writes delta
// - Calculated as: (current_writes - previous_writes) / time_interval
//
// Database I/O Stall Time (ms):
// - Query: sys.dm_io_virtual_file_stats() - io_stall
// - Sum of: io_stall_read_ms + io_stall_write_ms
//
// Active Transactions:
// - Query: COUNT(*) from sys.dm_tran_active_transactions joined with sessions
// - Filtered by database_id for specific database
//
// Database Sessions:
// - Query: COUNT(*) from sys.dm_exec_sessions WHERE database_id = DB_ID()
// - Excludes system sessions and background tasks
//
// Log Flush Rate per Second:
// - Query: sys.dm_os_performance_counters for 'Log Flushes/sec'
// - Database-specific counter from SQL Server performance counters
//
// Usage in Scrapers:
// - Populated by DatabaseScraper from SQL Server system catalogs and DMVs
// - Provides structured access to all 9 database-level metrics
// - Supports per-database metric collection for multi-database instances
// - Enables consistent data handling across different SQL Server editions
package models

// DatabaseBufferMetrics represents buffer pool metrics for a specific database
// This model captures the buffer pool size per database as defined in New Relic's MSSQL integration
type DatabaseBufferMetrics struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"db_name"`
	
	// BufferPoolSizeBytes represents the size of buffer pool allocated for this database in bytes
	// This metric corresponds to bufferpool.sizePerDatabaseInBytes from New Relic
	// Query source: sys.dm_os_buffer_descriptors with database-specific filtering
	BufferPoolSizeBytes *int64 `db:"buffer_pool_size" metric_name:"sqlserver.database.bufferpool.sizePerDatabaseInBytes" source_type:"gauge"`
}

// DatabaseDiskMetrics represents disk-related metrics for a specific database
// This model captures the maximum disk size metrics as defined in New Relic's MSSQL integration
type DatabaseDiskMetrics struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"db_name"`
	
	// MaxDiskSizeBytes represents the maximum size allowed for the database in bytes
	// This metric corresponds to maxDiskSizeInBytes from New Relic
	// Query source: DATABASEPROPERTYEX function for Azure SQL Database compatibility
	MaxDiskSizeBytes *int64 `db:"max_disk_space" metric_name:"sqlserver.database.maxDiskSizeInBytes" source_type:"gauge"`
}

// DatabaseIOMetrics represents IO stall metrics for a specific database
// This model captures the IO stall time metrics as defined in New Relic's MSSQL integration
type DatabaseIOMetrics struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"db_name"`
	
	// IOStallTimeMs represents the total IO stall time for the database in milliseconds
	// This metric corresponds to io.stallInMilliseconds from New Relic
	// Query source: sys.dm_io_virtual_file_stats for database-specific IO statistics
	IOStallTimeMs *int64 `db:"io_stalls" metric_name:"sqlserver.database.io.stallInMilliseconds" source_type:"gauge"`
}

// DatabaseLogGrowthMetrics represents log growth metrics for a specific database
// This model captures the log growth events as defined in New Relic's MSSQL integration
type DatabaseLogGrowthMetrics struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"db_name"`
	
	// LogGrowthCount represents the number of log growth events for the database
	// This metric corresponds to log.transactionGrowth from New Relic
	// Query source: sys.dm_os_performance_counters for 'Log Growths' counter
	LogGrowthCount *int64 `db:"log_growth" metric_name:"sqlserver.database.log.transactionGrowth" source_type:"gauge"`
}

// DatabasePageFileMetrics represents page file metrics for a specific database
// This model captures the available page file space as defined in New Relic's MSSQL integration
type DatabasePageFileMetrics struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"db_name"`
	
	// PageFileAvailableBytes represents the reserved space not used (available page file) in bytes
	// This metric corresponds to pageFileAvailable from New Relic
	// Query source: sys.partitions and sys.allocation_units for space allocation statistics
	PageFileAvailableBytes *float64 `db:"reserved_space_not_used" metric_name:"sqlserver.database.pageFileAvailable" source_type:"gauge"`
}

// DatabasePageFileTotalMetrics represents page file total metrics for a specific database
// This model captures the total reserved space (page file total) as defined in New Relic's MSSQL integration
type DatabasePageFileTotalMetrics struct {
	// DatabaseName is the name of the database
	DatabaseName string `db:"db_name"`
	
	// PageFileTotalBytes represents the total reserved space (page file total) in bytes
	// This metric corresponds to pageFileTotal from New Relic
	// Query source: sys.partitions and sys.allocation_units for total space allocation statistics
	PageFileTotalBytes *float64 `db:"reserved_space" metric_name:"sqlserver.database.pageFileTotal" source_type:"gauge"`
}

// DatabaseMemoryMetrics represents comprehensive memory metrics
// This model captures the total memory, available memory, and memory utilization as defined in New Relic's MSSQL integration
// Note: These are instance-level metrics (not per-database) that provide system memory information
type DatabaseMemoryMetrics struct {
	// TotalPhysicalMemoryBytes represents the total physical memory on the system in bytes
	// This metric corresponds to memoryTotal from New Relic
	// Query source: sys.dm_os_sys_memory for system memory statistics
	TotalPhysicalMemoryBytes *float64 `db:"total_physical_memory" metric_name:"sqlserver.instance.memoryTotal" source_type:"gauge"`

	// AvailablePhysicalMemoryBytes represents the available physical memory on the system in bytes
	// This metric corresponds to memoryAvailable from New Relic
	// Query source: sys.dm_os_sys_memory for system memory statistics
	AvailablePhysicalMemoryBytes *float64 `db:"available_physical_memory" metric_name:"sqlserver.instance.memoryAvailable" source_type:"gauge"`

	// MemoryUtilizationPercent represents the percentage of memory utilization
	// This metric corresponds to memoryUtilization from New Relic
	// Query source: calculated from sys.dm_os_process_memory and sys.dm_os_sys_memory
	MemoryUtilizationPercent *float64 `db:"memory_utilization" metric_name:"sqlserver.instance.memoryUtilization" source_type:"gauge"`
}
