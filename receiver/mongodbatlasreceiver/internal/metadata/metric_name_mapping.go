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
	"go.opentelemetry.io/collector/model/pdata"
)

type metricRecordFunc func(*MetricsBuilder, *mongodbatlas.DataPoints, pdata.Timestamp)

type metricMappingData struct {
	metricName string
	attributes map[string]pdata.Value
}

var metricNameMapping = map[string]metricRecordFunc{
	// MongoDB CPU usage. For hosts with more than one CPU core, these values can exceed 100%.

	"PROCESS_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"MAX_PROCESS_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"PROCESS_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	"MAX_PROCESS_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	"PROCESS_CPU_CHILDREN_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"MAX_PROCESS_CPU_CHILDREN_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"PROCESS_CPU_CHILDREN_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	"MAX_PROCESS_CPU_CHILDREN_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	// MongoDB CPU usage scaled to a range of 0% to 100%. Atlas computes this value by dividing by the number of CPU cores.

	"PROCESS_NORMALIZED_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"MAX_PROCESS_NORMALIZED_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"PROCESS_NORMALIZED_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	"MAX_PROCESS_NORMALIZED_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	"PROCESS_NORMALIZED_CPU_CHILDREN_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	// Context: Process
	"MAX_PROCESS_NORMALIZED_CPU_CHILDREN_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},

	"PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	"MAX_PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCPUChildrenNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	// Rate of asserts for a MongoDB process found in the asserts document that the serverStatus command generates.

	"ASSERT_REGULAR": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.Regular)
	},

	"ASSERT_WARNING": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.Warning)
	},

	"ASSERT_MSG": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.Msg)
	},

	"ASSERT_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessAssertsDataPoint(ts, float64(*dp.Value), AttributeAssertType.User)
	},

	// Amount of data flushed in the background.

	"BACKGROUND_FLUSH_AVG": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessBackgroundFlushDataPoint(ts, float64(*dp.Value))
	},

	// Amount of bytes in the WiredTiger storage engine cache and tickets found in the wiredTiger.cache and wiredTiger.concurrentTransactions documents that the serverStatus command generates.

	"CACHE_BYTES_READ_INTO": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirection.ReadInto)
	},

	"CACHE_BYTES_WRITTEN_FROM": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCacheIoDataPoint(ts, float64(*dp.Value), AttributeCacheDirection.WrittenFrom)
	},

	"CACHE_DIRTY_BYTES": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatus.Dirty)
	},

	"CACHE_USED_BYTES": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCacheSizeDataPoint(ts, float64(*dp.Value), AttributeCacheStatus.Used)
	},

	"TICKETS_AVAILABLE_READS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketType.AvailableReads)
	},

	"TICKETS_AVAILABLE_WRITE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessTicketsDataPoint(ts, float64(*dp.Value), AttributeTicketType.AvailableWrites)
	},

	// Number of connections to a MongoDB process found in the connections document that the serverStatus command generates.
	"CONNECTIONS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessConnectionsDataPoint(ts, float64(*dp.Value))
	},

	// Number of cursors for a MongoDB process found in the metrics.cursor document that the serverStatus command generates.
	"CURSORS_TOTAL_OPEN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorState.Open)
	},

	"CURSORS_TOTAL_TIMED_OUT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessCursorsDataPoint(ts, float64(*dp.Value), AttributeCursorState.TimedOut)
	},

	// Numbers of Memory Issues and Page Faults for a MongoDB process.
	"EXTRA_INFO_PAGE_FAULTS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueType.ExtraInfo)
	},

	"GLOBAL_ACCESSES_NOT_IN_MEMORY": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueType.GlobalAccessesNotInMemory)
	},
	"GLOBAL_PAGE_FAULT_EXCEPTIONS_THROWN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessPageFaultsDataPoint(ts, float64(*dp.Value), AttributeMemoryIssueType.ExceptionsThrown)
	},

	// Number of operations waiting on locks for the MongoDB process that the serverStatus command generates. Cloud Manager computes these values based on the type of storage engine.
	"GLOBAL_LOCK_CURRENT_QUEUE_TOTAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockState.CurrentQueueTotal)
	},
	"GLOBAL_LOCK_CURRENT_QUEUE_READERS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockState.CurrentQueueReaders)
	},
	"GLOBAL_LOCK_CURRENT_QUEUE_WRITERS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessGlobalLockDataPoint(ts, float64(*dp.Value), AttributeGlobalLockState.CurrentQueueWriters)
	},

	// Number of index btree operations.
	"INDEX_COUNTERS_BTREE_ACCESSES": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterType.Accesses)
	},
	"INDEX_COUNTERS_BTREE_HITS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterType.Hits)
	},
	"INDEX_COUNTERS_BTREE_MISSES": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessIndexCountersDataPoint(ts, float64(*dp.Value), AttributeBtreeCounterType.Misses)
	},
	"INDEX_COUNTERS_BTREE_MISS_RATIO": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessIndexBtreeMissRatioDataPoint(ts, float64(*dp.Value))
	},

	// Number of journaling operations.
	"JOURNALING_COMMITS_IN_WRITE_LOCK": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessJournalingCommitsDataPoint(ts, float64(*dp.Value))
	},
	"JOURNALING_MB": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessJournalingWrittenDataPoint(ts, float64(*dp.Value))
	},
	"JOURNALING_WRITE_DATA_FILES_MB": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessJournalingDataFilesDataPoint(ts, float64(*dp.Value))
	},

	// Amount of memory for a MongoDB process found in the mem document that the serverStatus command collects.
	"MEMORY_RESIDENT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Resident)
	},
	"MEMORY_VIRTUAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Virtual)
	},

	"MEMORY_MAPPED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Mapped)
	},
	"COMPUTED_MEMORY": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Computed)
	},

	// Amount of throughput for MongoDB process found in the network document that the serverStatus command collects.

	"NETWORK_BYTES_IN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
	},
	"NETWORK_BYTES_OUT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessNetworkIoDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
	},
	"NETWORK_NUM_REQUESTS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessNetworkRequestsDataPoint(ts, float64(*dp.Value))
	},

	// Durations and throughput of the MongoDB process' oplog.
	"OPLOG_SLAVE_LAG_MASTER_TIME": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogType.SlaveLagMasterTime)
	},
	"OPLOG_MASTER_TIME": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogType.MasterTime)
	},
	"OPLOG_MASTER_LAG_TIME_DIFF": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessOplogTimeDataPoint(ts, float64(*dp.Value), AttributeOplogType.MasterLagTimeDiff)
	},
	"OPLOG_RATE_GB_PER_HOUR": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessOplogRateDataPoint(ts, float64(*dp.Value))
	},

	// Number of database operations on a MongoDB process since the process last started.

	"DB_STORAGE_TOTAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.Total)
	},

	"DB_DATA_SIZE_TOTAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.DataSize)
	},
	"DB_INDEX_SIZE_TOTAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.IndexSize)
	},
	"DB_DATA_SIZE_TOTAL_WO_SYSTEM": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbStorageDataPoint(ts, float64(*dp.Value), AttributeStorageStatus.DataSizeWoSystem)
	},

	// Rate of database operations on a MongoDB process since the process last started found in the opcounters document that the serverStatus command collects.
	"OPCOUNTER_CMD": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Cmd, AttributeClusterRole.Primary)
	},
	"OPCOUNTER_QUERY": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Query, AttributeClusterRole.Primary)
	},
	"OPCOUNTER_UPDATE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Update, AttributeClusterRole.Primary)
	},
	"OPCOUNTER_DELETE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Delete, AttributeClusterRole.Primary)
	},
	"OPCOUNTER_GETMORE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Getmore, AttributeClusterRole.Primary)
	},
	"OPCOUNTER_INSERT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Insert, AttributeClusterRole.Primary)
	},

	// Rate of database operations on MongoDB secondaries found in the opcountersRepl document that the serverStatus command collects.
	"OPCOUNTER_REPL_CMD": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Cmd, AttributeClusterRole.Replica)
	},
	"OPCOUNTER_REPL_UPDATE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Update, AttributeClusterRole.Replica)
	},
	"OPCOUNTER_REPL_DELETE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Delete, AttributeClusterRole.Replica)
	},
	"OPCOUNTER_REPL_INSERT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.Insert, AttributeClusterRole.Replica)
	},

	// Average rate of documents returned, inserted, updated, or deleted per second during a selected time period.
	"DOCUMENT_METRICS_RETURNED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Returned)
	},
	"DOCUMENT_METRICS_INSERTED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Inserted)
	},
	"DOCUMENT_METRICS_UPDATED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Updated)
	},
	"DOCUMENT_METRICS_DELETED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbDocumentRateDataPoint(ts, float64(*dp.Value), AttributeDocumentStatus.Deleted)
	},

	// Average rate for operations per second during a selected time period that perform a sort but cannot perform the sort using an index.
	"OPERATIONS_SCAN_AND_ORDER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsRateDataPoint(ts, float64(*dp.Value), AttributeOperation.ScanAndOrder, AttributeClusterRole.Primary)
	},

	// Average execution time in milliseconds per read, write, or command operation during a selected time period.
	"OP_EXECUTION_TIME_READS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionType.Reads)
	},
	"OP_EXECUTION_TIME_WRITES": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionType.Writes)
	},
	"OP_EXECUTION_TIME_COMMANDS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbOperationsTimeDataPoint(ts, float64(*dp.Value), AttributeExecutionType.Commands)
	},

	// Number of times the host restarted within the previous hour.
	"RESTARTS_IN_LAST_HOUR": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessRestartsDataPoint(ts, float64(*dp.Value))
	},

	// Average rate per second to scan index items during queries and query-plan evaluations found in the value of totalKeysExamined from the explain command.
	"QUERY_EXECUTOR_SCANNED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value), AttributeScannedType.IndexItems)
	},

	// Average rate of documents scanned per second during queries and query-plan evaluations found in the value of totalDocsExamined from the explain command.
	"QUERY_EXECUTOR_SCANNED_OBJECTS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbQueryExecutorScannedDataPoint(ts, float64(*dp.Value), AttributeScannedType.Objects)
	},

	// Ratio of the number of index items scanned to the number of documents returned.
	"QUERY_TARGETING_SCANNED_PER_RETURNED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value), AttributeScannedType.IndexItems)
	},

	// Ratio of the number of documents scanned to the number of documents returned.
	"QUERY_TARGETING_SCANNED_OBJECTS_PER_RETURNED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasProcessDbQueryTargetingScannedPerReturnedDataPoint(ts, float64(*dp.Value), AttributeScannedType.Objects)
	},

	// CPU usage of processes on the host. For hosts with more than one CPU core, this value can exceed 100%.
	"SYSTEM_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},
	"MAX_SYSTEM_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},
	"SYSTEM_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},
	"MAX_SYSTEM_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},
	"SYSTEM_CPU_NICE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
	},
	"MAX_SYSTEM_CPU_NICE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
	},
	"SYSTEM_CPU_IOWAIT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
	},
	"MAX_SYSTEM_CPU_IOWAIT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
	},
	"SYSTEM_CPU_IRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
	},
	"MAX_SYSTEM_CPU_IRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
	},
	"SYSTEM_CPU_SOFTIRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
	},
	"MAX_SYSTEM_CPU_SOFTIRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
	},
	"SYSTEM_CPU_GUEST": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
	},
	"MAX_SYSTEM_CPU_GUEST": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
	},
	"SYSTEM_CPU_STEAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
	},
	"MAX_SYSTEM_CPU_STEAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
	},

	// CPU usage of processes on the host scaled to a range of 0 to 100% by dividing by the number of CPU cores.
	"SYSTEM_NORMALIZED_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_NICE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
	},
	"SYSTEM_NORMALIZED_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},
	"SYSTEM_NORMALIZED_CPU_NICE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Nice)
	},
	"SYSTEM_NORMALIZED_CPU_IOWAIT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_IOWAIT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Iowait)
	},
	"SYSTEM_NORMALIZED_CPU_IRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_IRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Irq)
	},
	"SYSTEM_NORMALIZED_CPU_SOFTIRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_SOFTIRQ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Softirq)
	},
	"SYSTEM_NORMALIZED_CPU_GUEST": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_GUEST": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Guest)
	},
	"SYSTEM_NORMALIZED_CPU_STEAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageAverageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
	},
	"MAX_SYSTEM_NORMALIZED_CPU_STEAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemCPUNormalizedUsageMaxDataPoint(ts, float64(*dp.Value), AttributeCPUState.Steal)
	},
	// Physical memory usage, in bytes, that the host uses.
	"SYSTEM_MEMORY_AVAILABLE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Available)
	},
	"MAX_SYSTEM_MEMORY_AVAILABLE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Available)
	},
	"SYSTEM_MEMORY_BUFFERS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Buffers)
	},
	"MAX_SYSTEM_MEMORY_BUFFERS": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Buffers)
	},
	"SYSTEM_MEMORY_CACHED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Cached)
	},
	"MAX_SYSTEM_MEMORY_CACHED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Cached)
	},
	"SYSTEM_MEMORY_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Free)
	},
	"MAX_SYSTEM_MEMORY_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Free)
	},
	"SYSTEM_MEMORY_SHARED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Shared)
	},
	"MAX_SYSTEM_MEMORY_SHARED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Shared)
	},
	"SYSTEM_MEMORY_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Used)
	},
	"MAX_SYSTEM_MEMORY_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemMemoryUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryStatus.Used)
	},

	// Average rate of physical bytes per second that the eth0 network interface received and transmitted.
	"SYSTEM_NETWORK_IN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
	},
	"MAX_SYSTEM_NETWORK_IN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
	},
	"SYSTEM_NETWORK_OUT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemNetworkIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
	},
	"MAX_SYSTEM_NETWORK_OUT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemNetworkIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
	},

	// Total amount of memory that swap uses.
	"SWAP_USAGE_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Used)
	},
	"MAX_SWAP_USAGE_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Used)
	},
	"SWAP_USAGE_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingUsageAverageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Free)
	},
	"MAX_SWAP_USAGE_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingUsageMaxDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Free)
	},

	// Total amount of memory written and read from swap.
	"SWAP_IO_IN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
	},
	"MAX_SWAP_IO_IN": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Receive)
	},
	"SWAP_IO_OUT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingIoAverageDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
	},
	"MAX_SWAP_IO_OUT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemPagingIoMaxDataPoint(ts, float64(*dp.Value), AttributeDirection.Transmit)
	},

	// Memory usage, in bytes, that Atlas Search processes use.
	"FTS_PROCESS_RESIDENT_MEMORY": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Resident)
	},
	"FTS_PROCESS_VIRTUAL_MEMORY": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Virtual)
	},
	"FTS_PROCESS_SHARED_MEMORY": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Shared)
	},
	"FTS_MEMORY_MAPPED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsMemoryUsageDataPoint(ts, float64(*dp.Value), AttributeMemoryState.Mapped)
	},

	// Disk space, in bytes, that Atlas Search indexes use.
	"FTS_DISK_USAGE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsDiskUsedDataPoint(ts, float64(*dp.Value))
	},

	// Percentage of CPU that Atlas Search processes use.
	"FTS_PROCESS_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},
	"FTS_PROCESS_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsCPUUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},
	"FTS_PROCESS_NORMALIZED_CPU_USER": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.User)
	},
	"FTS_PROCESS_NORMALIZED_CPU_KERNEL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasSystemFtsCPUNormalizedUsageDataPoint(ts, float64(*dp.Value), AttributeCPUState.Kernel)
	},

	// Process Disk Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)

	// Measures throughput of I/O operations for the disk partition used for MongoDB.
	"DISK_PARTITION_IOPS_READ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
	},

	"MAX_DISK_PARTITION_IOPS_READ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
	},

	"DISK_PARTITION_IOPS_WRITE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
	},

	"MAX_DISK_PARTITION_IOPS_WRITE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
	},

	"DISK_PARTITION_IOPS_TOTAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionIopsAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Total)
	},

	"MAX_DISK_PARTITION_IOPS_TOTAL": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionIopsMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Total)
	},

	// The percentage of time during which requests are being issued to and serviced by the partition.
	// This includes requests from any process, not just MongoDB processes.
	"DISK_PARTITION_LATENCY_READ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
	},

	"MAX_DISK_PARTITION_LATENCY_READ": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Read)
	},

	"DISK_PARTITION_LATENCY_WRITE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionLatencyAverageDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
	},

	"MAX_DISK_PARTITION_LATENCY_WRITE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionLatencyMaxDataPoint(ts, float64(*dp.Value), AttributeDiskDirection.Write)
	},

	// Measures latency per operation type of the disk partition used by MongoDB.
	"DISK_PARTITION_SPACE_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
	},

	"MAX_DISK_PARTITION_SPACE_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
	},

	"DISK_PARTITION_SPACE_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionSpaceAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
	},

	"MAX_DISK_PARTITION_SPACE_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionSpaceMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
	},

	"DISK_PARTITION_SPACE_PERCENT_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionUtilizationAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
	},
	"MAX_DISK_PARTITION_SPACE_PERCENT_FREE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionUtilizationMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Free)
	},
	"DISK_PARTITION_SPACE_PERCENT_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionUtilizationAverageDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
	},
	"MAX_DISK_PARTITION_SPACE_PERCENT_USED": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDiskPartitionUtilizationMaxDataPoint(ts, float64(*dp.Value), AttributeDiskStatus.Used)
	},

	// Process Database Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)
	"DATABASE_COLLECTION_COUNT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Collection)
	},
	"DATABASE_INDEX_COUNT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Index)
	},
	"DATABASE_EXTENT_COUNT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Extent)
	},
	"DATABASE_OBJECT_COUNT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.Object)
	},
	"DATABASE_VIEW_COUNT": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbCountsDataPoint(ts, float64(*dp.Value), AttributeObjectType.View)
	},
	"DATABASE_AVERAGE_OBJECT_SIZE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Object)
	},
	"DATABASE_STORAGE_SIZE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Storage)
	},
	"DATABASE_INDEX_SIZE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Index)
	},
	"DATABASE_DATA_SIZE": func(mb *MetricsBuilder, dp *mongodbatlas.DataPoints, ts pdata.Timestamp) {
		mb.RecordMongodbatlasDbSizeDataPoint(ts, float64(*dp.Value), AttributeObjectType.Data)
	},
}

func MeasurementsToMetric(mb *MetricsBuilder, meas *mongodbatlas.Measurements, buildUnrecognized bool) error {
	recordFunc, ok := metricNameMapping[meas.Name]
	if !ok {
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
			recordFunc(mb, point, pdata.NewTimestampFromTime(curTime))
		}
	}
	return nil
}
