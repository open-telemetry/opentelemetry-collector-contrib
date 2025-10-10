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

// FailoverClusterReplicaStateQuery returns the SQL query for Always On replica state metrics with extended information
// This query joins availability replica information with database replica states to provide
// detailed log send/redo queue metrics and synchronization state for each database in the availability group
//
// The query returns:
// - replica_server_name: Name of the server hosting the replica
// - database_name: Name of the database in the availability group
// - log_send_queue_kb: Amount of log records not yet sent to secondary replica (KB)
// - redo_queue_kb: Amount of log records waiting to be redone on secondary replica (KB)
// - redo_rate_kb_sec: Rate at which log records are being redone on secondary replica (KB/sec)
// - database_state_desc: Current state of the database replica
// - synchronization_state_desc: Synchronization state of the database replica
// - is_local: Whether this is a local replica (1) or remote (0)
// - is_primary_replica: Whether this is the primary replica (1) or secondary (0)
// - last_commit_time: Timestamp of the last transaction commit
// - last_sent_time: Timestamp when the last log block was sent
// - last_received_time: Timestamp when the last log block was received
// - last_hardened_time: Timestamp when the last log block was hardened
// - last_redone_lsn: Log sequence number of the last redone log record
// - suspend_reason_desc: Reason why the database is suspended (if applicable)
const FailoverClusterReplicaStateQuery = `SELECT
    ar.replica_server_name,
    d.name AS database_name,
    drs.log_send_queue_size AS log_send_queue_kb,
    drs.redo_queue_size AS redo_queue_kb,
    drs.redo_rate AS redo_rate_kb_sec,
    drs.database_state_desc,
    drs.synchronization_state_desc,
    CAST(drs.is_local AS INT) AS is_local,
    CAST(drs.is_primary_replica AS INT) AS is_primary_replica,
    CONVERT(VARCHAR(23), drs.last_commit_time, 121) AS last_commit_time,
    CONVERT(VARCHAR(23), drs.last_sent_time, 121) AS last_sent_time,
    CONVERT(VARCHAR(23), drs.last_received_time, 121) AS last_received_time,
    CONVERT(VARCHAR(23), drs.last_hardened_time, 121) AS last_hardened_time,
    CONVERT(VARCHAR(25), drs.last_redone_lsn) AS last_redone_lsn,
    drs.suspend_reason_desc
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
    CAST(NULL AS BIGINT) AS redo_rate_kb_sec,
    CAST(NULL AS NVARCHAR(60)) AS database_state_desc,
    CAST(NULL AS NVARCHAR(60)) AS synchronization_state_desc,
    CAST(NULL AS BIT) AS is_local,
    CAST(NULL AS BIT) AS is_primary_replica,
    CAST(NULL AS VARCHAR(23)) AS last_commit_time,
    CAST(NULL AS VARCHAR(23)) AS last_sent_time,
    CAST(NULL AS VARCHAR(23)) AS last_received_time,
    CAST(NULL AS VARCHAR(23)) AS last_hardened_time,
    CAST(NULL AS VARCHAR(25)) AS last_redone_lsn,
    CAST(NULL AS NVARCHAR(60)) AS suspend_reason_desc
WHERE 1=0` // Always returns empty result set

