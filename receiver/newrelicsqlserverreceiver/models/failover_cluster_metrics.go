// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package models provides data structures for failover cluster-level metrics and information.
// This file defines the data models used to represent SQL Server Always On Availability Groups
// and failover cluster performance metrics for high availability environments.
//
// Failover Cluster-Level Data Structures:
//
// 1. Failover Cluster Replica Metrics:
//
//	type FailoverClusterReplicaMetrics struct {
//	    LogBytesReceivedPerSec    *int64   // Log bytes received per second from primary replica
//	    TransactionDelayMs        *int64   // Transaction delay in milliseconds
//	    FlowControlTimeMs         *int64   // Flow control time in milliseconds per second
//	}
//
// Data Source Mappings:
// - sys.dm_os_performance_counters: Database replica performance counters for Always On AG
//
// Metric Calculations:
//
// Log Bytes Received/sec:
// - Query: sys.dm_os_performance_counters WHERE object_name LIKE '%Database Replica%' AND counter_name = 'Log Bytes Received/sec'
// - Purpose: Measures the rate of log records received by the secondary replica from the primary replica
// - Unit: Bytes per second
// - Scope: Per database replica in Always On Availability Group
//
// Transaction Delay:
// - Query: sys.dm_os_performance_counters WHERE object_name LIKE '%Database Replica%' AND counter_name = 'Transaction Delay'
// - Purpose: Average delay for transactions on the secondary replica
// - Unit: Milliseconds
// - Scope: Per database replica in Always On Availability Group
//
// Flow Control Time (ms/sec):
// - Query: sys.dm_os_performance_counters WHERE object_name LIKE '%Database Replica%' AND counter_name = 'Flow Control Time (ms/sec)'
// - Purpose: Time spent in flow control by log records from the primary replica
// - Unit: Milliseconds per second
// - Scope: Per database replica in Always On Availability Group
//
// Usage in Scrapers:
// - Only available on SQL Server instances configured with Always On Availability Groups
// - Provides failover cluster performance monitoring for high availability scenarios
// - Helps monitor replication lag and flow control in Always On environments
// - Enables performance optimization for Always On Availability Groups
package models

// FailoverClusterReplicaMetrics represents Always On Availability Group replica performance metrics
// This model captures the replica-level performance data as defined for Always On failover clusters
type FailoverClusterReplicaMetrics struct {
	// LogBytesReceivedPerSec represents the rate of log records received by secondary replica from primary
	// This metric corresponds to 'Log Bytes Received/sec' performance counter
	// Query source: sys.dm_os_performance_counters for Database Replica counters
	LogBytesReceivedPerSec *int64 `db:"Log Bytes Received/sec" metric_name:"sqlserver.failover_cluster.log_bytes_received_per_sec" source_type:"gauge"`

	// TransactionDelayMs represents the average delay for transactions on the secondary replica
	// This metric corresponds to 'Transaction Delay' performance counter
	// Query source: sys.dm_os_performance_counters for Database Replica counters
	TransactionDelayMs *int64 `db:"Transaction Delay" metric_name:"sqlserver.failover_cluster.transaction_delay_ms" source_type:"gauge"`

	// FlowControlTimeMs represents the time spent in flow control by log records from primary replica
	// This metric corresponds to 'Flow Control Time (ms/sec)' performance counter
	// Query source: sys.dm_os_performance_counters for Database Replica counters
	FlowControlTimeMs *int64 `db:"Flow Control Time (ms/sec)" metric_name:"sqlserver.failover_cluster.flow_control_time_ms" source_type:"gauge"`
}

