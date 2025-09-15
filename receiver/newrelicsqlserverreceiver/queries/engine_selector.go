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
