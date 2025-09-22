// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides data structures for performance monitoring metrics and query results.
// This file defines the data models used to represent SQL Server performance monitoring data
// including slow queries, wait statistics, blocking sessions, and execution plan information.
//
// Performance Data Structures:
//
// 1. Slow Query Information:
//
//	type SlowQuery struct {
//	    QueryText           string        // SQL query text
//	    QueryHash           string        // Query hash for identification
//	    PlanHash            string        // Execution plan hash
//	    ExecutionCount      int64         // Number of executions
//	    TotalElapsedTime    time.Duration // Total elapsed time
//	    AvgElapsedTime      time.Duration // Average elapsed time per execution
//	    TotalCPUTime        time.Duration // Total CPU time consumed
//	    AvgCPUTime          time.Duration // Average CPU time per execution
//	    TotalLogicalReads   int64         // Total logical read operations
//	    AvgLogicalReads     int64         // Average logical reads per execution
//	    TotalPhysicalReads  int64         // Total physical read operations
//	    AvgPhysicalReads    int64         // Average physical reads per execution
//	    TotalWrites         int64         // Total write operations
//	    AvgWrites           int64         // Average writes per execution
//	    CompileTime         time.Duration // Time spent compiling the query
//	    RecompileCount      int64         // Number of recompiles
//	    LastExecutionTime   time.Time     // Timestamp of last execution
//	    CreationTime        time.Time     // Timestamp when plan was created
//	}
//
// 2. Wait Statistics Information:
//
//	type WaitStatistic struct {
//	    WaitType            string        // Type of wait (e.g., PAGEIOLATCH_SH)
//	    WaitCategory        string        // Wait category (CPU, I/O, Network, etc.)
//	    WaitingTasksCount   int64         // Number of waits on this wait type
//	    WaitTimeMs          int64         // Total wait time in milliseconds
//	    MaxWaitTimeMs       int64         // Maximum wait time for single wait
//	    SignalWaitTimeMs    int64         // Signal wait time (CPU scheduling)
//	    ResourceWaitTimeMs  int64         // Resource wait time (actual resource wait)
//	    WaitTimePerSecond   float64       // Wait time per second (calculated)
//	    PercentageTotal     float64       // Percentage of total wait time
//	}
//
// 3. Blocking Session Information:
//
//	type BlockingSession struct {
//	    BlockedSessionID    int           // Session ID of blocked session
//	    BlockingSessionID   int           // Session ID of blocking session
//	    BlockedSPID         int           // Process ID of blocked session
//	    BlockingSPID        int           // Process ID of blocking session
//	    WaitType            string        // Type of wait causing the block
//	    WaitResource        string        // Resource being waited for
//	    WaitTime            time.Duration // How long the session has been blocked
//	    BlockedLoginName    string        // Login name of blocked session
//	    BlockingLoginName   string        // Login name of blocking session
//	    BlockedHostName     string        // Host name of blocked session client
//	    BlockingHostName    string        // Host name of blocking session client
//	    BlockedProgramName  string        // Program name of blocked session
//	    BlockingProgramName string        // Program name of blocking session
//	    BlockedCommand      string        // Command being executed by blocked session
//	    BlockingCommand     string        // Command being executed by blocking session
//	    BlockedStatus       string        // Status of blocked session
//	    BlockingStatus      string        // Status of blocking session
//	    IsDeadlock          bool          // Whether this is part of a deadlock
//	    BlockingLevel       int           // Level in blocking chain (head blocker = 0)
//	}
//
// 4. Execution Plan Cache Information:
//
//	type ExecutionPlanCache struct {
//	    PlanType            string        // Type of plan (Adhoc, Prepared, etc.)
//	    CacheObjectType     string        // Cache object type (Compiled Plan, etc.)
//	    ObjectName          string        // Name of cached object
//	    PlanHandle          string        // Handle to the execution plan
//	    UseCounts           int64         // Number of times plan has been used
//	    SizeInBytes         int64         // Size of cached plan in bytes
//	    PlanAge             time.Duration // Age of the plan in cache
//	    CreationTime        time.Time     // When the plan was created
//	    LastUsedTime        time.Time     // When the plan was last used
//	    IsParameterized     bool          // Whether the plan is parameterized
//	    ParameterList       string        // List of parameters (if any)
//	}
//
// 5. Performance Summary Metrics:
//
//	type PerformanceMetrics struct {
//	    SlowQueries         []SlowQuery         // Collection of slow queries
//	    WaitStatistics      []WaitStatistic     // Collection of wait statistics
//	    BlockingSessions    []BlockingSession   // Collection of blocking sessions
//	    PlanCacheStats      []ExecutionPlanCache // Collection of plan cache statistics
//	    CollectionTime      time.Time           // When metrics were collected
//	    ServerName          string              // SQL Server instance name
//	    DatabaseName        string              // Database name (if applicable)
//	    EngineEdition       int                 // SQL Server engine edition
//	}
//
// Common Field Types:
// - time.Duration: Used for time measurements (elapsed time, wait time, etc.)
// - time.Time: Used for timestamps (creation time, last execution, etc.)
// - int64: Used for counters and large numeric values
// - float64: Used for calculated percentages and rates
// - string: Used for text fields, identifiers, and names
// - bool: Used for boolean flags and status indicators
//
// Usage in Scrapers:
// - Scrapers populate these structures from SQL query results
// - Structures are converted to OpenTelemetry metrics
// - Provides type safety and clear data contracts
// - Enables consistent data handling across different engines
package models