// FailoverClusterReplicaStateMetrics represents Always On Availability Group database replica state metrics
// This model captures the database-level replica state information including log synchronization details
type FailoverClusterReplicaStateMetrics struct {
	// ReplicaServerName represents the name of the server hosting the replica
	// Query source: sys.availability_replicas joined with sys.dm_hadr_database_replica_states
	ReplicaServerName string `db:"replica_server_name" source_type:"attribute"`

	// DatabaseName represents the name of the database in the availability group
	// Query source: sys.databases joined with sys.dm_hadr_database_replica_states
	DatabaseName string `db:"database_name" source_type:"attribute"`

	// LogSendQueueKB represents the amount of log records that haven't been sent to secondary replica
	// This metric corresponds to 'log_send_queue_size' from sys.dm_hadr_database_replica_states
	// Query source: sys.dm_hadr_database_replica_states.log_send_queue_size
	LogSendQueueKB *int64 `db:"log_send_queue_kb" metric_name:"sqlserver.failover_cluster.log_send_queue_kb" source_type:"gauge"`

	// RedoQueueKB represents the amount of log records waiting to be redone on secondary replica
	// This metric corresponds to 'redo_queue_size' from sys.dm_hadr_database_replica_states
	// Query source: sys.dm_hadr_database_replica_states.redo_queue_size
	RedoQueueKB *int64 `db:"redo_queue_kb" metric_name:"sqlserver.failover_cluster.redo_queue_kb" source_type:"gauge"`

	// RedoRateKBSec represents the rate at which log records are being redone on secondary replica
	// This metric corresponds to 'redo_rate' from sys.dm_hadr_database_replica_states
	// Query source: sys.dm_hadr_database_replica_states.redo_rate
	RedoRateKBSec *int64 `db:"redo_rate_kb_sec" metric_name:"sqlserver.failover_cluster.redo_rate_kb_sec" source_type:"gauge"`

	// DatabaseStateDesc represents the current state of the database replica
	// Query source: sys.dm_hadr_database_replica_states.database_state_desc
	// Expected values: "ONLINE", "RESTORING", "RECOVERING", "RECOVERY_PENDING", "SUSPECT", etc.
	// Can be NULL if not applicable
	DatabaseStateDesc *string `db:"database_state_desc" metric_name:"sqlserver.failover_cluster.database_state" source_type:"info"`

	// SynchronizationStateDesc represents the synchronization state of the database replica
	// Query source: sys.dm_hadr_database_replica_states.synchronization_state_desc
	// Expected values: "SYNCHRONIZED", "SYNCHRONIZING", "NOT_SYNCHRONIZING", "INITIALIZING", "REVERTING"
	// Can be NULL if not applicable
	SynchronizationStateDesc *string `db:"synchronization_state_desc" metric_name:"sqlserver.failover_cluster.synchronization_state" source_type:"info"`

	// IsLocal indicates if this is a local replica (1) or remote replica (0)
	// Query source: sys.dm_hadr_database_replica_states.is_local
	IsLocal *int64 `db:"is_local" metric_name:"sqlserver.failover_cluster.is_local_replica" source_type:"gauge"`

	// IsPrimaryReplica indicates if this is the primary replica (1) or secondary (0)
	// Query source: sys.dm_hadr_database_replica_states.is_primary_replica
	IsPrimaryReplica *int64 `db:"is_primary_replica" metric_name:"sqlserver.failover_cluster.is_primary_replica" source_type:"gauge"`

	// LastCommitTime represents the timestamp of the last transaction commit
	// Query source: sys.dm_hadr_database_replica_states.last_commit_time
	LastCommitTime *string `db:"last_commit_time" source_type:"attribute"`

	// LastSentTime represents the timestamp when the last log block was sent
	// Query source: sys.dm_hadr_database_replica_states.last_sent_time
	LastSentTime *string `db:"last_sent_time" source_type:"attribute"`

	// LastReceivedTime represents the timestamp when the last log block was received
	// Query source: sys.dm_hadr_database_replica_states.last_received_time
	LastReceivedTime *string `db:"last_received_time" source_type:"attribute"`

	// LastHardenedTime represents the timestamp when the last log block was hardened
	// Query source: sys.dm_hadr_database_replica_states.last_hardened_time
	LastHardenedTime *string `db:"last_hardened_time" source_type:"attribute"`

	// LastRedoneLSN represents the log sequence number of the last redone log record
	// Query source: sys.dm_hadr_database_replica_states.last_redone_lsn
	LastRedoneLSN *string `db:"last_redone_lsn" source_type:"attribute"`

	// SuspendReasonDesc represents the reason why the database is suspended (if applicable)
	// Query source: sys.dm_hadr_database_replica_states.suspend_reason_desc
	// Expected values: NULL, "SUSPEND", "REDO", "APPLY", "CAPTURE", "RESTART", "UNDO", "REVALIDATE", "OTHER"
	SuspendReasonDesc *string `db:"suspend_reason_desc" metric_name:"sqlserver.failover_cluster.suspend_reason" source_type:"info"`
}

