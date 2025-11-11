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

// SlowQuery represents slow query performance data collected from SQL Server
// This struct is modeled after nri-mssql's TopNSlowQueryDetails for compatibility
type SlowQuery struct {
	QueryID                *QueryID `db:"query_id" metric_name:"query_id" source_type:"attribute"`
	PlanHandle             *QueryID `db:"plan_handle" metric_name:"plan_handle" source_type:"attribute"`
	QueryText              *string  `db:"query_text" metric_name:"query_text" source_type:"attribute"`
	DatabaseName           *string  `db:"database_name" metric_name:"database_name" source_type:"attribute"`
	SchemaName             *string  `db:"schema_name" metric_name:"schema_name" source_type:"attribute"`
	LastExecutionTimestamp *string  `db:"last_execution_timestamp" metric_name:"last_execution_timestamp" source_type:"attribute"`
	ExecutionCount         *int64   `db:"execution_count" metric_name:"execution_count" source_type:"gauge"`
	AvgCPUTimeMS           *float64 `db:"avg_cpu_time_ms" metric_name:"avg_cpu_time_ms" source_type:"gauge"`
	AvgElapsedTimeMS       *float64 `db:"avg_elapsed_time_ms" metric_name:"avg_elapsed_time_ms" source_type:"gauge"`
	AvgDiskReads           *float64 `db:"avg_disk_reads" metric_name:"avg_disk_reads" source_type:"gauge"`
	AvgDiskWrites          *float64 `db:"avg_disk_writes" metric_name:"avg_disk_writes" source_type:"gauge"`
	StatementType          *string  `db:"statement_type" metric_name:"statement_type" source_type:"attribute"`
	CollectionTimestamp    *string  `db:"collection_timestamp" metric_name:"collection_timestamp" source_type:"attribute"`
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

// WaitTimeAnalysis represents wait time analysis data for SQL Server queries
type WaitTimeAnalysis struct {
	QueryID             *QueryID   `db:"query_id" metric_name:"query_id" source_type:"attribute"`
	DatabaseName        *string    `db:"database_name" metric_name:"database_name" source_type:"attribute"`
	QueryText           *string    `db:"query_text" metric_name:"query_text" source_type:"attribute"`
	WaitCategory        *string    `db:"wait_category" metric_name:"wait_category" source_type:"attribute"`
	TotalWaitTimeMs     *float64   `db:"total_wait_time_ms" metric_name:"total_wait_time_ms" source_type:"gauge"`
	AvgWaitTimeMs       *float64   `db:"avg_wait_time_ms" metric_name:"avg_wait_time_ms" source_type:"gauge"`
	WaitEventCount      *int64     `db:"wait_event_count" metric_name:"wait_event_count" source_type:"gauge"`
	LastExecutionTime   *string    `db:"last_execution_time" metric_name:"last_execution_time" source_type:"attribute"`
	CollectionTimestamp *string    `db:"collection_timestamp" metric_name:"collection_timestamp" source_type:"attribute"`
}

// QueryExecutionPlan represents detailed execution plan analysis data for SQL Server queries
// This model is used for drill-down analysis from slow query detection to specific execution plans
type QueryExecutionPlan struct {
	QueryID            *QueryID `db:"query_id" metric_name:"query_id" source_type:"attribute"`
	PlanHandle         *QueryID `db:"plan_handle" metric_name:"plan_handle" source_type:"attribute"`
	QueryPlanID        *QueryID `db:"query_plan_id" metric_name:"query_plan_id" source_type:"attribute"`
	SQLText            *string  `db:"sql_text" metric_name:"sql_text" source_type:"attribute"`
	TotalCPUMs         *float64 `db:"total_cpu_ms" metric_name:"total_cpu_ms" source_type:"gauge"`
	TotalElapsedMs     *float64 `db:"total_elapsed_ms" metric_name:"total_elapsed_ms" source_type:"gauge"`
	CreationTime       *int64   `db:"creation_time" metric_name:"creation_time" source_type:"gauge"`
	LastExecutionTime  *int64   `db:"last_execution_time" metric_name:"last_execution_time" source_type:"gauge"`
	ExecutionPlanXML   *string  `db:"execution_plan_xml" metric_name:"execution_plan_xml" source_type:"attribute"`
}

// ExecutionPlanNode represents a parsed execution plan node with detailed operator information
// This model contains the parsed data structure from XML execution plans for New Relic logging
type ExecutionPlanNode struct {
	// Identifiers
	QueryID     string `json:"query_id"`
	PlanHandle  string `json:"plan_handle"`
	NodeID      int    `json:"node_id"`
	
	// SQL Query Information
	SQLText     string `json:"sql_text"`
	
	// Operator Information
	PhysicalOp  string  `json:"physical_op"`
	LogicalOp   string  `json:"logical_op"`
	
	// Cost Estimates
	EstimateRows           float64 `json:"estimate_rows"`
	EstimateIO             float64 `json:"estimate_io"`
	EstimateCPU            float64 `json:"estimate_cpu"`
	AvgRowSize             float64 `json:"avg_row_size"`
	TotalSubtreeCost       float64 `json:"total_subtree_cost"`
	EstimatedOperatorCost  float64 `json:"estimated_operator_cost"`
	
	// Execution Details
	EstimatedExecutionMode string  `json:"estimated_execution_mode"`
	GrantedMemoryKb        int64   `json:"granted_memory_kb"`
	SpillOccurred          bool    `json:"spill_occurred"`
	NoJoinPredicate        bool    `json:"no_join_predicate"`
	
	// Performance Metrics
	TotalWorkerTime       float64 `json:"total_worker_time"`
	TotalElapsedTime      float64 `json:"total_elapsed_time"`
	TotalLogicalReads     int64   `json:"total_logical_reads"`
	TotalLogicalWrites    int64   `json:"total_logical_writes"`
	ExecutionCount        int64   `json:"execution_count"`
	AvgElapsedTimeMs      float64 `json:"avg_elapsed_time_ms"`
	
	// Timestamps
	CollectionTimestamp   string  `json:"collection_timestamp"`
	LastExecutionTime     string  `json:"last_execution_time"`
}

// ExecutionPlanAnalysis represents the complete parsed execution plan with metadata
type ExecutionPlanAnalysis struct {
	QueryID           string              `json:"query_id"`
	PlanHandle        string              `json:"plan_handle"`
	SQLText           string              `json:"sql_text"`
	TotalCost         float64             `json:"total_cost"`
	CompileTime       string              `json:"compile_time"`
	CompileCPU        int64               `json:"compile_cpu"`
	CompileMemory     int64               `json:"compile_memory"`
	Nodes             []ExecutionPlanNode `json:"nodes"`
	CollectionTime    string              `json:"collection_time"`
}
