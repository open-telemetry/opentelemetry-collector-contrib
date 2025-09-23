// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides the EngineSet pattern for SQL Server engine-specific query selection.
// This file implements the generic EngineSet structure that allows different query sets
// based on the SQL Server engine edition (Default, Azure SQL Database, Azure SQL Managed Instance).
//
// EngineSet Pattern Implementation:
//
//	type EngineSet[T any] struct {
//	    Default                 T  // Standard SQL Server queries
//	    AzureSQLDatabase        T  // Azure SQL Database specific queries
//	    AzureSQLManagedInstance T  // Azure SQL Managed Instance specific queries
//	}
//
//	func (s EngineSet[T]) Select(engineEdition int) T {
//	    switch engineEdition {
//	    case database.AzureSQLDatabaseEngineEditionNumber:
//	        return s.AzureSQLDatabase
//	    case database.AzureSQLManagedInstanceEngineEditionNumber:
//	        return s.AzureSQLManagedInstance
//	    default:
//	        return s.Default
//	    }
//	}
//
// Query Definition Types:
// - PerformanceQueries: Query sets for performance monitoring
// - InstanceQueries: Query sets for 32 instance-level metrics
// - DatabaseQueries: Query sets for 9 database-level metrics
//
// Usage Pattern:
//
//	var queryDefinitionSets = map[QueryDefinitionType]EngineSet[[]*QueryDefinition]{
//	    PerformanceQueries: {
//	        Default:                 performanceQueriesDefault,
//	        AzureSQLDatabase:        performanceQueriesAzureDB,
//	        AzureSQLManagedInstance: performanceQueriesAzureMI,
//	    },
//	    InstanceQueries: {
//	        Default:                 instanceQueriesDefault,
//	        AzureSQLDatabase:        instanceQueriesAzureDB,
//	        AzureSQLManagedInstance: instanceQueriesAzureMI,
//	    },
//	    // ... additional query sets
//	}
//
// Engine Detection:
// - Query sys.dm_database_service_objectives for Azure SQL Database
// - Query @@VERSION and SERVERPROPERTY() for engine edition identification
// - Default to standard SQL Server if detection fails
//
// Engine-Specific Limitations:
// - Azure SQL Database: No OS-level DMVs, limited instance-level metrics
// - Azure SQL Managed Instance: Most DMVs available, some OS limitations
// - Standard SQL Server: Full access to all DMVs and system objects
package queries

import (
	"fmt"
)

// Engine edition constants for SQL Server engine identification
const (
	// StandardSQLServerEngineEdition represents on-premises SQL Server
	StandardSQLServerEngineEdition = 0
	// AzureSQLDatabaseEngineEdition represents Azure SQL Database
	AzureSQLDatabaseEngineEdition = 5
	// AzureSQLManagedInstanceEngineEdition represents Azure SQL Managed Instance
	AzureSQLManagedInstanceEngineEdition = 8
)

// EngineSet is a generic struct that acts as a "bucket" for holding
// the default and Azure-specific implementations for a given resource.
type EngineSet[T any] struct {
	Default                 T
	AzureSQLDatabase        T
	AzureSQLManagedInstance T
}

// Select returns the correct implementation from the set based on the engine edition.
func (s EngineSet[T]) Select(engineEdition int) T {
	switch engineEdition {
	case AzureSQLDatabaseEngineEdition:
		return s.AzureSQLDatabase
	case AzureSQLManagedInstanceEngineEdition:
		return s.AzureSQLManagedInstance
	default:
		return s.Default
	}
}

// QueryDefinitionType is a custom type for identifying different query sets.
type QueryDefinitionType int

// Enum of the different query definition types.
const (
	InstanceQueries = iota
	DatabaseQueries
	PerformanceQueries
)

// QueryDefinition represents a SQL query with metadata
type QueryDefinition struct {
	Query       string
	MetricName  string
	Description string
}

// DetectEngineEdition queries the SQL Server to determine the engine edition
func DetectEngineEdition(queryFunc func(string) (int, error)) (int, error) {
	// Query to detect engine edition
	// SERVERPROPERTY('EngineEdition') returns:
	// 1 = Personal or Desktop Engine
	// 2 = Standard
	// 3 = Enterprise
	// 4 = Express
	// 5 = Azure SQL Database
	// 6 = Azure SQL Data Warehouse (now Azure Synapse Analytics)
	// 8 = Azure SQL Managed Instance
	query := "SELECT CAST(SERVERPROPERTY('EngineEdition') AS INT) AS EngineEdition"

	engineEdition, err := queryFunc(query)
	if err != nil {
		// Default to standard SQL Server if detection fails
		return StandardSQLServerEngineEdition, err
	}

	return engineEdition, nil
}

