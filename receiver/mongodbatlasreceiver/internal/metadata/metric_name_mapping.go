// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"

import (
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// metricRecordFunc records the data point to the metric builder at the supplied timestamp
type metricRecordFunc func(*ResourceMetricsBuilder, *mongodbatlas.DataPoints, pcommon.Timestamp)

// getRecordFunc returns the metricRecordFunc that matches the metric name. Nil if none is found.
func getRecordFunc(metricName string) metricRecordFunc {
	switch metricName {
	// MongoDB CPU usage. For hosts with more than one CPU core, these values can exceed 100%.

	case "PROCESS_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "MAX_PROCESS_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "PROCESS_CPU_CHILDREN_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "MAX_PROCESS_CPU_CHILDREN_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_CPU_CHILDREN_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_CPU_CHILDREN_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	// MongoDB CPU usage scaled to a range of 0% to 100%. Atlas computes this value by dividing by the number of CPU cores.

	case "PROCESS_NORMALIZED_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateUser)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_NORMALIZED_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "PROCESS_NORMALIZED_CPU_CHILDREN_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateUser)
		}

	// Context: Process
	case "MAX_PROCESS_NORMALIZED_CPU_CHILDREN_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateUser)
		}

	case "PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateKernel)
		}

	// Rate of asserts for a MongoDB process found in the asserts document that the serverStatus command generates.

	case "ASSERT_REGULAR":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeRegular)
		}

	case "ASSERT_WARNING":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeWarning)
		}

	case "ASSERT_MSG":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeMsg)
		}

	case "ASSERT_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeUser)
		}

	// Amount of data flushed in the background.

	case "BACKGROUND_FLUSH_AVG":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessBackgroundFlushDataPoint(ts, float64(*dp.Value))
		}

	// Amount of bytes in the WiredTiger storage engine cache and tickets found in the wiredTiger.cache and wiredTiger.concurrentTransactions documents that the serverStatus command generates.

	case "CACHE_BYTES_READ_INTO":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirectionReadInto)
		}

	case "CACHE_BYTES_WRITTEN_FROM":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirectionWrittenFrom)
		}

	case "CACHE_DIRTY_BYTES":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatusDirty)
		}

	case "CACHE_USED_BYTES":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatusUsed)
		}

	case "TICKETS_AVAILABLE_READS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketTypeAvailableReads)
		}

	case "TICKETS_AVAILABLE_WRITE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketTypeAvailableWrites)
		}

	// Number of connections to a MongoDB process found in the connections document that the serverStatus command generates.
	case "CONNECTIONS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessConnectionsDataPoint(ts, float64(*dp.Value))
		}

	// Number of cursors for a MongoDB process found in the metrics.cursor document that the serverStatus command generates.
	case "CURSORS_TOTAL_OPEN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorStateOpen)
		}

	case "CURSORS_TOTAL_TIMED_OUT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorStateTimedOut)
		}

	// Numbers of Memory Issues and Page Faults for a MongoDB process.
	case "EXTRA_INFO_PAGE_FAULTS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueTypeExtraInfo)
		}

	case "GLOBAL_ACCESSES_NOT_IN_MEMORY":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value),
				AttributeMemoryIssueTypeGlobalAccessesNotInMemory)
		}
	case "GLOBAL_PAGE_FAULT_EXCEPTIONS_THROWN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value),
				AttributeMemoryIssueTypeExceptionsThrown)
		}

	// Number of operations waiting on locks for the MongoDB process that the serverStatus command generates. Cloud Manager computes these values based on the type of storage engine.
	case "GLOBAL_LOCK_CURRENT_QUEUE_TOTAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value),
				AttributeGlobalLockStateCurrentQueueTotal)
		}
	case "GLOBAL_LOCK_CURRENT_QUEUE_READERS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value),
				AttributeGlobalLockStateCurrentQueueReaders)
		}
	case "GLOBAL_LOCK_CURRENT_QUEUE_WRITERS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value),
				AttributeGlobalLockStateCurrentQueueWriters)
		}

	// Number of index btree operations.
	case "INDEX_COUNTERS_BTREE_ACCESSES":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value),
				AttributeBtreeCounterTypeAccesses)
		}
	case "INDEX_COUNTERS_BTREE_HITS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterTypeHits)
		}
	case "INDEX_COUNTERS_BTREE_MISSES":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterTypeMisses)
		}
	case "INDEX_COUNTERS_BTREE_MISS_RATIO":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessIndexBtreeMissRatioDataPoint(ts, float64(*dp.Value))
		}

	// Number of journaling operations.
	case "JOURNALING_COMMITS_IN_WRITE_LOCK":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessJournalingCommitsDataPoint(ts, float64(*dp.Value))
		}
	case "JOURNALING_MB":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessJournalingWrittenDataPoint(ts, float64(*dp.Value))
		}
	case "JOURNALING_WRITE_DATA_FILES_MB":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessJournalingDataFilesDataPoint(ts, float64(*dp.Value))
		}

	// Amount of memory for a MongoDB process found in the mem document that the serverStatus command collects.
	case "MEMORY_RESIDENT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateResident)
		}
	case "MEMORY_VIRTUAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateVirtual)
		}

	case "MEMORY_MAPPED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateMapped)
		}
	case "COMPUTED_MEMORY":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateComputed)
		}

	// Amount of throughput for MongoDB process found in the network document that the serverStatus command collects.

	case "NETWORK_BYTES_IN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "NETWORK_BYTES_OUT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}
	case "NETWORK_NUM_REQUESTS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessNetworkRequestsDataPoint(ts, float64(*dp.Value))
		}

	// Durations and throughput of the MongoDB process' oplog.
	case "OPLOG_SLAVE_LAG_MASTER_TIME":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value),
				AttributeOplogTypeSlaveLagMasterTime)
		}
	case "OPLOG_MASTER_TIME":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogTypeMasterTime)
		}
	case "OPLOG_MASTER_LAG_TIME_DIFF":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogTypeMasterLagTimeDiff)
		}
	case "OPLOG_RATE_GB_PER_HOUR":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessOplogRateDataPoint(ts, float64(*dp.Value))
		}

	// Number of database operations on a MongoDB process since the process last started.

	case "DB_STORAGE_TOTAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusTotal)
		}

	case "DB_DATA_SIZE_TOTAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusDataSize)
		}
	case "DB_INDEX_SIZE_TOTAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusIndexSize)
		}
	case "DB_DATA_SIZE_TOTAL_WO_SYSTEM":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value),
				AttributeStorageStatusDataSizeWoSystem)
		}

	// Rate of database operations on a MongoDB process since the process last started found in the opcounters document that the serverStatus command collects.
	case "OPCOUNTER_CMD":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationCmd,
				AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_QUERY":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationQuery,
				AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_UPDATE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationUpdate,
				AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_DELETE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationDelete,
				AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_GETMORE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationGetmore,
				AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_INSERT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationInsert,
				AttributeClusterRolePrimary)
		}

	// Rate of database operations on MongoDB secondaries found in the opcountersRepl document that the serverStatus command collects.
	case "OPCOUNTER_REPL_CMD":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationCmd,
				AttributeClusterRoleReplica)
		}
	case "OPCOUNTER_REPL_UPDATE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationUpdate,
				AttributeClusterRoleReplica)
		}
	case "OPCOUNTER_REPL_DELETE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationDelete,
				AttributeClusterRoleReplica)
		}
	case "OPCOUNTER_REPL_INSERT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationInsert,
				AttributeClusterRoleReplica)
		}

	// Average rate of documents returned, inserted, updated, or deleted per second during a selected time period.
	case "DOCUMENT_METRICS_RETURNED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value),
				AttributeDocumentStatusReturned)
		}
	case "DOCUMENT_METRICS_INSERTED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value),
				AttributeDocumentStatusInserted)
		}
	case "DOCUMENT_METRICS_UPDATED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatusUpdated)
		}
	case "DOCUMENT_METRICS_DELETED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatusDeleted)
		}

	// Average rate for operations per second during a selected time period that perform a sort but cannot perform the sort using an index.
	case "OPERATIONS_SCAN_AND_ORDER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value),
				AttributeOperationScanAndOrder, AttributeClusterRolePrimary)
		}

	// Average execution time in milliseconds per read, write, or command operation during a selected time period.
	case "OP_EXECUTION_TIME_READS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionTypeReads)
		}
	case "OP_EXECUTION_TIME_WRITES":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionTypeWrites)
		}
	case "OP_EXECUTION_TIME_COMMANDS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value),
				AttributeExecutionTypeCommands)
		}

	// Number of times the host restarted within the previous hour.
	case "RESTARTS_IN_LAST_HOUR":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessRestartsDataPoint(ts, float64(*dp.Value))
		}

	// Average rate per second to scan index items during queries and query-plan evaluations found in the value of totalKeysExamined from the explain command.
	case "QUERY_EXECUTOR_SCANNED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value),
				AttributeScannedTypeIndexItems)
		}

	// Average rate of documents scanned per second during queries and query-plan evaluations found in the value of totalDocsExamined from the explain command.
	case "QUERY_EXECUTOR_SCANNED_OBJECTS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value),
				AttributeScannedTypeObjects)
		}

	// Ratio of the number of index items scanned to the number of documents returned.
	case "QUERY_TARGETING_SCANNED_PER_RETURNED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value),
				AttributeScannedTypeIndexItems)
		}

	// Ratio of the number of documents scanned to the number of documents returned.
	case "QUERY_TARGETING_SCANNED_OBJECTS_PER_RETURNED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value),
				AttributeScannedTypeObjects)
		}

	// CPU usage of processes on the host. For hosts with more than one CPU core, this value can exceed 100%.
	case "SYSTEM_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "MAX_SYSTEM_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "SYSTEM_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "MAX_SYSTEM_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "SYSTEM_CPU_NICE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "MAX_SYSTEM_CPU_NICE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "SYSTEM_CPU_IOWAIT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "MAX_SYSTEM_CPU_IOWAIT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "SYSTEM_CPU_IRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "MAX_SYSTEM_CPU_IRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "SYSTEM_CPU_SOFTIRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "MAX_SYSTEM_CPU_SOFTIRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "SYSTEM_CPU_GUEST":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "MAX_SYSTEM_CPU_GUEST":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "SYSTEM_CPU_STEAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}
	case "MAX_SYSTEM_CPU_STEAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}

	// CPU usage of processes on the host scaled to a range of 0 to 100% by dividing by the number of CPU cores.
	case "SYSTEM_NORMALIZED_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateUser)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_NICE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "SYSTEM_NORMALIZED_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateKernel)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "SYSTEM_NORMALIZED_CPU_NICE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateNice)
		}
	case "SYSTEM_NORMALIZED_CPU_IOWAIT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateIowait)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_IOWAIT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "SYSTEM_NORMALIZED_CPU_IRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_IRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "SYSTEM_NORMALIZED_CPU_SOFTIRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateSoftirq)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_SOFTIRQ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "SYSTEM_NORMALIZED_CPU_GUEST":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateGuest)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_GUEST":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "SYSTEM_NORMALIZED_CPU_STEAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeCPUStateSteal)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_STEAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}
	// Physical memory usage, in bytes, that the host uses.
	case "SYSTEM_MEMORY_AVAILABLE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeMemoryStatusAvailable)
		}
	case "MAX_SYSTEM_MEMORY_AVAILABLE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusAvailable)
		}
	case "SYSTEM_MEMORY_BUFFERS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value),
				AttributeMemoryStatusBuffers)
		}
	case "MAX_SYSTEM_MEMORY_BUFFERS":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusBuffers)
		}
	case "SYSTEM_MEMORY_CACHED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusCached)
		}
	case "MAX_SYSTEM_MEMORY_CACHED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusCached)
		}
	case "SYSTEM_MEMORY_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusFree)
		}
	case "MAX_SYSTEM_MEMORY_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusFree)
		}
	case "SYSTEM_MEMORY_SHARED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusShared)
		}
	case "MAX_SYSTEM_MEMORY_SHARED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusShared)
		}
	case "SYSTEM_MEMORY_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusUsed)
		}
	case "MAX_SYSTEM_MEMORY_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusUsed)
		}

	// Average rate of physical bytes per second that the eth0 network interface received and transmitted.
	case "SYSTEM_NETWORK_IN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "MAX_SYSTEM_NETWORK_IN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "SYSTEM_NETWORK_OUT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}
	case "MAX_SYSTEM_NETWORK_OUT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}

	// Total amount of memory that swap uses.
	case "SWAP_USAGE_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateUsed)
		}
	case "MAX_SWAP_USAGE_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStateUsed)
		}
	case "SWAP_USAGE_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateFree)
		}
	case "MAX_SWAP_USAGE_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStateFree)
		}

	// Total amount of memory written and read from swap.
	case "SWAP_IO_IN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "MAX_SWAP_IO_IN":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "SWAP_IO_OUT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}
	case "MAX_SWAP_IO_OUT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}

	// Memory usage, in bytes, that Atlas Search processes use.
	case "FTS_PROCESS_RESIDENT_MEMORY":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateResident)
		}
	case "FTS_PROCESS_VIRTUAL_MEMORY":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateVirtual)
		}
	case "FTS_PROCESS_SHARED_MEMORY":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateShared)
		}
	case "FTS_MEMORY_MAPPED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateMapped)
		}

	// Disk space, in bytes, that Atlas Search indexes use.
	// FTS_DISK_UTILIZATION is the documented field name, but FTS_DISK_USAGE is what is returned from the API.
	// Including both so if the API changes to match the documentation this metric is still collected.
	case "FTS_DISK_USAGE", "FTS_DISK_UTILIZATION":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsDiskUsedDataPoint(ts, float64(*dp.Value))
		}
	// Percentage of CPU that Atlas Search processes use.
	case "FTS_PROCESS_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "FTS_PROCESS_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "FTS_PROCESS_NORMALIZED_CPU_USER":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "FTS_PROCESS_NORMALIZED_CPU_KERNEL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	// Process Disk Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)

	// Measures throughput of I/O operations for the disk partition used for MongoDB.
	case "DISK_PARTITION_IOPS_READ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "MAX_DISK_PARTITION_IOPS_READ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "DISK_PARTITION_IOPS_WRITE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	case "MAX_DISK_PARTITION_IOPS_WRITE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	case "DISK_PARTITION_IOPS_TOTAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionTotal)
		}

	case "MAX_DISK_PARTITION_IOPS_TOTAL":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionTotal)
		}

	// Measures latency per operation type of the disk partition used by MongoDB.
	case "DISK_PARTITION_LATENCY_READ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value),
				AttributeDiskDirectionRead)
		}

	case "MAX_DISK_PARTITION_LATENCY_READ":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "DISK_PARTITION_LATENCY_WRITE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value),
				AttributeDiskDirectionWrite)
		}

	case "MAX_DISK_PARTITION_LATENCY_WRITE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	// The percentage of time during which requests are being issued to and serviced by the partition.
	// This includes requests from any process, not just MongoDB processes.
	case "DISK_PARTITION_UTILIZATION":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionUtilizationAverageDataPoint(ts, float64(*dp.Value))
		}

	case "MAX_DISK_PARTITION_UTILIZATION":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionUtilizationMaxDataPoint(ts, float64(*dp.Value))
		}

	// Measures the free disk space and used disk space on the disk partition used by MongoDB.
	case "DISK_PARTITION_SPACE_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}

	case "MAX_DISK_PARTITION_SPACE_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}

	case "DISK_PARTITION_SPACE_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}

	case "MAX_DISK_PARTITION_SPACE_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}

	case "DISK_PARTITION_SPACE_PERCENT_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionUsageAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}
	case "MAX_DISK_PARTITION_SPACE_PERCENT_FREE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionUsageMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}
	case "DISK_PARTITION_SPACE_PERCENT_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionUsageAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}
	case "MAX_DISK_PARTITION_SPACE_PERCENT_USED":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDiskPartitionUsageMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}

	// Process Database Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)
	case "DATABASE_COLLECTION_COUNT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeCollection)
		}
	case "DATABASE_INDEX_COUNT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeIndex)
		}
	case "DATABASE_EXTENT_COUNT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeExtent)
		}
	case "DATABASE_OBJECT_COUNT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeObject)
		}
	case "DATABASE_VIEW_COUNT":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeView)
		}
	case "DATABASE_AVERAGE_OBJECT_SIZE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeObject)
		}
	case "DATABASE_STORAGE_SIZE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeStorage)
		}
	case "DATABASE_INDEX_SIZE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeIndex)
		}
	case "DATABASE_DATA_SIZE":
		return func(rmb *ResourceMetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			rmb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeData)
		}

	default:
		return nil
	}
}

func MeasurementsToMetric(rmb *ResourceMetricsBuilder, meas *mongodbatlas.Measurements, _ bool) error {
	recordFunc := getRecordFunc(meas.Name)
	if recordFunc == nil {
		return nil
	}

	return addDataPoint(rmb, meas, recordFunc)
}

func addDataPoint(rmb *ResourceMetricsBuilder, meas *mongodbatlas.Measurements, recordFunc metricRecordFunc) error {
	for _, point := range meas.DataPoints {
		if point.Value != nil {
			curTime, err := time.Parse(time.RFC3339, point.Timestamp)
			if err != nil {
				return err
			}
			recordFunc(rmb, point, pcommon.NewTimestampFromTime(curTime))
		}
	}
	return nil
}
