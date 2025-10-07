// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package queries provides SQL query definitions for failover cluster-level metrics.
// This file contains SQL queries for collecting SQL Server Always On Availability Group
// replica performance metrics when running in high availability failover cluster environments.
//
// Failover Cluster Metrics Categories:
//
// 1. Database Replica Performance Metrics:
//   - Log bytes received per second from primary replica
//   - Transaction delay on secondary replica
//   - Flow control time for log record processing
//
// Query Sources:
// - sys.dm_os_performance_counters: Always On Database Replica performance counters
//
// Metric Collection Strategy:
// - Uses PIVOT operation to transform performance counter rows into columns
// - Filters for Database Replica object counters specific to Always On Availability Groups
// - Returns structured data for log replication performance monitoring
// - Provides insights into replication lag and flow control behavior
//
// Engine Support:
// - Default: Full failover cluster metrics for SQL Server with Always On Availability Groups
// - AzureSQLDatabase: Not applicable (no Always On AG support in single database)
// - AzureSQLManagedInstance: Limited support (managed service handles AG internally)
//
// Availability:
// - Only available on SQL Server instances configured with Always On Availability Groups
// - Returns empty result set on instances without Always On AG enabled
// - Requires appropriate permissions to query performance counter DMVs
package queries

// FailoverClusterReplicaQuery returns the SQL query for Always On replica performance metrics
// This query uses PIVOT operation to transform performance counter data into structured columns
// Source: sys.dm_os_performance_counters for Database Replica performance counters
//
// The query returns:
// - Log Bytes Received/sec: Rate of log records received by secondary replica from primary (bytes/sec)
// - Transaction Delay: Average delay for transactions on the secondary replica (milliseconds)
// - Flow Control Time (ms/sec): Time spent in flow control by log records (milliseconds/sec)
const FailoverClusterReplicaQuery = `SELECT
    [Log Bytes Received/sec],
    [Transaction Delay],
    [Flow Control Time (ms/sec)]
FROM
(
    -- Source data from Database Replica performance counters
    SELECT
        RTRIM(counter_name) AS counter_name,
        cntr_value
    FROM
        sys.dm_os_performance_counters
    WHERE
        object_name LIKE '%Database Replica%'
        AND counter_name IN (
            'Log Bytes Received/sec',
            'Transaction Delay',
            'Flow Control Time (ms/sec)'
        )
) AS SourceData
PIVOT
(
    MAX(cntr_value) -- Aggregate function for pivot operation
    FOR counter_name IN
    (
        -- Transform counter names into columns
        [Log Bytes Received/sec],
        [Transaction Delay],
        [Flow Control Time (ms/sec)]
    )
) AS PivotTable;`

// FailoverClusterReplicaQueryAzureSQL returns empty result for Azure SQL Database
// Always On Availability Groups are not supported in Azure SQL Database (single database model)
const FailoverClusterReplicaQueryAzureSQL = `SELECT 
    CAST(NULL AS BIGINT) AS [Log Bytes Received/sec],
    CAST(NULL AS BIGINT) AS [Transaction Delay], 
    CAST(NULL AS BIGINT) AS [Flow Control Time (ms/sec)]
WHERE 1=0` // Always returns empty result set

// FailoverClusterReplicaQueryAzureMI returns limited query for Azure SQL Managed Instance
// Azure SQL Managed Instance has built-in high availability but limited access to AG performance counters
const FailoverClusterReplicaQueryAzureMI = `SELECT 
    [Log Bytes Received/sec],
    [Transaction Delay],
    [Flow Control Time (ms/sec)]
FROM
(
    -- Source data from Database Replica performance counters (if available)
    SELECT
        RTRIM(counter_name) AS counter_name,
        cntr_value
    FROM
        sys.dm_os_performance_counters
    WHERE
        object_name LIKE '%Database Replica%'
        AND counter_name IN (
            'Log Bytes Received/sec',
            'Transaction Delay', 
            'Flow Control Time (ms/sec)'
        )
) AS SourceData
PIVOT
(
    MAX(cntr_value) -- Aggregate function for pivot operation
    FOR counter_name IN
    (
        -- Transform counter names into columns
        [Log Bytes Received/sec],
        [Transaction Delay],
        [Flow Control Time (ms/sec)]
    )
) AS PivotTable;`

