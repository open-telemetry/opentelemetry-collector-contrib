// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			mb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "MAX_PROCESS_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "PROCESS_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	case "MAX_PROCESS_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	case "PROCESS_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "MAX_PROCESS_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "PROCESS_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	case "MAX_PROCESS_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	// MongoDB CPU usage scaled to a range of 0% to 100%. Atlas computes this value by dividing by the number of CPU cores.

	case "PROCESS_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "PROCESS_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	case "PROCESS_NORMALIZED_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	// Context: Process
	case "MAX_PROCESS_NORMALIZED_CPU_CHILDREN_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}

	case "PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	case "MAX_PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	// Rate of asserts for a MongoDB process found in the asserts document that the serverStatus command generates.

	case "ASSERT_REGULAR":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.Regular)
		}

	case "ASSERT_WARNING":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.Warning)
		}

	case "ASSERT_MSG":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.Msg)
		}

	case "ASSERT_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.User)
		}

	// Amount of data flushed in the background.

	case "BACKGROUND_FLUSH_AVG":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessBackgroundFlushDataPoint(ts, float64(*dp.Value))
		}

	// Amount of bytes in the WiredTiger storage engine cache and tickets found in the wiredTiger.cache and wiredTiger.concurrentTransactions documents that the serverStatus command generates.

	case "CACHE_BYTES_READ_INTO":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirection.ReadInto)
		}

	case "CACHE_BYTES_WRITTEN_FROM":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirection.WrittenFrom)
		}

	case "CACHE_DIRTY_BYTES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatus.Dirty)
		}

	case "CACHE_USED_BYTES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatus.Used)
		}

	case "TICKETS_AVAILABLE_READS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketType.AvailableReads)
		}

	case "TICKETS_AVAILABLE_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketType.AvailableWrites)
		}

	// Number of connections to a MongoDB process found in the connections document that the serverStatus command generates.
	case "CONNECTIONS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessConnectionsDataPoint(ts, float64(*dp.Value))
		}

	// Number of cursors for a MongoDB process found in the metrics.cursor document that the serverStatus command generates.
	case "CURSORS_TOTAL_OPEN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorState.Open)
		}

	case "CURSORS_TOTAL_TIMED_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorState.TimedOut)
		}

	// Numbers of Memory Issues and Page Faults for a MongoDB process.
	case "EXTRA_INFO_PAGE_FAULTS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueType.ExtraInfo)
		}

	case "GLOBAL_ACCESSES_NOT_IN_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueType.GlobalAccessesNotInMemory)
		}
	case "GLOBAL_PAGE_FAULT_EXCEPTIONS_THROWN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueType.ExceptionsThrown)
		}

	// Number of operations waiting on locks for the MongoDB process that the serverStatus command generates. Cloud Manager computes these values based on the type of storage engine.
	case "GLOBAL_LOCK_CURRENT_QUEUE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockState.CurrentQueueTotal)
		}
	case "GLOBAL_LOCK_CURRENT_QUEUE_READERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockState.CurrentQueueReaders)
		}
	case "GLOBAL_LOCK_CURRENT_QUEUE_WRITERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockState.CurrentQueueWriters)
		}

	// Number of index btree operations.
	case "INDEX_COUNTERS_BTREE_ACCESSES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterType.Accesses)
		}
	case "INDEX_COUNTERS_BTREE_HITS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterType.Hits)
		}
	case "INDEX_COUNTERS_BTREE_MISSES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterType.Misses)
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
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Resident)
		}
	case "MEMORY_VIRTUAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Virtual)
		}

	case "MEMORY_MAPPED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Mapped)
		}
	case "COMPUTED_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Computed)
		}

	// Amount of throughput for MongoDB process found in the network document that the serverStatus command collects.

	case "NETWORK_BYTES_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
		}
	case "NETWORK_BYTES_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
		}
	case "NETWORK_NUM_REQUESTS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessNetworkRequestsDataPoint(ts, float64(*dp.Value))
		}

	// Durations and throughput of the MongoDB process' oplog.
	case "OPLOG_SLAVE_LAG_MASTER_TIME":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogType.SlaveLagMasterTime)
		}
	case "OPLOG_MASTER_TIME":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogType.MasterTime)
		}
	case "OPLOG_MASTER_LAG_TIME_DIFF":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogType.MasterLagTimeDiff)
		}
	case "OPLOG_RATE_GB_PER_HOUR":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessOplogRateDataPoint(ts, float64(*dp.Value))
		}

	// Number of database operations on a MongoDB process since the process last started.

	case "DB_STORAGE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.Total)
		}

	case "DB_DATA_SIZE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.DataSize)
		}
	case "DB_INDEX_SIZE_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.IndexSize)
		}
	case "DB_DATA_SIZE_TOTAL_WO_SYSTEM":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.DataSizeWoSystem)
		}

	// Rate of database operations on a MongoDB process since the process last started found in the opcounters document that the serverStatus command collects.
	case "OPCOUNTER_CMD":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Cmd, AttributeClusterRole.Primary)
		}
	case "OPCOUNTER_QUERY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Query, AttributeClusterRole.Primary)
		}
	case "OPCOUNTER_UPDATE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Update, AttributeClusterRole.Primary)
		}
	case "OPCOUNTER_DELETE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Delete, AttributeClusterRole.Primary)
		}
	case "OPCOUNTER_GETMORE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Getmore, AttributeClusterRole.Primary)
		}
	case "OPCOUNTER_INSERT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Insert, AttributeClusterRole.Primary)
		}

	// Rate of database operations on MongoDB secondaries found in the opcountersRepl document that the serverStatus command collects.
	case "OPCOUNTER_REPL_CMD":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Cmd, AttributeClusterRole.Replica)
		}
	case "OPCOUNTER_REPL_UPDATE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Update, AttributeClusterRole.Replica)
		}
	case "OPCOUNTER_REPL_DELETE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Delete, AttributeClusterRole.Replica)
		}
	case "OPCOUNTER_REPL_INSERT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Insert, AttributeClusterRole.Replica)
		}

	// Average rate of documents returned, inserted, updated, or deleted per second during a selected time period.
	case "DOCUMENT_METRICS_RETURNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Returned)
		}
	case "DOCUMENT_METRICS_INSERTED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Inserted)
		}
	case "DOCUMENT_METRICS_UPDATED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Updated)
		}
	case "DOCUMENT_METRICS_DELETED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Deleted)
		}

	// Average rate for operations per second during a selected time period that perform a sort but cannot perform the sort using an index.
	case "OPERATIONS_SCAN_AND_ORDER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.ScanAndOrder, AttributeClusterRole.Primary)
		}

	// Average execution time in milliseconds per read, write, or command operation during a selected time period.
	case "OP_EXECUTION_TIME_READS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionType.Reads)
		}
	case "OP_EXECUTION_TIME_WRITES":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionType.Writes)
		}
	case "OP_EXECUTION_TIME_COMMANDS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionType.Commands)
		}

	// Number of times the host restarted within the previous hour.
	case "RESTARTS_IN_LAST_HOUR":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessRestartsDataPoint(ts, float64(*dp.Value))
		}

	// Average rate per second to scan index items during queries and query-plan evaluations found in the value of totalKeysExamined from the explain command.
	case "QUERY_EXECUTOR_SCANNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value), AttributeScannedType.IndexItems)
		}

	// Average rate of documents scanned per second during queries and query-plan evaluations found in the value of totalDocsExamined from the explain command.
	case "QUERY_EXECUTOR_SCANNED_OBJECTS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value), AttributeScannedType.Objects)
		}

	// Ratio of the number of index items scanned to the number of documents returned.
	case "QUERY_TARGETING_SCANNED_PER_RETURNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value), AttributeScannedType.IndexItems)
		}

	// Ratio of the number of documents scanned to the number of documents returned.
	case "QUERY_TARGETING_SCANNED_OBJECTS_PER_RETURNED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value), AttributeScannedType.Objects)
		}

	// CPU usage of processes on the host. For hosts with more than one CPU core, this value can exceed 100%.
	case "SYSTEM_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}
	case "MAX_SYSTEM_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}
	case "SYSTEM_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}
	case "MAX_SYSTEM_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}
	case "SYSTEM_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
		}
	case "MAX_SYSTEM_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
		}
	case "SYSTEM_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
		}
	case "MAX_SYSTEM_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
		}
	case "SYSTEM_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
		}
	case "MAX_SYSTEM_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
		}
	case "SYSTEM_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
		}
	case "MAX_SYSTEM_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
		}
	case "SYSTEM_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
		}
	case "MAX_SYSTEM_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
		}
	case "SYSTEM_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
		}
	case "MAX_SYSTEM_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
		}

	// CPU usage of processes on the host scaled to a range of 0 to 100% by dividing by the number of CPU cores.
	case "SYSTEM_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
		}
	case "SYSTEM_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}
	case "SYSTEM_NORMALIZED_CPU_NICE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
		}
	case "SYSTEM_NORMALIZED_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_IOWAIT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
		}
	case "SYSTEM_NORMALIZED_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_IRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
		}
	case "SYSTEM_NORMALIZED_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_SOFTIRQ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
		}
	case "SYSTEM_NORMALIZED_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_GUEST":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
		}
	case "SYSTEM_NORMALIZED_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
		}
	case "MAX_SYSTEM_NORMALIZED_CPU_STEAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
		}
	// Physical memory usage, in bytes, that the host uses.
	case "SYSTEM_MEMORY_AVAILABLE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Available)
		}
	case "MAX_SYSTEM_MEMORY_AVAILABLE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Available)
		}
	case "SYSTEM_MEMORY_BUFFERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Buffers)
		}
	case "MAX_SYSTEM_MEMORY_BUFFERS":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Buffers)
		}
	case "SYSTEM_MEMORY_CACHED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Cached)
		}
	case "MAX_SYSTEM_MEMORY_CACHED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Cached)
		}
	case "SYSTEM_MEMORY_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Free)
		}
	case "MAX_SYSTEM_MEMORY_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Free)
		}
	case "SYSTEM_MEMORY_SHARED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Shared)
		}
	case "MAX_SYSTEM_MEMORY_SHARED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Shared)
		}
	case "SYSTEM_MEMORY_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Used)
		}
	case "MAX_SYSTEM_MEMORY_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Used)
		}

	// Average rate of physical bytes per second that the eth0 network interface received and transmitted.
	case "SYSTEM_NETWORK_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
		}
	case "MAX_SYSTEM_NETWORK_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
		}
	case "SYSTEM_NETWORK_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
		}
	case "MAX_SYSTEM_NETWORK_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
		}

	// Total amount of memory that swap uses.
	case "SWAP_USAGE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Used)
		}
	case "MAX_SWAP_USAGE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Used)
		}
	case "SWAP_USAGE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Free)
		}
	case "MAX_SWAP_USAGE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Free)
		}

	// Total amount of memory written and read from swap.
	case "SWAP_IO_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
		}
	case "MAX_SWAP_IO_IN":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
		}
	case "SWAP_IO_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
		}
	case "MAX_SWAP_IO_OUT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
		}

	// Memory usage, in bytes, that Atlas Search processes use.
	case "FTS_PROCESS_RESIDENT_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Resident)
		}
	case "FTS_PROCESS_VIRTUAL_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Virtual)
		}
	case "FTS_PROCESS_SHARED_MEMORY":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Shared)
		}
	case "FTS_MEMORY_MAPPED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Mapped)
		}

	// Disk space, in bytes, that Atlas Search indexes use.
	case "FTS_DISK_USAGE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsDiskUsedDataPoint(ts, float64(*dp.Value))
		}

	// Percentage of CPU that Atlas Search processes use.
	case "FTS_PROCESS_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}
	case "FTS_PROCESS_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}
	case "FTS_PROCESS_NORMALIZED_CPU_USER":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
		}
	case "FTS_PROCESS_NORMALIZED_CPU_KERNEL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
		}

	// Process Disk Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)

	// Measures throughput of I/O operations for the disk partition used for MongoDB.
	case "DISK_PARTITION_IOPS_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
		}

	case "MAX_DISK_PARTITION_IOPS_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
		}

	case "DISK_PARTITION_IOPS_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
		}

	case "MAX_DISK_PARTITION_IOPS_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
		}

	case "DISK_PARTITION_IOPS_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Total)
		}

	case "MAX_DISK_PARTITION_IOPS_TOTAL":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Total)
		}

	// The percentage of time during which requests are being issued to and serviced by the partition.
	// This includes requests from any process, not just MongoDB processes.
	case "DISK_PARTITION_LATENCY_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
		}

	case "MAX_DISK_PARTITION_LATENCY_READ":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
		}

	case "DISK_PARTITION_LATENCY_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
		}

	case "MAX_DISK_PARTITION_LATENCY_WRITE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
		}

	// Measures latency per operation type of the disk partition used by MongoDB.
	case "DISK_PARTITION_SPACE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
		}

	case "MAX_DISK_PARTITION_SPACE_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
		}

	case "DISK_PARTITION_SPACE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
		}

	case "MAX_DISK_PARTITION_SPACE_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
		}

	case "DISK_PARTITION_SPACE_PERCENT_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUtilizationAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
		}
	case "MAX_DISK_PARTITION_SPACE_PERCENT_FREE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUtilizationMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
		}
	case "DISK_PARTITION_SPACE_PERCENT_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUtilizationAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
		}
	case "MAX_DISK_PARTITION_SPACE_PERCENT_USED":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDiskPartitionUtilizationMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
		}

	// Process Database Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)
	case "DATABASE_COLLECTION_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Collection)
		}
	case "DATABASE_INDEX_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Index)
		}
	case "DATABASE_EXTENT_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Extent)
		}
	case "DATABASE_OBJECT_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Object)
		}
	case "DATABASE_VIEW_COUNT":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.View)
		}
	case "DATABASE_AVERAGE_OBJECT_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Object)
		}
	case "DATABASE_STORAGE_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Storage)
		}
	case "DATABASE_INDEX_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Index)
		}
	case "DATABASE_DATA_SIZE":
		return func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pcommon.Timestamp) {
			mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Data)
		}

	default:
		return nil
	}
}

func MeasurementsToMetric(mb *MetricsBuilder, meas *mongodbatlas.Measurements, buildUnrecognized bool) error {
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