// Query definitions for instance metrics
var instanceQueriesDefault = []*QueryDefinition{
	{
		Query:       InstanceBufferPoolQuery,
		MetricName:  "sqlserver.instance.buffer_pool_size",
		Description: "Buffer pool size in bytes",
	},
	{
		Query: `SELECT COUNT(*) as user_connections 
		FROM sys.dm_exec_sessions 
		WHERE is_user_process = 1`,
		MetricName:  "sqlserver.instance.user_connections",
		Description: "Current number of user connections",
	},
	{
		Query: `SELECT cntr_value as page_life_expectancy
		FROM sys.dm_os_performance_counters 
		WHERE counter_name = 'Page life expectancy' 
		AND object_name LIKE '%Buffer Manager%'`,
		MetricName:  "sqlserver.instance.page_life_expectancy",
		Description: "Page life expectancy in seconds",
	},
	{
		Query:       InstanceMemoryDefinitions,
		MetricName:  "sqlserver.instance.memory_metrics",
		Description: "SQL Server instance memory metrics",
	},
	{
		Query:       InstanceStatsQuery,
		MetricName:  "sqlserver.instance.comprehensive_stats",
		Description: "Comprehensive SQL Server instance statistics",
	},
}

var instanceQueriesAzureManagedDatabase = []*QueryDefinition{
	{
		Query:       InstanceBufferPoolQuery,
		MetricName:  "sqlserver.instance.buffer_pool_size",
		Description: "Buffer pool size in bytes",
	},
}

var instanceQueriesAzureManagedInstance = []*QueryDefinition{
	{
		Query:       InstanceBufferPoolQuery,
		MetricName:  "sqlserver.instance.buffer_pool_size",
		Description: "Buffer pool size in bytes",
	},
	{
		Query:       InstanceMemoryDefinitions,
		MetricName:  "sqlserver.instance.memory_metrics",
		Description: "SQL Server instance memory metrics for Azure SQL Managed Instance",
	},
	{
		Query:       InstanceStatsQuery,
		MetricName:  "sqlserver.instance.comprehensive_stats",
		Description: "Comprehensive SQL Server instance statistics",
	},
}

// Database-level query definitions for Default SQL Server
var databaseQueriesDefault = []*QueryDefinition{
	{
		Query:       DatabaseBufferPoolQuery,
		MetricName:  "sqlserver.database.buffer_pool_size",
		Description: "Buffer pool size per database in bytes",
	},
	{
		Query:       DatabaseIOStallQuery,
		MetricName:  "sqlserver.database.io_stall",
		Description: "Total IO stall time for the database in milliseconds",
	},
	{
		Query:       DatabaseLogGrowthQuery,
		MetricName:  "sqlserver.database.log_growth",
		Description: "Number of log growth events for the database",
	},
	{
		Query:       DatabasePageFileQuery,
		MetricName:  "sqlserver.database.page_file_available",
		Description: "Available page file space (reserved space not used) for the database in bytes",
	},
	{
		Query:       DatabasePageFileTotalQuery,
		MetricName:  "sqlserver.database.page_file_total",
		Description: "Total page file space (total reserved space) for the database in bytes",
	},
	{
		Query:       DatabaseListQuery,
		MetricName:  "sqlserver.database.list",
		Description: "List of user databases for metric collection",
	},
}

// Database-level query definitions for Azure SQL Database
var databaseQueriesAzureManagedDatabase = []*QueryDefinition{
	{
		Query:       DatabaseBufferPoolQuery,
		MetricName:  "sqlserver.database.buffer_pool_size",
		Description: "Buffer pool size per database in bytes (Azure SQL Database)",
	},
	{
		Query:       DatabaseMaxDiskSizeQueryAzureSQL,
		MetricName:  "sqlserver.database.max_disk_size",
		Description: "Maximum disk size allowed for the database in bytes (Azure SQL Database)",
	},
	{
		Query:       DatabaseIOStallQueryAzureSQL,
		MetricName:  "sqlserver.database.io_stall",
		Description: "Total IO stall time for the database in milliseconds (Azure SQL Database)",
	},
	{
		Query:       DatabaseLogGrowthQueryAzureSQL,
		MetricName:  "sqlserver.database.log_growth",
		Description: "Number of log growth events for the database (Azure SQL Database)",
	},
	{
		Query:       DatabasePageFileQueryAzureSQL,
		MetricName:  "sqlserver.database.page_file_available",
		Description: "Available page file space (reserved space not used) for the database in bytes (Azure SQL Database)",
	},
	{
		Query:       DatabasePageFileTotalQueryAzureSQL,
		MetricName:  "sqlserver.database.page_file_total",
		Description: "Total page file space (total reserved space) for the database in bytes (Azure SQL Database)",
	},
	{
		Query:       DatabaseMemoryQueryAzureSQL,
		MetricName:  "sqlserver.database_memory_metrics",
		Description: "Available physical memory on the system in bytes (Azure SQL Database)",
	},
	{
		Query:       DatabaseListQueryAzureSQL,
		MetricName:  "sqlserver.database.list",
		Description: "List of user databases for metric collection (Azure SQL Database)",
	},
}