// FailoverClusterNodeMetrics represents cluster node information and status
// This model captures the server node details in a Windows Server Failover Cluster
type FailoverClusterNodeMetrics struct {
	// NodeName represents the name of the server node in the cluster
	// Query source: sys.dm_os_cluster_nodes.nodename
	NodeName string `db:"nodename" source_type:"attribute"`

	// StatusDescription represents the health state of the cluster node
	// Query source: sys.dm_os_cluster_nodes.status_description
	// Expected values: "Up", "Down", "Paused", etc.
	StatusDescription string `db:"status_description" metric_name:"sqlserver.failover_cluster.node_status" source_type:"info"`

	// IsCurrentOwner indicates if this is the active node currently running the SQL Server instance
	// Query source: sys.dm_os_cluster_nodes.is_current_owner
	// Value: 1 = active node, 0 = passive node
	IsCurrentOwner *int64 `db:"is_current_owner" metric_name:"sqlserver.failover_cluster.node_is_current_owner" source_type:"gauge"`
}

// FailoverClusterAvailabilityGroupHealthMetrics represents Always On Availability Group health status
// This model captures the health and role information for availability group replicas
type FailoverClusterAvailabilityGroupHealthMetrics struct {
	// ReplicaServerName represents the name of the server instance hosting the availability replica
	// Query source: sys.availability_replicas.replica_server_name
	ReplicaServerName string `db:"replica_server_name" source_type:"attribute"`

	// RoleDesc describes the current role of the replica within the Availability Group
	// Query source: sys.dm_hadr_availability_replica_states.role_desc
	// Expected values: "PRIMARY", "SECONDARY"
	RoleDesc string `db:"role_desc" metric_name:"sqlserver.failover_cluster.ag_replica_role" source_type:"info"`

	// SynchronizationHealthDesc indicates the health of data synchronization between primary and secondary
	// Query source: sys.dm_hadr_availability_replica_states.synchronization_health_desc
	// Expected values: "HEALTHY", "PARTIALLY_HEALTHY", "NOT_HEALTHY"
	SynchronizationHealthDesc string `db:"synchronization_health_desc" metric_name:"sqlserver.failover_cluster.ag_synchronization_health" source_type:"info"`

	// AvailabilityModeDesc represents the availability mode of the replica
	// Query source: sys.availability_replicas.availability_mode_desc
	// Expected values: "SYNCHRONOUS_COMMIT", "ASYNCHRONOUS_COMMIT"
	AvailabilityModeDesc string `db:"availability_mode_desc" metric_name:"sqlserver.failover_cluster.availability_mode" source_type:"info"`

	// FailoverModeDesc represents the failover mode of the replica
	// Query source: sys.availability_replicas.failover_mode_desc
	// Expected values: "AUTOMATIC", "MANUAL", "EXTERNAL"
	FailoverModeDesc string `db:"failover_mode_desc" metric_name:"sqlserver.failover_cluster.failover_mode" source_type:"info"`

	// BackupPriority represents the backup priority setting for this replica
	// Query source: sys.availability_replicas.backup_priority
	// Range: 0-100, higher values indicate higher priority for backups
	BackupPriority *int64 `db:"backup_priority" metric_name:"sqlserver.failover_cluster.backup_priority" source_type:"gauge"`

	// EndpointURL represents the database mirroring endpoint URL for this replica
	// Query source: sys.availability_replicas.endpoint_url
	EndpointURL *string `db:"endpoint_url" source_type:"attribute"`

	// ReadOnlyRoutingURL represents the read-only routing URL for this replica
	// Query source: sys.availability_replicas.read_only_routing_url
	ReadOnlyRoutingURL *string `db:"read_only_routing_url" source_type:"attribute"`

	// ConnectedStateDesc indicates the connection state of the replica
	// Query source: sys.dm_hadr_availability_replica_states.connected_state_desc
	// Expected values: "CONNECTED", "DISCONNECTED"
	ConnectedStateDesc string `db:"connected_state_desc" metric_name:"sqlserver.failover_cluster.connected_state" source_type:"info"`

	// OperationalStateDesc indicates the operational state of the replica
	// Query source: sys.dm_hadr_availability_replica_states.operational_state_desc
	// Expected values: "PENDING_FAILOVER", "PENDING", "ONLINE", "OFFLINE", "FAILED", "FAILED_NO_QUORUM"
	// Can be NULL if not applicable
	OperationalStateDesc *string `db:"operational_state_desc" metric_name:"sqlserver.failover_cluster.operational_state" source_type:"info"`

	// RecoveryHealthDesc indicates the recovery health of the replica
	// Query source: sys.dm_hadr_availability_replica_states.recovery_health_desc
	// Expected values: "ONLINE_IN_PROGRESS", "ONLINE", "OFFLINE"
	// Can be NULL if not applicable
	RecoveryHealthDesc *string `db:"recovery_health_desc" metric_name:"sqlserver.failover_cluster.recovery_health" source_type:"info"`
}