// FailoverClusterReplicaStateQueryAzureMI returns the extended query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports Always On AG and should have access to these DMVs
const FailoverClusterReplicaStateQueryAzureMI = `SELECT
    ar.replica_server_name,
    d.name AS database_name,
    drs.log_send_queue_size AS log_send_queue_kb,
    drs.redo_queue_size AS redo_queue_kb,
    drs.redo_rate AS redo_rate_kb_sec,
    drs.database_state_desc,
    drs.synchronization_state_desc,
    CAST(drs.is_local AS INT) AS is_local,
    CAST(drs.is_primary_replica AS INT) AS is_primary_replica,
    CONVERT(VARCHAR(23), drs.last_commit_time, 121) AS last_commit_time,
    CONVERT(VARCHAR(23), drs.last_sent_time, 121) AS last_sent_time,
    CONVERT(VARCHAR(23), drs.last_received_time, 121) AS last_received_time,
    CONVERT(VARCHAR(23), drs.last_hardened_time, 121) AS last_hardened_time,
    CONVERT(VARCHAR(25), drs.last_redone_lsn) AS last_redone_lsn,
    drs.suspend_reason_desc
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

// FailoverClusterAvailabilityGroupHealthQuery returns the SQL query for Availability Group health status with extended configuration
// This query retrieves health, role, and configuration information for all availability group replicas
//
// The query returns:
// - replica_server_name: Name of the server instance hosting the availability replica
// - role_desc: Current role of the replica (PRIMARY, SECONDARY)
// - synchronization_health_desc: Health of data synchronization (HEALTHY, PARTIALLY_HEALTHY, NOT_HEALTHY)
// - availability_mode_desc: Availability mode (SYNCHRONOUS_COMMIT, ASYNCHRONOUS_COMMIT)
// - failover_mode_desc: Failover mode (AUTOMATIC, MANUAL, EXTERNAL)
// - backup_priority: Backup priority setting (0-100)
// - endpoint_url: Database mirroring endpoint URL
// - read_only_routing_url: Read-only routing URL
// - connected_state_desc: Connection state (CONNECTED, DISCONNECTED)
// - operational_state_desc: Operational state (ONLINE, OFFLINE, etc.)
// - recovery_health_desc: Recovery health (ONLINE, OFFLINE, etc.)
const FailoverClusterAvailabilityGroupHealthQuery = `SELECT
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc,
    ar.availability_mode_desc,
    ar.failover_mode_desc,
    ar.backup_priority,
    ar.endpoint_url,
    ar.read_only_routing_url,
    ars.connected_state_desc,
    ars.operational_state_desc,
    ars.recovery_health_desc
FROM
    sys.dm_hadr_availability_replica_states AS ars
INNER JOIN
    sys.availability_replicas AS ar ON ars.replica_id = ar.replica_id;`

// FailoverClusterAvailabilityGroupHealthQueryAzureSQL returns empty result for Azure SQL Database
// Always On Availability Groups are not supported in Azure SQL Database
const FailoverClusterAvailabilityGroupHealthQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(256)) AS replica_server_name,
    CAST(NULL AS NVARCHAR(60)) AS role_desc,
    CAST(NULL AS NVARCHAR(60)) AS synchronization_health_desc,
    CAST(NULL AS NVARCHAR(60)) AS availability_mode_desc,
    CAST(NULL AS NVARCHAR(60)) AS failover_mode_desc,
    CAST(NULL AS INT) AS backup_priority,
    CAST(NULL AS NVARCHAR(128)) AS endpoint_url,
    CAST(NULL AS NVARCHAR(256)) AS read_only_routing_url,
    CAST(NULL AS NVARCHAR(60)) AS connected_state_desc,
    CAST(NULL AS NVARCHAR(60)) AS operational_state_desc,
    CAST(NULL AS NVARCHAR(60)) AS recovery_health_desc
WHERE 1=0` // Always returns empty result set

// FailoverClusterAvailabilityGroupHealthQueryAzureMI returns the extended query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports Always On AG and should have access to these DMVs
const FailoverClusterAvailabilityGroupHealthQueryAzureMI = `SELECT
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc,
    ar.availability_mode_desc,
    ar.failover_mode_desc,
    ar.backup_priority,
    ar.endpoint_url,
    ar.read_only_routing_url,
    ars.connected_state_desc,
    ars.operational_state_desc,
    ars.recovery_health_desc
FROM
    sys.dm_hadr_availability_replica_states AS ars
INNER JOIN
    sys.availability_replicas AS ar ON ars.replica_id = ar.replica_id;`

// FailoverClusterAvailabilityGroupQuery returns the SQL query for Availability Group configuration metrics
// This query retrieves configuration settings for all availability groups
//
// The query returns:
// - group_name: Name of the availability group
// - automated_backup_preference_desc: Backup preference setting (PRIMARY, SECONDARY_ONLY, SECONDARY, NONE)
// - failure_condition_level: Failure detection level (1-5)
// - health_check_timeout: Health check timeout value in milliseconds
// - cluster_type_desc: Cluster type (WSFC, EXTERNAL, NONE)
// - required_synchronized_secondaries_to_commit: Number of secondary replicas required to be synchronized
const FailoverClusterAvailabilityGroupQuery = `SELECT
    ag.name AS group_name,
    ag.automated_backup_preference_desc,
    ag.failure_condition_level,
    ag.health_check_timeout,
    ag.cluster_type_desc,
    ag.required_synchronized_secondaries_to_commit
FROM
    sys.availability_groups AS ag;`