import (
	"time"
)

// SlowQuery represents a slow query execution statistics
type SlowQuery struct {
	QueryHash          string    `db:"query_hash"`
	QueryPlanHash      string    `db:"query_plan_hash"`
	ExecutionCount     int64     `db:"execution_count"`
	TotalElapsedTime   int64     `db:"total_elapsed_time"`
	AvgElapsedTime     int64     `db:"avg_elapsed_time"`
	TotalCpuTime       int64     `db:"total_cpu_time"`
	AvgCpuTime         int64     `db:"avg_cpu_time"`
	TotalLogicalReads  int64     `db:"total_logical_reads"`
	AvgLogicalReads    int64     `db:"avg_logical_reads"`
	TotalPhysicalReads int64     `db:"total_physical_reads"`
	AvgPhysicalReads   int64     `db:"avg_physical_reads"`
	TotalWrites        int64     `db:"total_writes"`
	AvgWrites          int64     `db:"avg_writes"`
	QueryTextTruncated string    `db:"query_text_truncated"`
	LastExecutionTime  time.Time `db:"last_execution_time"`
	CreationTime       time.Time `db:"creation_time"`
}

// BlockingSession represents blocking session information
type BlockingSession struct {
	BlockingSPID          *int64   `db:"blocking_spid" metric_name:"sqlserver.blocking.spid" source_type:"gauge"`
	BlockingStatus        *string  `db:"blocking_status" metric_name:"sqlserver.blocking.status" source_type:"attribute"`
	BlockedSPID           *int64   `db:"blocked_spid" metric_name:"sqlserver.blocked.spid" source_type:"gauge"`
	BlockedStatus         *string  `db:"blocked_status" metric_name:"sqlserver.blocked.status" source_type:"attribute"`
	WaitType              *string  `db:"wait_type" metric_name:"sqlserver.wait.type" source_type:"attribute"`
	WaitTimeInSeconds     *float64 `db:"wait_time_in_seconds" metric_name:"sqlserver.wait.time_seconds" source_type:"gauge"`
	CommandType           *string  `db:"command_type" metric_name:"sqlserver.command.type" source_type:"attribute"`
	DatabaseName          *string  `db:"database_name" metric_name:"sqlserver.database.name" source_type:"attribute"`
	BlockingQueryText     *string  `db:"blocking_query_text" metric_name:"sqlserver.blocking.query_text" source_type:"attribute"`
	BlockedQueryText      *string  `db:"blocked_query_text" metric_name:"sqlserver.blocked.query_text" source_type:"attribute"`
	BlockedQueryStartTime *string  `db:"blocked_query_start_time" metric_name:"sqlserver.blocked.query_start_time" source_type:"attribute"`
}
