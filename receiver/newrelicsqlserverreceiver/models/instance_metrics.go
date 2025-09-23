// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides data structures for instance-level metrics and system information.
// This file defines the data models used to represent SQL Server instance-level performance data
// including memory management, CPU utilization, I/O operations, connections, and transactions.
//
// Instance-Level Data Structures:
//
// 1. Memory Management Metrics:
//
//	type MemoryMetrics struct {
//	    TotalServerMemoryKB      int64   // Current memory used by SQL Server (KB)
//	    TargetServerMemoryKB     int64   // Target memory SQL Server wants to achieve (KB)
//	    BufferCacheHitRatio      float64 // Percentage of pages found in buffer cache
//	    PageLifeExpectancy       int64   // Average time pages stay in buffer cache (seconds)
//	    BufferPoolDataPages      int64   // Number of data pages in buffer pool
//	    BufferPoolFreePages      int64   // Number of free pages in buffer pool
//	    MemoryGrantsPending      int64   // Number of processes waiting for memory grants
//	    SQLCompilationRate       float64 // SQL compilations per second
//	}
//
// 2. CPU Utilization Metrics:
//
//	type CPUMetrics struct {
//	    CPUUsagePercentage       float64 // SQL Server CPU utilization percentage
//	    CPUUsageMs               int64   // Total CPU time consumed (milliseconds)
//	    SchedulerRunnableTasks   int64   // Tasks waiting in runnable queues
//	    ContextSwitchesPerSecond float64 // OS context switches per second
//	}
//
// 3. I/O Operation Metrics:
//
//	type IOMetrics struct {
//	    DiskReadIOPerSecond      float64 // Physical disk reads per second
//	    DiskWriteIOPerSecond     float64 // Physical disk writes per second
//	    DiskReadBytesPerSecond   float64 // Physical bytes read per second
//	    DiskWriteBytesPerSecond  float64 // Physical bytes written per second
//	    LogFlushWaitTimeMs       float64 // Average log flush wait time (milliseconds)
//	    LogFlushWaitsPerSecond   float64 // Log flush waits per second
//	    CheckpointPagesPerSecond float64 // Pages flushed by checkpoint per second
//	    LazyWritesPerSecond      float64 // Lazy writer writes per second
//	}
//
// 4. Connection Management Metrics:
//
//	type ConnectionMetrics struct {
//	    UserConnections          int64   // Current number of user connections
//	    ActiveConnections        int64   // Number of active connections
//	    LoginAttemptsPerSecond   float64 // Successful login attempts per second
//	    LoginFailuresPerSecond   float64 // Failed login attempts per second
//	}
//
// 5. Transaction Processing Metrics:
//
//	type TransactionMetrics struct {
//	    TransactionsPerSecond    float64 // Number of transactions per second
//	    ActiveTransactions       int64   // Currently active transactions
//	    BlockedProcesses         int64   // Number of currently blocked processes
//	    DeadlocksPerSecond       float64 // Number of deadlocks per second
//	}
//
// 6. Database Engine Metrics:
//
//	type EngineMetrics struct {
//	    BatchRequestsPerSecond   float64 // SQL batch requests per second
//	    SQLRecompilationsPerSecond float64 // SQL statement recompiles per second
//	    LockWaitsPerSecond       float64 // Lock requests that had to wait per second
//	    LockWaitTimeMs           float64 // Total wait time for locks (milliseconds)
//	}
//
// 7. Comprehensive Instance Metrics:
//
//	type InstanceMetrics struct {
//	    Memory          MemoryMetrics      // Memory management metrics (8 metrics)
//	    CPU             CPUMetrics         // CPU utilization metrics (4 metrics)
//	    IO              IOMetrics          // I/O operation metrics (8 metrics)
//	    Connections     ConnectionMetrics  // Connection management metrics (4 metrics)
//	    Transactions    TransactionMetrics // Transaction processing metrics (4 metrics)
//	    Engine          EngineMetrics      // Database engine metrics (4 metrics)
//	    CollectionTime  time.Time          // When metrics were collected
//	    ServerName      string             // SQL Server instance name
//	    EngineEdition   int                // SQL Server engine edition
//	    VersionInfo     string             // SQL Server version information
//	    ServicePack     string             // Service pack level
//	    ProductLevel    string             // Product level (RTM, SP1, etc.)
//	}
//
// 8. Performance Counter Data:
//
//	type PerformanceCounter struct {
//	    ObjectName      string  // Performance counter object name
//	    CounterName     string  // Performance counter name
//	    InstanceName    string  // Performance counter instance name
//	    CounterValue    int64   // Current counter value
//	    CounterType     int     // Counter type (from sys.dm_os_performance_counters)
//	    BaseValue       int64   // Base value for ratio counters
//	    Timestamp       time.Time // When counter was read
//	}
//
// 9. System Information:
//
//	type SystemInfo struct {
//	    MachineName          string    // Physical machine name
//	    ServerName           string    // SQL Server instance name
//	    EngineEdition        int       // Engine edition ID
//	    EngineEditionName    string    // Engine edition name
//	    ProductVersion       string    // SQL Server product version
//	    ProductLevel         string    // Product level (RTM, SP1, etc.)
//	    Edition              string    // SQL Server edition
//	    ProcessorCount       int       // Number of logical processors
//	    PhysicalMemoryMB     int64     // Total physical memory (MB)
//	    MaxServerMemoryMB    int64     // Max server memory setting (MB)
//	    MinServerMemoryMB    int64     // Min server memory setting (MB)
//	    Collation            string    // Server collation
//	    IsClustered          bool      // Whether instance is clustered
//	    IsHadrEnabled        bool      // Whether AlwaysOn is enabled
//	    StartupTime          time.Time // SQL Server startup time
//	}
//
// Data Source Mappings:
// - sys.dm_os_performance_counters: Performance counter data
// - sys.dm_os_sys_memory: System memory information
// - sys.dm_exec_sessions: Session and connection data
// - sys.dm_os_schedulers: CPU scheduler information
// - sys.dm_io_virtual_file_stats: I/O statistics
// - SERVERPROPERTY(): Server property information
// - @@VERSION: Version information
//
// Usage Patterns:
// - Populated by InstanceScraper from SQL Server DMVs
// - Converted to OpenTelemetry metrics with appropriate labels
// - Provides structured access to all 32 instance-level metrics
// - Enables consistent data handling across SQL Server editions
package models

