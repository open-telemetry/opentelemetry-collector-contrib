// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"

import (
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// metricRecordFunc records the data point to the metric builder at the supplied timestamp
type metricRecordFunc func(*MetricsBuilder, *mongodbatlas.DataPoints, pcommon.Timestamp)

// getRecordFunc returns the metricRecordFunc that matches the metric name. Nil if none is found.
func getRecordFunc(metricName string) metricRecordFunc {
	switch metricName {
	// MongoDB CPU usage. For hosts with more than one CPU core, these values can exceed 100%.

	case "PROCESS_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "MAX_PROCESS_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "PROCESS_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "MAX_PROCESS_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	// MongoDB CPU usage scaled to a range of 0% to 100%. Atlas computes this value by dividing by the number of CPU cores.

	case "PROCESS_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "PROCESS_NORMALIZED_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	// Context: Process
	case "MAX_PROCESS_NORMALIZED_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}

	case "PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	// Rate of asserts for a MongoDB process found in the asserts document that the serverStatus command generates.

	case "ASSERT_REGULAR":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeRegular)
		}

	case "ASSERT_WARNING":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeWarning)
		}

	case "ASSERT_MSG":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeMsg)
		}

	case "ASSERT_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertTypeUser)
		}

	// Amount of data flushed in the background.

	case "BACKGROUND_FLUSH_AVG":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessBackgroundFlushDataPoint(ts, float64(*dp.Value))
		}

	// Amount of bytes in the WiredTiger storage engine cache and tickets found in the wiredTiger.cache and wiredTiger.concurrentTransactions documents that the serverStatus command generates.

	case "CACHE_BYTES_READ_INTO":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirectionReadInto)
		}

	case "CACHE_BYTES_WRITTEN_FROM":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirectionWrittenFrom)
		}

	case "CACHE_DIRTY_BYTES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatusDirty)
		}

	case "CACHE_USED_BYTES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatusUsed)
		}

	case "CACHE_FILL_RATIO":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheRatioDataPoint(ts, float64(*dp.Value), AttributeCacheRatioTypeCacheFill)
		}

	case "DIRTY_FILL_RATIO":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheRatioDataPoint(ts, float64(*dp.Value), AttributeCacheRatioTypeDirtyFill)
		}

	case "TICKETS_AVAILABLE_READS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketTypeAvailableReads)
		}

	case "TICKETS_AVAILABLE_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketTypeAvailableWrites)
		}

	// Number of connections to a MongoDB process found in the connections document that the serverStatus command generates.
	case "CONNECTIONS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessConnectionsDataPoint(ts, float64(*dp.Value))
		}

	// Number of cursors for a MongoDB process found in the metrics.cursor document that the serverStatus command generates.
	case "CURSORS_TOTAL_OPEN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorStateOpen)
		}

	case "CURSORS_TOTAL_TIMED_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorStateTimedOut)
		}

	// Numbers of Memory Issues and Page Faults for a MongoDB process.
	case "EXTRA_INFO_PAGE_FAULTS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueTypeExtraInfo)
		}

	case "GLOBAL_ACCESSES_NOT_IN_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueTypeGlobalAccessesNotInMemory)
		}
	case "GLOBAL_PAGE_FAULT_EXCEPTIONS_THROWN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueTypeExceptionsThrown)
		}

	// Number of operations waiting on locks for the MongoDB process that the serverStatus command generates. Cloud Manager computes these values based on the type of storage engine.
	case "GLOBAL_LOCK_CURRENT_QUEUE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockStateCurrentQueueTotal)
		}
	case "GLOBAL_LOCK_CURRENT_QUEUE_READERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockStateCurrentQueueReaders)
		}
	case "GLOBAL_LOCK_CURRENT_QUEUE_WRITERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockStateCurrentQueueWriters)
		}

	// Number of index btree operations.
	case "INDEX_COUNTERS_BTREE_ACCESSES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterTypeAccesses)
		}
	case "INDEX_COUNTERS_BTREE_HITS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterTypeHits)
		}
	case "INDEX_COUNTERS_BTREE_MISSES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterTypeMisses)
		}
	case "INDEX_COUNTERS_BTREE_MISS_RATIO":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexBtreeMissRatioDataPoint(ts, float64(*dp.Value))
		}

	// Number of journaling operations.
	case "JOURNALING_COMMITS_IN_WRITE_LOCK":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessJournalingCommitsDataPoint(ts, float64(*dp.Value))
		}
	case "JOURNALING_MB":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessJournalingWrittenDataPoint(ts, float64(*dp.Value))
		}
	case "JOURNALING_WRITE_DATA_FILES_MB":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessJournalingDataFilesDataPoint(ts, float64(*dp.Value))
		}

	// Amount of memory for a MongoDB process found in the mem document that the serverStatus command collects.
	case "MEMORY_RESIDENT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateResident)
		}
	case "MEMORY_VIRTUAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateVirtual)
		}

	case "MEMORY_MAPPED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateMapped)
		}
	case "COMPUTED_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateComputed)
		}

	// Amount of throughput for MongoDB process found in the network document that the serverStatus command collects.

	case "NETWORK_BYTES_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "NETWORK_BYTES_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}
	case "NETWORK_NUM_REQUESTS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessNetworkRequestsDataPoint(ts, float64(*dp.Value))
		}

	// Durations and throughput of the MongoDB process' oplog.
	case "OPLOG_SLAVE_LAG_MASTER_TIME":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogTypeSlaveLagMasterTime)
		}
	case "OPLOG_MASTER_TIME":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogTypeMasterTime)
		}
	case "OPLOG_MASTER_LAG_TIME_DIFF":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogTypeMasterLagTimeDiff)
		}
	case "OPLOG_RATE_GB_PER_HOUR":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogRateDataPoint(ts, float64(*dp.Value))
		}

	// Number of database operations on a MongoDB process since the process last started.

	case "DB_STORAGE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusTotal)
		}

	case "DB_DATA_SIZE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusDataSize)
		}
	case "DB_INDEX_SIZE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusIndexSize)
		}
	case "DB_DATA_SIZE_TOTAL_WO_SYSTEM":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatusDataSizeWoSystem)
		}

	// Rate of database operations on a MongoDB process since the process last started found in the opcounters document that the serverStatus command collects.
	case "OPCOUNTER_CMD":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationCmd, AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_QUERY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationQuery, AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_UPDATE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationUpdate, AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_DELETE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationDelete, AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_GETMORE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationGetmore, AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_INSERT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationInsert, AttributeClusterRolePrimary)
		}
	case "OPCOUNTER_TTL_DELETED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationTTLDeleted, AttributeClusterRolePrimary)
		}

	// Rate of database operations on MongoDB secondaries found in the opcountersRepl document that the serverStatus command collects.
	case "OPCOUNTER_REPL_CMD":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationCmd, AttributeClusterRoleReplica)
		}
	case "OPCOUNTER_REPL_UPDATE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationUpdate, AttributeClusterRoleReplica)
		}
	case "OPCOUNTER_REPL_DELETE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationDelete, AttributeClusterRoleReplica)
		}
	case "OPCOUNTER_REPL_INSERT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationInsert, AttributeClusterRoleReplica)
		}

	// Average rate of documents returned, inserted, updated, or deleted per second during a selected time period.
	case "DOCUMENT_METRICS_RETURNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatusReturned)
		}
	case "DOCUMENT_METRICS_INSERTED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatusInserted)
		}
	case "DOCUMENT_METRICS_UPDATED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatusUpdated)
		}
	case "DOCUMENT_METRICS_DELETED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatusDeleted)
		}

	// Average rate for operations per second during a selected time period that perform a sort but cannot perform the sort using an index.
	case "OPERATIONS_SCAN_AND_ORDER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperationScanAndOrder, AttributeClusterRolePrimary)
		}

	// Average execution time in milliseconds per read, write, or command operation during a selected time period.
	case "OP_EXECUTION_TIME_READS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionTypeReads)
		}
	case "OP_EXECUTION_TIME_WRITES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionTypeWrites)
		}
	case "OP_EXECUTION_TIME_COMMANDS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionTypeCommands)
		}

	// Number of times the host restarted within the previous hour.
	case "RESTARTS_IN_LAST_HOUR":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessRestartsDataPoint(ts, float64(*dp.Value))
		}

	// Average rate per second to scan index items during queries and query-plan evaluations found in the value of totalKeysExamined from the explain command.
	case "QUERY_EXECUTOR_SCANNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value), AttributeScannedTypeIndexItems)
		}

	// Average rate of documents scanned per second during queries and query-plan evaluations found in the value of totalDocsExamined from the explain command.
	case "QUERY_EXECUTOR_SCANNED_OBJECTS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value), AttributeScannedTypeObjects)
		}

	// Ratio of the number of index items scanned to the number of documents returned.
	case "QUERY_TARGETING_SCANNED_PER_RETURNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value), AttributeScannedTypeIndexItems)
		}

	// Ratio of the number of documents scanned to the number of documents returned.
	case "QUERY_TARGETING_SCANNED_OBJECTS_PER_RETURNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value), AttributeScannedTypeObjects)
		}

	// CPU usage of processes on the host. For hosts with more than one CPU core, this value can exceed 100%.
	case "SYSTEM_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "MAX_SYSTEM_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "SYSTEM_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "MAX_SYSTEM_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "SYSTEM_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "MAX_SYSTEM_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "SYSTEM_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "MAX_SYSTEM_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "SYSTEM_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "MAX_SYSTEM_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "SYSTEM_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "MAX_SYSTEM_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "SYSTEM_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "MAX_SYSTEM_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "SYSTEM_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}
	case "MAX_SYSTEM_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}

	// CPU usage of processes on the host scaled to a range of 0 to 100% by dividing by the number of CPU cores.
	case "SYSTEM_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "SYSTEM_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "SYSTEM_NORMALIZED_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateNice)
		}
	case "SYSTEM_NORMALIZED_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIowait)
		}
	case "SYSTEM_NORMALIZED_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateIrq)
		}
	case "SYSTEM_NORMALIZED_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSoftirq)
		}
	case "SYSTEM_NORMALIZED_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateGuest)
		}
	case "SYSTEM_NORMALIZED_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUStateSteal)
		}
	// Physical memory usage, in bytes, that the host uses.
	case "SYSTEM_MEMORY_AVAILABLE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusAvailable)
		}
	case "MAX_SYSTEM_MEMORY_AVAILABLE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusAvailable)
		}
	case "SYSTEM_MEMORY_BUFFERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusBuffers)
		}
	case "MAX_SYSTEM_MEMORY_BUFFERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusBuffers)
		}
	case "SYSTEM_MEMORY_CACHED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusCached)
		}
	case "MAX_SYSTEM_MEMORY_CACHED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusCached)
		}
	case "SYSTEM_MEMORY_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusFree)
		}
	case "MAX_SYSTEM_MEMORY_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusFree)
		}
	case "SYSTEM_MEMORY_SHARED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusShared)
		}
	case "MAX_SYSTEM_MEMORY_SHARED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusShared)
		}
	case "SYSTEM_MEMORY_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusUsed)
		}
	case "MAX_SYSTEM_MEMORY_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatusUsed)
		}

	// Average rate of physical bytes per second that the eth0 network interface received and transmitted.
	case "SYSTEM_NETWORK_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "MAX_SYSTEM_NETWORK_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "SYSTEM_NETWORK_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}
	case "MAX_SYSTEM_NETWORK_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}

	// Total amount of memory that swap uses.
	case "SWAP_USAGE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateUsed)
		}
	case "MAX_SWAP_USAGE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStateUsed)
		}
	case "SWAP_USAGE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateFree)
		}
	case "MAX_SWAP_USAGE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStateFree)
		}

	// Total amount of memory written and read from swap.
	case "SWAP_IO_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "MAX_SWAP_IO_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionReceive)
		}
	case "SWAP_IO_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}
	case "MAX_SWAP_IO_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirectionTransmit)
		}

	// Memory usage, in bytes, that Atlas Search processes use.
	case "FTS_PROCESS_RESIDENT_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateResident)
		}
	case "FTS_PROCESS_VIRTUAL_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateVirtual)
		}
	case "FTS_PROCESS_SHARED_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateShared)
		}
	case "FTS_MEMORY_MAPPED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryStateMapped)
		}

	// Disk space, in bytes, that Atlas Search indexes use.
	// FTS_DISK_UTILIZATION is the documented field name, but FTS_DISK_USAGE is what is returned from the API.
	// Including both so if the API changes to match the documentation this metric is still collected.
	case "FTS_DISK_USAGE", "FTS_DISK_UTILIZATION":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsDiskUsedDataPoint(ts, float64(*dp.Value))
		}
	// Percentage of CPU that Atlas Search processes use.
	case "FTS_PROCESS_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "FTS_PROCESS_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}
	case "FTS_PROCESS_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateUser)
		}
	case "FTS_PROCESS_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUStateKernel)
		}

	// Process Disk Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)

	// Measures throughput of I/O operations for the disk partition used for MongoDB.
	case "DISK_PARTITION_IOPS_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "MAX_DISK_PARTITION_IOPS_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "DISK_PARTITION_IOPS_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	case "MAX_DISK_PARTITION_IOPS_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	case "DISK_PARTITION_IOPS_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionTotal)
		}

	case "MAX_DISK_PARTITION_IOPS_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionTotal)
		}

	// Measures throughput of data read and written to the disk partition (not cache) used by MongoDB.
	case "DISK_PARTITION_THROUGHPUT_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionThroughputDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "DISK_PARTITION_THROUGHPUT_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionThroughputDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	// This is a calculated metric that is the sum of the read and write throughput.
	case "DISK_PARTITION_THROUGHPUT_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionThroughputDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionTotal)
		}

	// Measures the queue depth of the disk partition used by MongoDB.
	case "DISK_QUEUE_DEPTH":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionQueueDepthDataPoint(ts, float64(*dp.Value))
		}

	// Measures latency per operation type of the disk partition used by MongoDB.
	case "DISK_PARTITION_LATENCY_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "MAX_DISK_PARTITION_LATENCY_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionRead)
		}

	case "DISK_PARTITION_LATENCY_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	case "MAX_DISK_PARTITION_LATENCY_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirectionWrite)
		}

	// The percentage of time during which requests are being issued to and serviced by the partition.
	// This includes requests from any process, not just MongoDB processes.
	case "DISK_PARTITION_UTILIZATION":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUtilizationAverageDataPoint(ts, float64(*dp.Value))
		}

	case "MAX_DISK_PARTITION_UTILIZATION":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUtilizationMaxDataPoint(ts, float64(*dp.Value))
		}

	// Measures the free disk space and used disk space on the disk partition used by MongoDB.
	case "DISK_PARTITION_SPACE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}

	case "MAX_DISK_PARTITION_SPACE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}

	case "DISK_PARTITION_SPACE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}

	case "MAX_DISK_PARTITION_SPACE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}

	case "DISK_PARTITION_SPACE_PERCENT_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUsageAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}
	case "MAX_DISK_PARTITION_SPACE_PERCENT_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUsageMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusFree)
		}
	case "DISK_PARTITION_SPACE_PERCENT_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUsageAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}
	case "MAX_DISK_PARTITION_SPACE_PERCENT_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUsageMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatusUsed)
		}

	// Process Database Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)
	case "DATABASE_COLLECTION_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeCollection)
		}
	case "DATABASE_INDEX_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeIndex)
		}
	case "DATABASE_EXTENT_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeExtent)
		}
	case "DATABASE_OBJECT_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeObject)
		}
	case "DATABASE_VIEW_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectTypeView)
		}
	case "DATABASE_AVERAGE_OBJECT_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeObject)
		}
	case "DATABASE_STORAGE_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeStorage)
		}
	case "DATABASE_INDEX_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeIndex)
		}
	case "DATABASE_DATA_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectTypeData)
		}

	default:
		return nil
	}
}

func MeasurementsToMetric(mb *MetricsBuilder, meas *mongodbatlas.Measurements) error {
	recordFunc := getRecordFunc(meas.Name)
	if recordFunc == nil {
		return nil
	}

	return addDataPoint(mb, meas, recordFunc)
}

func addDataPoint(mb *MetricsBuilder, meas *mongodbatlas.Measurements, recordFunc metricRecordFunc) error {
	for _, point := range meas.DataPoints {
		if point.Value != nil {
			curTime, err := time.Parse(time.RFC3339, point.Timestamp)
			if err != nil {
				return err
			}
			recordFunc(mb, point, pcommon.NewTimestampFromTime(curTime))
		}
	}
	return nil
}
