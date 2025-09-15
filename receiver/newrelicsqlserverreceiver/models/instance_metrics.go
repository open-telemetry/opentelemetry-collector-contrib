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

// InstanceBufferMetrics represents buffer pool metrics result
// This is the currently implemented instance metric following the nri-mssql pattern
type InstanceBufferMetrics struct {
	InstanceBufferPoolSize *int64 `db:"instance_buffer_pool_size" metric_name:"sqlserver.buffer_pool.size_bytes" source_type:"gauge"`
}