type InstanceBufferMetricsModel struct {
	BufferPoolSize *int64 `db:"buffer_pool_size" metric_name:"sqlserver.instance.buffer_pool_size" source_type:"gauge" description:"Buffer pool size" unit:"By"`
}

type InstanceMemoryDefinitionsModel struct {
	TotalPhysicalMemory     *float64 `db:"total_physical_memory" metric_name:"sqlserver.instance.memory_total" source_type:"gauge" description:"Total physical memory available to SQL Server" unit:"By"`
	AvailablePhysicalMemory *float64 `db:"available_physical_memory" metric_name:"sqlserver.instance.memory_available" source_type:"gauge" description:"Available physical memory" unit:"By"`
	MemoryUtilization       *float64 `db:"memory_utilization" metric_name:"sqlserver.instance.memory_utilization" source_type:"gauge" description:"Memory utilization percentage" unit:"Percent"`
}

// InstanceStatsModel represents comprehensive instance statistics
// Note: SQL Server performance counters with "/sec" suffix are PERF_COUNTER_BULK_COUNT (272696576)
// which are cumulative counters. We calculate the delta/rate ourselves like nri-mssql does.
type InstanceStatsModel struct {
	SQLCompilations            *int64   `db:"sql_compilations" metric_name:"sqlserver.stats.sql_compilations_per_sec" source_type:"rate" description:"SQL compilations per second" unit:"1/s"`
	SQLRecompilations          *int64   `db:"sql_recompilations" metric_name:"sqlserver.stats.sql_recompilations_per_sec" source_type:"rate" description:"SQL recompilations per second" unit:"1/s"`
	UserConnections            *int64   `db:"user_connections" metric_name:"sqlserver.stats.connections" source_type:"gauge" description:"Current user connections" unit:"1"`
	LockWaitTimeMs             *int64   `db:"lock_wait_time_ms" metric_name:"sqlserver.stats.lock_waits_per_sec" source_type:"rate" description:"Lock waits per second" unit:"1/s"`
	PageSplitsSec              *int64   `db:"page_splits_sec" metric_name:"sqlserver.access.page_splits_per_sec" source_type:"rate" description:"Page splits per second" unit:"1/s"`
	CheckpointPagesSec         *int64   `db:"checkpoint_pages_sec" metric_name:"sqlserver.buffer.checkpoint_pages_per_sec" source_type:"rate" description:"Checkpoint pages per second" unit:"1/s"`
	DeadlocksSec               *int64   `db:"deadlocks_sec" metric_name:"sqlserver.stats.deadlocks_per_sec" source_type:"rate" description:"Deadlocks per second" unit:"1/s"`
	UserErrors                 *int64   `db:"user_errors" metric_name:"sqlserver.stats.user_errors_per_sec" source_type:"rate" description:"User errors per second" unit:"1/s"`
	KillConnectionErrors       *int64   `db:"kill_connection_errors" metric_name:"sqlserver.stats.kill_connection_errors_per_sec" source_type:"rate" description:"Kill connection errors per second" unit:"1/s"`
	BatchRequestSec            *int64   `db:"batch_request_sec" metric_name:"sqlserver.bufferpool.batch_requests_per_sec" source_type:"rate" description:"Batch requests per second" unit:"1/s"`
	PageLifeExpectancySec      *float64 `db:"page_life_expectancy_ms" metric_name:"sqlserver.bufferpool.page_life_expectancy_ms" source_type:"gauge" description:"Page life expectancy in milliseconds" unit:"ms"`
	TransactionsSec            *int64   `db:"transactions_sec" metric_name:"sqlserver.instance.transactions_per_sec" source_type:"rate" description:"Transactions per second" unit:"1/s"`
	ForcedParameterizationsSec *int64   `db:"forced_parameterizations_sec" metric_name:"sqlserver.instance.forced_parameterizations_per_sec" source_type:"rate" description:"Forced parameterizations per second" unit:"1/s"`
}