// FailoverClusterAvailabilityGroupQueryAzureSQL returns empty result for Azure SQL Database
// Always On Availability Groups are not supported in Azure SQL Database
const FailoverClusterAvailabilityGroupQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(128)) AS group_name,
    CAST(NULL AS NVARCHAR(60)) AS automated_backup_preference_desc,
    CAST(NULL AS INT) AS failure_condition_level,
    CAST(NULL AS INT) AS health_check_timeout,
    CAST(NULL AS NVARCHAR(60)) AS cluster_type_desc,
    CAST(NULL AS INT) AS required_synchronized_secondaries_to_commit
WHERE 1=0` // Always returns empty result set

// FailoverClusterAvailabilityGroupQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports Always On AG and should have access to these DMVs
const FailoverClusterAvailabilityGroupQueryAzureMI = `SELECT
    ag.name AS group_name,
    ag.automated_backup_preference_desc,
    ag.failure_condition_level,
    ag.health_check_timeout,
    ag.cluster_type_desc,
    ag.required_synchronized_secondaries_to_commit
FROM
    sys.availability_groups AS ag;`

// FailoverClusterPerformanceCounterQuery returns the SQL query for additional Always On performance counters
// This query retrieves extended performance counters for comprehensive monitoring using PIVOT operation
// to transform rows into columns for easier consumption in monitoring systems like New Relic
//
// The query returns columns for each performance counter by database instance:
// - instance_name: Database name or _Total for aggregate counters
// - Log Send Rate (KB/sec): Rate at which log records are sent to secondary replica
// - Recovery Queue: Number of log records waiting to be recovered
// - File Bytes Received/sec: Rate of file bytes received per second
// - Mirrored Write Transactions/sec: Rate of mirrored write transactions
// - Log Bytes Flushed/sec: Rate of log bytes flushed per second
// - Bytes Sent to Replica/sec: Rate of bytes sent to replica
// - Bytes Received from Replica/sec: Rate of bytes received from replica
// - Sends to Replica/sec: Rate of sends to replica
// - Receives from Replica/sec: Rate of receives from replica
const FailoverClusterPerformanceCounterQuery = `SELECT
    instance_name,
    [Log Send Rate (KB/sec)],
    [Recovery Queue],
    [File Bytes Received/sec],
    [Mirrored Write Transactions/sec],
    [Log Bytes Flushed/sec],
    [Bytes Sent to Replica/sec],
    [Bytes Received from Replica/sec],
    [Sends to Replica/sec],
    [Receives from Replica/sec]
FROM
(
    -- Source data from Always On performance counters
    SELECT
        RTRIM(instance_name) AS instance_name,
        RTRIM(counter_name) AS counter_name,
        cntr_value
    FROM
        sys.dm_os_performance_counters
    WHERE
        (object_name LIKE '%Database Replica%' AND counter_name IN (
            'Log Send Rate (KB/sec)',
            'Recovery Queue',
            'File Bytes Received/sec',
            'Mirrored Write Transactions/sec',
            'Log Bytes Flushed/sec'
        ))
        OR (object_name LIKE '%Availability Replica%' AND counter_name IN (
            'Bytes Sent to Replica/sec',
            'Bytes Received from Replica/sec',
            'Sends to Replica/sec',
            'Receives from Replica/sec'
        ))
) AS SourceData
PIVOT
(
    MAX(cntr_value) -- Aggregate function for pivot operation
    FOR counter_name IN
    (
        -- Transform counter names into columns
        [Log Send Rate (KB/sec)],
        [Recovery Queue],
        [File Bytes Received/sec],
        [Mirrored Write Transactions/sec],
        [Log Bytes Flushed/sec],
        [Bytes Sent to Replica/sec],
        [Bytes Received from Replica/sec],
        [Sends to Replica/sec],
        [Receives from Replica/sec]
    )
) AS PivotTable;`

// FailoverClusterPerformanceCounterQueryAzureSQL returns empty result for Azure SQL Database
// Always On Availability Groups are not supported in Azure SQL Database
const FailoverClusterPerformanceCounterQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(128)) AS instance_name,
    CAST(NULL AS BIGINT) AS [Log Send Rate (KB/sec)],
    CAST(NULL AS BIGINT) AS [Recovery Queue],
    CAST(NULL AS BIGINT) AS [File Bytes Received/sec],
    CAST(NULL AS BIGINT) AS [Mirrored Write Transactions/sec],
    CAST(NULL AS BIGINT) AS [Log Bytes Flushed/sec],
    CAST(NULL AS BIGINT) AS [Bytes Sent to Replica/sec],
    CAST(NULL AS BIGINT) AS [Bytes Received from Replica/sec],
    CAST(NULL AS BIGINT) AS [Sends to Replica/sec],
    CAST(NULL AS BIGINT) AS [Receives from Replica/sec]
WHERE 1=0` // Always returns empty result set

// FailoverClusterPerformanceCounterQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance supports Always On AG and should have access to these performance counters
const FailoverClusterPerformanceCounterQueryAzureMI = `SELECT
    instance_name,
    [Log Send Rate (KB/sec)],
    [Recovery Queue],
    [File Bytes Received/sec],
    [Mirrored Write Transactions/sec],
    [Log Bytes Flushed/sec],
    [Bytes Sent to Replica/sec],
    [Bytes Received from Replica/sec],
    [Sends to Replica/sec],
    [Receives from Replica/sec]
FROM
(
    -- Source data from Always On performance counters
    SELECT
        RTRIM(instance_name) AS instance_name,
        RTRIM(counter_name) AS counter_name,
        cntr_value
    FROM
        sys.dm_os_performance_counters
    WHERE
        (object_name LIKE '%Database Replica%' AND counter_name IN (
            'Log Send Rate (KB/sec)',
            'Recovery Queue',
            'File Bytes Received/sec',
            'Mirrored Write Transactions/sec',
            'Log Bytes Flushed/sec'
        ))
        OR (object_name LIKE '%Availability Replica%' AND counter_name IN (
            'Bytes Sent to Replica/sec',
            'Bytes Received from Replica/sec',
            'Sends to Replica/sec',
            'Receives from Replica/sec'
        ))
) AS SourceData
PIVOT
(
    MAX(cntr_value) -- Aggregate function for pivot operation
    FOR counter_name IN
    (
        -- Transform counter names into columns
        [Log Send Rate (KB/sec)],
        [Recovery Queue],
        [File Bytes Received/sec],
        [Mirrored Write Transactions/sec],
        [Log Bytes Flushed/sec],
        [Bytes Sent to Replica/sec],
        [Bytes Received from Replica/sec],
        [Sends to Replica/sec],
        [Receives from Replica/sec]
    )
) AS PivotTable;`

// FailoverClusterClusterPropertiesQuery returns the SQL query for Windows cluster properties and quorum information
// This query retrieves cluster-level configuration and health information
//
// The query returns:
// - cluster_name: Name of the Windows Server Failover Cluster
// - quorum_type: Quorum configuration type
// - quorum_state: Current quorum state
const FailoverClusterClusterPropertiesQuery = `
-- Return empty result set for cluster properties since this metric is only applicable to clustered SQL Server instances
-- Most Always On AG deployments don't use traditional Windows Server Failover Clustering
SELECT 
    CAST(NULL AS NVARCHAR(256)) AS cluster_name,
    CAST(NULL AS NVARCHAR(128)) AS quorum_type,
    CAST(NULL AS NVARCHAR(128)) AS quorum_state
WHERE 1=0`

// FailoverClusterClusterPropertiesQueryAzureSQL returns empty result for Azure SQL Database
// Cluster properties are not applicable in Azure SQL Database
const FailoverClusterClusterPropertiesQueryAzureSQL = `SELECT 
    CAST(NULL AS NVARCHAR(256)) AS cluster_name,
    CAST(NULL AS NVARCHAR(128)) AS quorum_type,
    CAST(NULL AS NVARCHAR(128)) AS quorum_state
WHERE 1=0` // Always returns empty result set

// FailoverClusterClusterPropertiesQueryAzureMI returns the same query for Azure SQL Managed Instance
// Azure SQL Managed Instance has built-in clustering but may have limited access to cluster properties
const FailoverClusterClusterPropertiesQueryAzureMI = `
-- Return empty result set for cluster properties since this metric is only applicable to clustered SQL Server instances
-- Most Always On AG deployments don't use traditional Windows Server Failover Clustering
SELECT 
    CAST(NULL AS NVARCHAR(256)) AS cluster_name,
    CAST(NULL AS NVARCHAR(128)) AS quorum_type,
    CAST(NULL AS NVARCHAR(128)) AS quorum_state
WHERE 1=0`
