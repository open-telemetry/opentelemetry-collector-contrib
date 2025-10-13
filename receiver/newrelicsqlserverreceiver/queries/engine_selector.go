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
	UserConnectionQueries
	FailoverClusterQueries
	DatabasePrincipalsQueries
	DatabaseRoleMembershipQueries
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
	{
		Query:       BufferPoolHitPercentMetricsQuery,
		MetricName:  "sqlserver.instance.buffer_pool.hitPercent",
		Description: "Buffer pool hit percentage",
	},
	{
		Query:       InstanceProcessCountMetricsQuery,
		MetricName:  "sqlserver.instance.process_counts",
		Description: "SQL Server instance process counts",
	},
	{
		Query:       RunnableTasksMetricsQuery,
		MetricName:  "sqlserver.instance.runnable_tasks",
		Description: "Number of runnable tasks",
	},
	{
		Query:       InstanceDiskMetricsQuery,
		MetricName:  "sqlserver.instance.disk_metrics",
		Description: "SQL Server instance disk metrics",
	},
	{
		Query:       InstanceActiveConnectionsMetricsQuery,
		MetricName:  "sqlserver.instance.active_connections",
		Description: "SQL Server instance active connections",
	},
}

var instanceQueriesAzureManagedDatabase = []*QueryDefinition{
	{
		Query:       InstanceBufferPoolQuery,
		MetricName:  "sqlserver.instance.buffer_pool_size",
		Description: "Buffer pool size in bytes",
	},
	{
		Query:       BufferPoolHitPercentMetricsQuery,
		MetricName:  "sqlserver.instance.buffer_pool.hitPercent",
		Description: "Buffer pool hit percentage",
	},
	{
		Query:       InstanceProcessCountMetricsQuery,
		MetricName:  "sqlserver.instance.process_counts",
		Description: "SQL Server instance process counts",
	},
	{
		Query:       RunnableTasksMetricsQuery,
		MetricName:  "sqlserver.instance.runnable_tasks",
		Description: "Number of runnable tasks",
	},
	{
		Query:       InstanceDiskMetricsQuery,
		MetricName:  "sqlserver.instance.disk_metrics",
		Description: "SQL Server instance disk metrics",
	},
	{
		Query:       InstanceActiveConnectionsMetricsQuery,
		MetricName:  "sqlserver.instance.active_connections",
		Description: "SQL Server instance active connections",
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
	{
		Query:       BufferPoolHitPercentMetricsQuery,
		MetricName:  "sqlserver.instance.buffer_pool.hitPercent",
		Description: "Buffer pool hit percentage",
	},
	{
		Query:       InstanceProcessCountMetricsQuery,
		MetricName:  "sqlserver.instance.process_counts",
		Description: "SQL Server instance process counts",
	},
	{
		Query:       RunnableTasksMetricsQuery,
		MetricName:  "sqlserver.instance.runnable_tasks",
		Description: "Number of runnable tasks",
	},
	{
		Query:       InstanceDiskMetricsQuery,
		MetricName:  "sqlserver.instance.disk_metrics",
		Description: "SQL Server instance disk metrics",
	},
	{
		Query:       InstanceActiveConnectionsMetricsQuery,
		MetricName:  "sqlserver.instance.active_connections",
		Description: "SQL Server instance active connections",
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

// Query definitions for user connection metrics (Default/Standard SQL Server)
var userConnectionQueriesDefault = []*QueryDefinition{
	{
		Query:       UserConnectionStatusQuery,
		MetricName:  "sqlserver.user_connections.status.metrics",
		Description: "User connection status distribution",
	},
	{
		Query:       LoginLogoutQuery,
		MetricName:  "sqlserver.user_connections.authentication.metrics",
		Description: "Login/logout rate metrics",
	},
	{
		Query:       LoginLogoutSummaryQuery,
		MetricName:  "sqlserver.user_connections.authentication.summary",
		Description: "Login/logout summary statistics",
	},
	{
		Query:       FailedLoginQuery,
		MetricName:  "sqlserver.user_connections.failed_logins.metrics",
		Description: "Failed login attempts from error log",
	},
	{
		Query:       FailedLoginSummaryQuery,
		MetricName:  "sqlserver.user_connections.failed_logins_summary.metrics",
		Description: "Failed login summary statistics",
	},
	{
		Query:       UserConnectionSummaryQuery,
		MetricName:  "sqlserver.user_connections.status.summary",
		Description: "User connection summary statistics",
	},
	{
		Query:       UserConnectionUtilizationQuery,
		MetricName:  "sqlserver.user_connections.utilization",
		Description: "User connection utilization metrics",
	},
	{
		Query:       UserConnectionByClientQuery,
		MetricName:  "sqlserver.user_connections.by_client",
		Description: "User connections grouped by client",
	},
	{
		Query:       UserConnectionClientSummaryQuery,
		MetricName:  "sqlserver.user_connections.client.summary",
		Description: "User connection client summary statistics",
	},
}

// Query definitions for user connection metrics (Azure SQL Database)
var userConnectionQueriesAzureDatabase = []*QueryDefinition{
	{
		Query:       UserConnectionStatusQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.status.metrics",
		Description: "User connection status distribution (Azure SQL Database)",
	},
	{
		Query:       LoginLogoutQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.authentication.metrics",
		Description: "Login/logout rate metrics (Azure SQL Database)",
	},
	{
		Query:       LoginLogoutSummaryQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.authentication.summary",
		Description: "Login/logout summary statistics (Azure SQL Database)",
	},
	{
		Query:       FailedLoginQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.failed_logins.metrics",
		Description: "Failed login attempts from sys.event_log (Azure SQL Database)",
	},
	{
		Query:       FailedLoginSummaryQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.failed_logins_summary.metrics",
		Description: "Failed login summary statistics (Azure SQL Database)",
	},
	{
		Query:       UserConnectionSummaryQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.status.summary",
		Description: "User connection summary statistics (Azure SQL Database)",
	},
	{
		Query:       UserConnectionUtilizationQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.utilization",
		Description: "User connection utilization metrics (Azure SQL Database)",
	},
	{
		Query:       UserConnectionByClientQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.by_client",
		Description: "User connections grouped by client (Azure SQL Database)",
	},
	{
		Query:       UserConnectionClientSummaryQueryAzureSQL,
		MetricName:  "sqlserver.user_connections.client.summary",
		Description: "User connection client summary statistics (Azure SQL Database)",
	},
}

// Query definitions for user connection metrics (Azure SQL Managed Instance)
var userConnectionQueriesAzureManagedInstance = []*QueryDefinition{
	{
		Query:       UserConnectionStatusQueryAzureMI,
		MetricName:  "sqlserver.user_connections.status.metrics",
		Description: "User connection status distribution (Azure SQL Managed Instance)",
	},
	{
		Query:       LoginLogoutQueryAzureMI,
		MetricName:  "sqlserver.user_connections.authentication.metrics",
		Description: "Login/logout rate metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       LoginLogoutSummaryQueryAzureMI,
		MetricName:  "sqlserver.user_connections.authentication.summary",
		Description: "Login/logout summary statistics (Azure SQL Managed Instance)",
	},
	{
		Query:       FailedLoginQueryAzureMI,
		MetricName:  "sqlserver.user_connections.failed_logins.metrics",
		Description: "Failed login attempts from error log (Azure SQL Managed Instance)",
	},
	{
		Query:       FailedLoginSummaryQueryAzureMI,
		MetricName:  "sqlserver.user_connections.failed_logins_summary.metrics",
		Description: "Failed login summary statistics (Azure SQL Managed Instance)",
	},
	{
		Query:       UserConnectionSummaryQueryAzureMI,
		MetricName:  "sqlserver.user_connections.status.summary",
		Description: "User connection summary statistics (Azure SQL Managed Instance)",
	},
	{
		Query:       UserConnectionUtilizationQueryAzureMI,
		MetricName:  "sqlserver.user_connections.utilization",
		Description: "User connection utilization metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       UserConnectionByClientQueryAzureMI,
		MetricName:  "sqlserver.user_connections.by_client",
		Description: "User connections grouped by client (Azure SQL Managed Instance)",
	},
	{
		Query:       UserConnectionClientSummaryQueryAzureMI,
		MetricName:  "sqlserver.user_connections.client.summary",
		Description: "User connection client summary statistics (Azure SQL Managed Instance)",
	},
}

// Failover cluster query definitions for Default SQL Server
var failoverClusterQueriesDefault = []*QueryDefinition{
	{
		Query:       FailoverClusterReplicaQuery,
		MetricName:  "sqlserver.failover_cluster.replica_metrics",
		Description: "Always On Availability Group replica performance metrics",
	},
	{
		Query:       FailoverClusterReplicaStateQuery,
		MetricName:  "sqlserver.failover_cluster.replica_state_metrics",
		Description: "Always On Availability Group database replica state metrics",
	},
	{
		Query:       FailoverClusterNodeQuery,
		MetricName:  "sqlserver.failover_cluster.node_metrics",
		Description: "Windows Server Failover Cluster node information and status",
	},
	{
		Query:       FailoverClusterAvailabilityGroupHealthQuery,
		MetricName:  "sqlserver.failover_cluster.availability_group_health_metrics",
		Description: "Always On Availability Group health status metrics",
	},
	{
		Query:       FailoverClusterAvailabilityGroupQuery,
		MetricName:  "sqlserver.failover_cluster.availability_group_metrics",
		Description: "Always On Availability Group configuration metrics",
	},
	{
		Query:       FailoverClusterPerformanceCounterQuery,
		MetricName:  "sqlserver.failover_cluster.performance_counter_metrics",
		Description: "Always On Availability Group performance counter metrics",
	},
	{
		Query:       FailoverClusterClusterPropertiesQuery,
		MetricName:  "sqlserver.failover_cluster.cluster_properties_metrics",
		Description: "Windows Server Failover Cluster properties and quorum information",
	},
}

// Failover cluster query definitions for Azure SQL Database
var failoverClusterQueriesAzureManagedDatabase = []*QueryDefinition{
	{
		Query:       FailoverClusterReplicaQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.replica_metrics",
		Description: "Always On Availability Group replica metrics (not applicable for Azure SQL Database)",
	},
	{
		Query:       FailoverClusterReplicaStateQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.replica_state_metrics",
		Description: "Always On Availability Group replica state metrics (not applicable for Azure SQL Database)",
	},
	{
		Query:       FailoverClusterNodeQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.node_metrics",
		Description: "Cluster node metrics (not applicable for Azure SQL Database)",
	},
	{
		Query:       FailoverClusterAvailabilityGroupHealthQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.availability_group_health_metrics",
		Description: "Availability Group health metrics (not applicable for Azure SQL Database)",
	},
	{
		Query:       FailoverClusterAvailabilityGroupQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.availability_group_metrics",
		Description: "Availability Group configuration metrics (not applicable for Azure SQL Database)",
	},
	{
		Query:       FailoverClusterPerformanceCounterQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.performance_counter_metrics",
		Description: "Performance counter metrics (not applicable for Azure SQL Database)",
	},
	{
		Query:       FailoverClusterClusterPropertiesQueryAzureSQL,
		MetricName:  "sqlserver.failover_cluster.cluster_properties_metrics",
		Description: "Cluster properties metrics (not applicable for Azure SQL Database)",
	},
}

// Failover cluster query definitions for Azure SQL Managed Instance
var failoverClusterQueriesAzureManagedInstance = []*QueryDefinition{
	{
		Query:       FailoverClusterReplicaQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.replica_metrics",
		Description: "Always On Availability Group replica metrics (limited support for Azure SQL Managed Instance)",
	},
	{
		Query:       FailoverClusterReplicaStateQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.replica_state_metrics",
		Description: "Always On Availability Group replica state metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       FailoverClusterNodeQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.node_metrics",
		Description: "Cluster node metrics (limited support for Azure SQL Managed Instance)",
	},
	{
		Query:       FailoverClusterAvailabilityGroupHealthQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.availability_group_health_metrics",
		Description: "Availability Group health metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       FailoverClusterAvailabilityGroupQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.availability_group_metrics",
		Description: "Availability Group configuration metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       FailoverClusterPerformanceCounterQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.performance_counter_metrics",
		Description: "Performance counter metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       FailoverClusterClusterPropertiesQueryAzureMI,
		MetricName:  "sqlserver.failover_cluster.cluster_properties_metrics",
		Description: "Cluster properties metrics (limited support for Azure SQL Managed Instance)",
	},
}

// Database principals query definitions for Standard SQL Server
var databasePrincipalsQueriesDefault = []*QueryDefinition{
	{
		Query:       DatabasePrincipalsQuery,
		MetricName:  "sqlserver.database.principals.details",
		Description: "Database security principals information",
	},
	{
		Query:       DatabasePrincipalsSummaryQuery,
		MetricName:  "sqlserver.database.principals.summary",
		Description: "Database principals summary statistics",
	},
	{
		Query:       DatabasePrincipalActivityQuery,
		MetricName:  "sqlserver.database.principals.activity",
		Description: "Database principals activity and lifecycle metrics",
	},
}

// Database principals query definitions for Azure SQL Database
var databasePrincipalsQueriesAzureDatabase = []*QueryDefinition{
	{
		Query:       DatabasePrincipalsQueryAzureSQL,
		MetricName:  "sqlserver.database.principals.details",
		Description: "Database security principals information (Azure SQL Database)",
	},
	{
		Query:       DatabasePrincipalsSummaryQueryAzureSQL,
		MetricName:  "sqlserver.database.principals.summary",
		Description: "Database principals summary statistics (Azure SQL Database)",
	},
	{
		Query:       DatabasePrincipalActivityQueryAzureSQL,
		MetricName:  "sqlserver.database.principals.activity",
		Description: "Database principals activity and lifecycle metrics (Azure SQL Database)",
	},
}

// Database principals query definitions for Azure SQL Managed Instance
var databasePrincipalsQueriesAzureManagedInstance = []*QueryDefinition{
	{
		Query:       DatabasePrincipalsQueryAzureMI,
		MetricName:  "sqlserver.database.principals.details",
		Description: "Database security principals information (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabasePrincipalsSummaryQueryAzureMI,
		MetricName:  "sqlserver.database.principals.summary",
		Description: "Database principals summary statistics (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabasePrincipalActivityQueryAzureMI,
		MetricName:  "sqlserver.database.principals.activity",
		Description: "Database principals activity and lifecycle metrics (Azure SQL Managed Instance)",
	},
}

// Database role membership query definitions for Standard SQL Server
var databaseRoleMembershipQueriesDefault = []*QueryDefinition{
	{
		Query:       DatabaseRoleMembershipMetricsQuery,
		MetricName:  "database_role_membership",
		Description: "Database role membership relationships",
	},
	{
		Query:       DatabaseRoleMembershipSummaryQuery,
		MetricName:  "database_role_membership_summary",
		Description: "Database role membership summary statistics",
	},
	{
		Query:       DatabaseRoleHierarchyQuery,
		MetricName:  "database_role_hierarchy",
		Description: "Database role hierarchy and nesting information",
	},
	{
		Query:       DatabaseRoleActivityQuery,
		MetricName:  "database_role_activity",
		Description: "Database role activity and usage metrics",
	},
	{
		Query:       DatabaseRolePermissionMatrixQuery,
		MetricName:  "database_role_permission_matrix",
		Description: "Database role permission matrix analysis",
	},
}

// Database role membership query definitions for Azure SQL Database
var databaseRoleMembershipQueriesAzureDatabase = []*QueryDefinition{
	{
		Query:       DatabaseRoleMembershipMetricsQuery,
		MetricName:  "database_role_membership",
		Description: "Database role membership relationships (Azure SQL Database)",
	},
	{
		Query:       DatabaseRoleMembershipSummaryQuery,
		MetricName:  "database_role_membership_summary",
		Description: "Database role membership summary statistics (Azure SQL Database)",
	},
	{
		Query:       DatabaseRoleHierarchyQuery,
		MetricName:  "database_role_hierarchy",
		Description: "Database role hierarchy and nesting information (Azure SQL Database)",
	},
	{
		Query:       DatabaseRoleActivityQuery,
		MetricName:  "database_role_activity",
		Description: "Database role activity and usage metrics (Azure SQL Database)",
	},
	{
		Query:       DatabaseRolePermissionMatrixQuery,
		MetricName:  "database_role_permission_matrix",
		Description: "Database role permission matrix analysis (Azure SQL Database)",
	},
}

// Database role membership query definitions for Azure SQL Managed Instance
var databaseRoleMembershipQueriesAzureManagedInstance = []*QueryDefinition{
	{
		Query:       DatabaseRoleMembershipMetricsQuery,
		MetricName:  "database_role_membership",
		Description: "Database role membership relationships (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabaseRoleMembershipSummaryQuery,
		MetricName:  "database_role_membership_summary",
		Description: "Database role membership summary statistics (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabaseRoleHierarchyQuery,
		MetricName:  "database_role_hierarchy",
		Description: "Database role hierarchy and nesting information (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabaseRoleActivityQuery,
		MetricName:  "database_role_activity",
		Description: "Database role activity and usage metrics (Azure SQL Managed Instance)",
	},
	{
		Query:       DatabaseRolePermissionMatrixQuery,
		MetricName:  "database_role_permission_matrix",
		Description: "Database role permission matrix analysis (Azure SQL Managed Instance)",
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
	UserConnectionQueries: {
		Default:                 userConnectionQueriesDefault,
		AzureSQLDatabase:        userConnectionQueriesAzureDatabase,
		AzureSQLManagedInstance: userConnectionQueriesAzureManagedInstance,
	},
	FailoverClusterQueries: {
		Default:                 failoverClusterQueriesDefault,
		AzureSQLDatabase:        failoverClusterQueriesAzureManagedDatabase,
		AzureSQLManagedInstance: failoverClusterQueriesAzureManagedInstance,
	},
	DatabasePrincipalsQueries: {
		Default:                 databasePrincipalsQueriesDefault,
		AzureSQLDatabase:        databasePrincipalsQueriesAzureDatabase,
		AzureSQLManagedInstance: databasePrincipalsQueriesAzureManagedInstance,
	},
	DatabaseRoleMembershipQueries: {
		Default:                 databaseRoleMembershipQueriesDefault,
		AzureSQLDatabase:        databaseRoleMembershipQueriesAzureDatabase,
		AzureSQLManagedInstance: databaseRoleMembershipQueriesAzureManagedInstance,
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