type BufferPoolHitPercentMetricsModel struct {
	BufferPoolHitPercent *float64 `db:"buffer_pool_hit_percent" metric_name:"sqlserver.bufferPoolHitPercent" source_type:"gauge"`
}

type InstanceProcessCountsModel struct {
	Preconnect *int64 `db:"preconnect" metric_name:"instance.preconnectProcessesCount" source_type:"gauge"`
	Background *int64 `db:"background" metric_name:"instance.backgroundProcessesCount" source_type:"gauge"`
	Dormant    *int64 `db:"dormant" metric_name:"instance.dormantProcessesCount" source_type:"gauge"`
	Runnable   *int64 `db:"runnable" metric_name:"instance.runnableProcessesCount" source_type:"gauge"`
	Suspended  *int64 `db:"suspended" metric_name:"instance.suspendedProcessesCount" source_type:"gauge"`
	Running    *int64 `db:"running" metric_name:"instance.runningProcessesCount" source_type:"gauge"`
	Blocked    *int64 `db:"blocked" metric_name:"instance.blockedProcessesCount" source_type:"gauge"`
	Sleeping   *int64 `db:"sleeping" metric_name:"instance.sleepingProcessesCount" source_type:"gauge"`
}

type RunnableTasksMetricsModel struct {
	RunnableTasksCount *int64 `db:"runnable_tasks_count" metric_name:"instance.runnableTasks" source_type:"gauge"`
}

type InstanceActiveConnectionsMetricsModel struct {
	InstanceActiveConnections *int64 `db:"instance_active_connections" metric_name:"activeConnections" source_type:"gauge"`
}

type InstanceDiskMetricsModel struct {
	TotalDiskSpace *int64 `db:"total_disk_space" metric_name:"instance.diskInBytes" source_type:"gauge"`
}
