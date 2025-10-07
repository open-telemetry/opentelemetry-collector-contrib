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
	ReplicaServerName string `db:"replica_server_name"`

	// DatabaseName represents the name of the database in the availability group
	// Query source: sys.databases joined with sys.dm_hadr_database_replica_states
	DatabaseName string `db:"database_name"`

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
}