// FailoverClusterReplicaStateQuery returns the SQL query for Always On replica state metrics
// This query joins availability replica information with database replica states to provide
// detailed log send/redo queue metrics for each database in the availability group
//
// The query returns:
// - replica_server_name: Name of the server hosting the replica
// - database_name: Name of the database in the availability group
// - log_send_queue_kb: Amount of log records not yet sent to secondary replica (KB)
// - redo_queue_kb: Amount of log records waiting to be redone on secondary replica (KB)
// - redo_rate_kb_sec: Rate at which log records are being redone on secondary replica (KB/sec)
const FailoverClusterReplicaStateQuery = `SELECT
    ar.replica_server_name,
    d.name AS database_name,
    drs.log_send_queue_size AS log_send_queue_kb,
    drs.redo_queue_size AS redo_queue_kb,
    drs.redo_rate AS redo_rate_kb_sec
FROM
    sys.dm_hadr_database_replica_states AS drs
JOIN
    sys.availability_replicas AS ar ON drs.replica_id = ar.replica_id
JOIN
    sys.databases AS d ON drs.database_id = d.database_id;`

// FailoverClusterReplicaStateQueryAzureSQL returns empty result for Azure SQL Database
// Always On Availability Groups are not supported in Azure SQL Database (single database model)
const FailoverClusterReplicaStateQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(256)) AS replica_server_name,
    CAST(NULL AS NVARCHAR(128)) AS database_name,
    CAST(NULL AS BIGINT) AS log_send_queue_kb,
    CAST(NULL AS BIGINT) AS redo_queue_kb,
    CAST(NULL AS BIGINT) AS redo_rate_kb_sec
WHERE 1=0` // Always returns empty result set

// FailoverClusterReplicaStateQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports Always On AG and should have access to these DMVs
const FailoverClusterReplicaStateQueryAzureMI = `SELECT
    ar.replica_server_name,
    d.name AS database_name,
    drs.log_send_queue_size AS log_send_queue_kb,
    drs.redo_queue_size AS redo_queue_kb,
    drs.redo_rate AS redo_rate_kb_sec
FROM
    sys.dm_hadr_database_replica_states AS drs
JOIN
    sys.availability_replicas AS ar ON drs.replica_id = ar.replica_id
JOIN
    sys.databases AS d ON drs.database_id = d.database_id;`

// FailoverClusterNodeQuery returns the SQL query for cluster node information
// This query retrieves information about cluster nodes including their status and ownership
//
// The query returns:
// - nodename: Name of the server node in the cluster
// - status_description: Health state of the node (Up, Down, Paused, etc.)
// - is_current_owner: 1 if this is the active node running SQL Server, 0 if passive
const FailoverClusterNodeQuery = `SELECT
    nodename,
    status_description,
    is_current_owner
FROM sys.dm_os_cluster_nodes;`

// FailoverClusterNodeQueryAzureSQL returns empty result for Azure SQL Database
// Cluster nodes are not applicable in Azure SQL Database (single database model)
const FailoverClusterNodeQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(256)) AS nodename,
    CAST(NULL AS NVARCHAR(128)) AS status_description,
    CAST(NULL AS BIT) AS is_current_owner
WHERE 1=0` // Always returns empty result set

// FailoverClusterNodeQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance has built-in clustering but may have limited access to cluster node details
const FailoverClusterNodeQueryAzureMI = `SELECT
    nodename,
    status_description,
    is_current_owner
FROM sys.dm_os_cluster_nodes;`

// FailoverClusterAvailabilityGroupHealthQuery returns the SQL query for Availability Group health status
// This query retrieves health and role information for all availability group replicas
//
// The query returns:
// - replica_server_name: Name of the server instance hosting the availability replica
// - role_desc: Current role of the replica (PRIMARY, SECONDARY)
// - synchronization_health_desc: Health of data synchronization (HEALTHY, PARTIALLY_HEALTHY, NOT_HEALTHY)
const FailoverClusterAvailabilityGroupHealthQuery = `SELECT
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc
FROM
    sys.dm_hadr_availability_replica_states AS ars
INNER JOIN
    sys.availability_replicas AS ar ON ars.replica_id = ar.replica_id;`

// FailoverClusterAvailabilityGroupHealthQueryAzureSQL returns empty result for Azure SQL Database
// Always On Availability Groups are not supported in Azure SQL Database
const FailoverClusterAvailabilityGroupHealthQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(256)) AS replica_server_name,
    CAST(NULL AS NVARCHAR(60)) AS role_desc,
    CAST(NULL AS NVARCHAR(60)) AS synchronization_health_desc
WHERE 1=0` // Always returns empty result set

// FailoverClusterAvailabilityGroupHealthQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports Always On AG and should have access to these DMVs
const FailoverClusterAvailabilityGroupHealthQueryAzureMI = `SELECT
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc
FROM
    sys.dm_hadr_availability_replica_states AS ars
INNER JOIN
    sys.availability_replicas AS ar ON ars.replica_id = ar.replica_id;`