// FailoverClusterAvailabilityGroupMetrics represents Always On Availability Group configuration metrics
// This model captures the availability group level configuration and settings
type FailoverClusterAvailabilityGroupMetrics struct {
	// GroupName represents the name of the availability group
	// Query source: sys.availability_groups.name
	GroupName string `db:"group_name" source_type:"attribute"`

	// AutomatedBackupPreferenceDesc represents the backup preference setting for the AG
	// Query source: sys.availability_groups.automated_backup_preference_desc
	// Expected values: "PRIMARY", "SECONDARY_ONLY", "SECONDARY", "NONE"
	AutomatedBackupPreferenceDesc string `db:"automated_backup_preference_desc" metric_name:"sqlserver.failover_cluster.automated_backup_preference" source_type:"info"`

	// FailureConditionLevel represents the failure detection level for the AG
	// Query source: sys.availability_groups.failure_condition_level
	// Range: 1-5, higher values indicate more sensitive failure detection
	FailureConditionLevel *int64 `db:"failure_condition_level" metric_name:"sqlserver.failover_cluster.failure_condition_level" source_type:"gauge"`

	// HealthCheckTimeout represents the health check timeout value in milliseconds
	// Query source: sys.availability_groups.health_check_timeout
	HealthCheckTimeout *int64 `db:"health_check_timeout" metric_name:"sqlserver.failover_cluster.health_check_timeout_ms" source_type:"gauge"`

	// ClusterTypeDesc represents the cluster type for the availability group
	// Query source: sys.availability_groups.cluster_type_desc
	// Expected values: "WSFC", "EXTERNAL", "NONE"
	ClusterTypeDesc string `db:"cluster_type_desc" metric_name:"sqlserver.failover_cluster.cluster_type" source_type:"info"`

	// RequiredSynchronizedSecondariesToCommit represents the number of secondary replicas required to be synchronized
	// Query source: sys.availability_groups.required_synchronized_secondaries_to_commit
	RequiredSynchronizedSecondariesToCommit *int64 `db:"required_synchronized_secondaries_to_commit" metric_name:"sqlserver.failover_cluster.required_sync_secondaries" source_type:"gauge"`
}