// Database-level query definitions for Azure SQL Managed Instance
var databaseQueriesAzureManagedInstance = []*QueryDefinition{
	{
		Query:       DatabaseBufferPoolQuery,
		MetricName:  "sqlserver.database.buffer_pool_size",
		Description: "Buffer pool size per database in bytes (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabaseIOStallQueryAzureMI,
		MetricName:  "sqlserver.database.io_stall",
		Description: "Total IO stall time for the database in milliseconds (Azure Managed Instance)",
	},
	{
		Query:       DatabaseLogGrowthQueryAzureMI,
		MetricName:  "sqlserver.database.log_growth",
		Description: "Number of log growth events for the database (Azure Managed Instance)",
	},
	{
		Query:       DatabasePageFileQueryAzureMI,
		MetricName:  "sqlserver.database.page_file_available",
		Description: "Available page file space (reserved space not used) for the database in bytes (Azure Managed Instance)",
	},
	{
		Query:       DatabasePageFileTotalQueryAzureMI,
		MetricName:  "sqlserver.database.page_file_total",
		Description: "Total page file space (total reserved space) for the database in bytes (Azure Managed Instance)",
	},
	{
		Query:       DatabaseListQueryAzureMI,
		MetricName:  "sqlserver.database.list",
		Description: "List of user databases for metric collection (Azure SQL Managed Instance)",
	},
}

// queryDefinitionSets maps query types to engine-specific query sets
var queryDefinitionSets = map[QueryDefinitionType]EngineSet[[]*QueryDefinition]{
	InstanceQueries: {
		Default:                 instanceQueriesDefault,
		AzureSQLDatabase:        instanceQueriesAzureManagedDatabase,
		AzureSQLManagedInstance: instanceQueriesAzureManagedInstance,
	},
	DatabaseQueries: {
		Default:                 databaseQueriesDefault,
		AzureSQLDatabase:        databaseQueriesAzureManagedDatabase,
		AzureSQLManagedInstance: databaseQueriesAzureManagedInstance,
	},
}

// GetQueryDefinitions returns the appropriate query definitions based on query type and engine edition
func GetQueryDefinitions(defType QueryDefinitionType, engineEdition int) []*QueryDefinition {
	definitionSet, ok := queryDefinitionSets[defType]
	if !ok {
		// Return empty slice if invalid query definition type
		return []*QueryDefinition{}
	}
	return definitionSet.Select(engineEdition)
}

// GetQueryForMetric retrieves the appropriate query for a metric based on query type and engine edition with Default fallback
func GetQueryForMetric(defType QueryDefinitionType, metricName string, engineEdition int) (string, bool) {
	// Strategy 1: Try engine-specific queries first
	queryDefs := GetQueryDefinitions(defType, engineEdition)
	for _, queryDef := range queryDefs {
		if queryDef.MetricName == metricName {
			return queryDef.Query, true
		}
	}

	// Strategy 2: Fallback to Default engine type queries if current engine doesn't have the specific query
	if engineEdition != StandardSQLServerEngineEdition {
		defaultQueryDefs := GetQueryDefinitions(defType, StandardSQLServerEngineEdition)
		for _, queryDef := range defaultQueryDefs {
			if queryDef.MetricName == metricName {
				return queryDef.Query, true
			}
		}
	}

	return "", false
}

// GetEngineTypeName returns a human-readable name for the engine edition
func GetEngineTypeName(engineEdition int) string {
	switch engineEdition {
	case StandardSQLServerEngineEdition:
		return "Standard SQL Server"
	case AzureSQLDatabaseEngineEdition:
		return "Azure SQL Database"
	case AzureSQLManagedInstanceEngineEdition:
		return "Azure SQL Managed Instance"
	default:
		return fmt.Sprintf("Unknown Engine Edition (%d)", engineEdition)
	}
}

// TruncateQuery truncates a query string for logging purposes
func TruncateQuery(query string, maxLen int) string {
	if len(query) <= maxLen {
		return query
	}
	return query[:maxLen] + "..."
}