// FailoverClusterPerformanceCounterMetrics represents additional Always On performance counter metrics
// This model captures extended performance counters for comprehensive monitoring using a PIVOT structure
// Each performance counter is represented as a separate column for easier consumption in monitoring systems
type FailoverClusterPerformanceCounterMetrics struct {
	// InstanceName represents the instance name for the performance counter (database name or _Total)
	// Query source: sys.dm_os_performance_counters.instance_name
	InstanceName string `db:"instance_name" source_type:"attribute"`

	// LogSendRateKBSec represents the rate at which log records are sent to secondary replica
	// Query source: sys.dm_os_performance_counters where counter_name = 'Log Send Rate (KB/sec)'
	LogSendRateKBSec *int64 `db:"Log Send Rate (KB/sec)" metric_name:"sqlserver.failover_cluster.log_send_rate_kb_sec" source_type:"gauge"`

	// RecoveryQueue represents the number of log records waiting to be recovered
	// Query source: sys.dm_os_performance_counters where counter_name = 'Recovery Queue'
	RecoveryQueue *int64 `db:"Recovery Queue" metric_name:"sqlserver.failover_cluster.recovery_queue" source_type:"gauge"`

	// FileBytesReceivedSec represents the rate of file bytes received per second
	// Query source: sys.dm_os_performance_counters where counter_name = 'File Bytes Received/sec'
	FileBytesReceivedSec *int64 `db:"File Bytes Received/sec" metric_name:"sqlserver.failover_cluster.file_bytes_received_sec" source_type:"gauge"`

	// MirroredWriteTransactionsSec represents the rate of mirrored write transactions
	// Query source: sys.dm_os_performance_counters where counter_name = 'Mirrored Write Transactions/sec'
	MirroredWriteTransactionsSec *int64 `db:"Mirrored Write Transactions/sec" metric_name:"sqlserver.failover_cluster.mirrored_write_transactions_sec" source_type:"gauge"`

	// LogBytesFlushedSec represents the rate of log bytes flushed per second
	// Query source: sys.dm_os_performance_counters where counter_name = 'Log Bytes Flushed/sec'
	LogBytesFlushedSec *int64 `db:"Log Bytes Flushed/sec" metric_name:"sqlserver.failover_cluster.log_bytes_flushed_sec" source_type:"gauge"`

	// BytesSentToReplicaSec represents the rate of bytes sent to replica
	// Query source: sys.dm_os_performance_counters where counter_name = 'Bytes Sent to Replica/sec'
	BytesSentToReplicaSec *int64 `db:"Bytes Sent to Replica/sec" metric_name:"sqlserver.failover_cluster.bytes_sent_to_replica_sec" source_type:"gauge"`

	// BytesReceivedFromReplicaSec represents the rate of bytes received from replica
	// Query source: sys.dm_os_performance_counters where counter_name = 'Bytes Received from Replica/sec'
	BytesReceivedFromReplicaSec *int64 `db:"Bytes Received from Replica/sec" metric_name:"sqlserver.failover_cluster.bytes_received_from_replica_sec" source_type:"gauge"`

	// SendsToReplicaSec represents the rate of sends to replica
	// Query source: sys.dm_os_performance_counters where counter_name = 'Sends to Replica/sec'
	SendsToReplicaSec *int64 `db:"Sends to Replica/sec" metric_name:"sqlserver.failover_cluster.sends_to_replica_sec" source_type:"gauge"`

	// ReceivesFromReplicaSec represents the rate of receives from replica
	// Query source: sys.dm_os_performance_counters where counter_name = 'Receives from Replica/sec'
	ReceivesFromReplicaSec *int64 `db:"Receives from Replica/sec" metric_name:"sqlserver.failover_cluster.receives_from_replica_sec" source_type:"gauge"`
}

// FailoverClusterClusterPropertiesMetrics represents Windows cluster properties and quorum information
// This model captures cluster-level configuration and health information
type FailoverClusterClusterPropertiesMetrics struct {
	// ClusterName represents the name of the Windows Server Failover Cluster
	// Query source: sys.dm_os_cluster_properties where property_name = 'cluster_name'
	ClusterName *string `db:"cluster_name" source_type:"attribute"`

	// QuorumType represents the quorum configuration type
	// Query source: sys.dm_os_cluster_properties where property_name = 'quorum_type'
	QuorumType *string `db:"quorum_type" metric_name:"sqlserver.failover_cluster.quorum_type" source_type:"info"`

	// QuorumState represents the current quorum state
	// Query source: sys.dm_os_cluster_properties where property_name = 'quorum_state'
	QuorumState *string `db:"quorum_state" metric_name:"sqlserver.failover_cluster.quorum_state" source_type:"info"`
}
