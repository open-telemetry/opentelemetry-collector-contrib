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

package metadata

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/model/pdata"
)

type metricMappingData struct {
	metricName string
	attributes map[string]pdata.AttributeValue
}

var metricNameMapping = map[string]metricMappingData{
	// MongoDB CPU usage. For hosts with more than one CPU core, these values can exceed 100%.

	// sfx: process.cpu.user
	"PROCESS_CPU_USER": {"process.cpu.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_CPU_USER": {"process.cpu.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx:process.cpu.kernel
	"PROCESS_CPU_KERNEL": {"process.cpu.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_CPU_KERNEL": {"process.cpu.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"PROCESS_CPU_CHILDREN_USER": {"process.cpu.children.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_CPU_CHILDREN_USER": {"process.cpu.children.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"PROCESS_CPU_CHILDREN_KERNEL": {"process.cpu.children.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_CPU_CHILDREN_KERNEL": {"process.cpu.children.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// MongoDB CPU usage scaled to a range of 0% to 100%. Atlas computes this value by dividing by the number of CPU cores.

	// sfx: process.normalized.cpu.children_user
	"PROCESS_NORMALIZED_CPU_USER": {"process.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_NORMALIZED_CPU_USER": {"process.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: process.normalized.cpu.children_kernel
	"PROCESS_NORMALIZED_CPU_KERNEL": {"process.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_NORMALIZED_CPU_KERNEL": {"process.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: process.normalized.cpu.user
	"PROCESS_NORMALIZED_CPU_CHILDREN_USER": {"process.cpu.children.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// Context: Process
	// sfx: skipped
	"MAX_PROCESS_NORMALIZED_CPU_CHILDREN_USER": {"process.cpu.children.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: process.normalized.cpu.kernel
	"PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL": {"process.cpu.children.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_PROCESS_NORMALIZED_CPU_CHILDREN_KERNEL": {"process.cpu.children.normalized.usage", map[string]pdata.AttributeValue{
		"state":       pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Rate of asserts for a MongoDB process found in the asserts document that the serverStatus command generates.

	// sfx: asserts.regular
	"ASSERT_REGULAR": {"process.asserts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("regular"),
	}},

	// sfx: asserts.warning
	"ASSERT_WARNING": {"process.asserts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("warning"),
	}},

	// sfx: asserts.msg
	"ASSERT_MSG": {"process.asserts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("msg"),
	}},

	// sfx: asserts.user
	"ASSERT_USER": {"process.asserts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("user"),
	}},

	// Amount of data flushed in the background.

	// sfx: background_flush_avg
	"BACKGROUND_FLUSH_AVG": {"process.background_flush", map[string]pdata.AttributeValue{}},

	// Amount of bytes in the WiredTiger storage engine cache and tickets found in the wiredTiger.cache and wiredTiger.concurrentTransactions documents that the serverStatus command generates.

	// sfx: cache.bytes.read_into
	"CACHE_BYTES_READ_INTO": {"process.cache.io", map[string]pdata.AttributeValue{
		"direction": pdata.NewAttributeValueString("read_into"),
	}},

	// sfx: cache.bytes.written_from
	"CACHE_BYTES_WRITTEN_FROM": {"process.cache.io", map[string]pdata.AttributeValue{
		"direction": pdata.NewAttributeValueString("written_from"),
	}},

	// sfx: cache.dirty_bytes
	"CACHE_DIRTY_BYTES": {"process.cache.size", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("dirty"),
	}},

	// sfx: cache.used_bytes
	"CACHE_USED_BYTES": {"process.cache.size", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("used"),
	}},

	// sfx: tickets.available.reads
	"TICKETS_AVAILABLE_READS": {"process.tickets", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("available_reads"),
	}},

	// sfx: tickets.available.writes
	"TICKETS_AVAILABLE_WRITE": {"process.tickets", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("available_writes"),
	}},

	// Number of connections to a MongoDB process found in the connections document that the serverStatus command generates.
	// sfx: connections.current
	"CONNECTIONS": {"process.connections", map[string]pdata.AttributeValue{}},

	// Number of cursors for a MongoDB process found in the metrics.cursor document that the serverStatus command generates.
	// sfx: cursors.total_open
	"CURSORS_TOTAL_OPEN": {"process.cursors", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("open"),
	}},

	// sfx: cursors.timed_out
	"CURSORS_TOTAL_TIMED_OUT": {"process.cursors", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("timed_out"),
	}},

	// Numbers of Memory Issues and Page Faults for a MongoDB process.
	// sfx: extra_info.page_faults
	"EXTRA_INFO_PAGE_FAULTS": {"process.page_faults", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("extra_info"),
	}},

	// sfx: skipped
	"GLOBAL_ACCESSES_NOT_IN_MEMORY": {"process.page_faults", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("global_accesses_not_in_memory"),
	}},
	// sfx: skipped
	"GLOBAL_PAGE_FAULT_EXCEPTIONS_THROWN": {"process.page_faults", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("exceptions_thrown"),
	}},

	// Number of operations waiting on locks for the MongoDB process that the serverStatus command generates. Cloud Manager computes these values based on the type of storage engine.
	// sfx: global_lock.current_queue.total
	"GLOBAL_LOCK_CURRENT_QUEUE_TOTAL": {"process.global_lock", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("current_queue_total"),
	}},
	// sfx: global_lock.current_queue.readers
	"GLOBAL_LOCK_CURRENT_QUEUE_READERS": {"process.global_lock", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("current_queue_readers"),
	}},
	// sfx: global_lock.current_queue.writers
	"GLOBAL_LOCK_CURRENT_QUEUE_WRITERS": {"process.global_lock", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("current_queue_writers"),
	}},

	// Number of index btree operations.
	// sfx: skipped
	"INDEX_COUNTERS_BTREE_ACCESSES": {"process.index.counters", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("btree_accesses"),
	}},
	// sfx: skipped
	"INDEX_COUNTERS_BTREE_HITS": {"process.index.counters", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("btree_hits"),
	}},
	// sfx: skipped
	"INDEX_COUNTERS_BTREE_MISSES": {"process.index.counters", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("btree_misses"),
	}},
	// sfx: skipped
	"INDEX_COUNTERS_BTREE_MISS_RATIO": {"process.index.btree_miss_ratio", map[string]pdata.AttributeValue{}},

	// Number of journaling operations.
	// sfx: skipped
	"JOURNALING_COMMITS_IN_WRITE_LOCK": {"process.journaling.commits", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("in_write_lock"),
	}},
	// sfx: skipped
	"JOURNALING_MB": {"process.journaling.written", map[string]pdata.AttributeValue{}},
	// sfx: skipped
	"JOURNALING_WRITE_DATA_FILES_MB": {"process.journaling.data_files", map[string]pdata.AttributeValue{}},

	// Amount of memory for a MongoDB process found in the mem document that the serverStatus command collects.
	// sfx: mem.resident
	"MEMORY_RESIDENT": {"process.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("resident"),
	}},
	// sfx: mem.virtual
	"MEMORY_VIRTUAL": {"process.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("virtual"),
	}},

	// sfx: mem.mapped
	"MEMORY_MAPPED": {"process.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("mapped"),
	}},
	// sfx: skipped
	"COMPUTED_MEMORY": {"process.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("computed"),
	}},

	// Amount of throughput for MongoDB process found in the network document that the serverStatus command collects.

	// sfx: network.bytes_in
	"NETWORK_BYTES_IN": {"process.network.io", map[string]pdata.AttributeValue{
		"direction": pdata.NewAttributeValueString("receive"),
	}},
	// sfx: network.bytes_out
	"NETWORK_BYTES_OUT": {"process.network.io", map[string]pdata.AttributeValue{
		"direction": pdata.NewAttributeValueString("transmit"),
	}},
	// sfx: network.num_requests
	"NETWORK_NUM_REQUESTS": {"process.network.requests", map[string]pdata.AttributeValue{}},

	// Durations and throughput of the MongoDB process' oplog.
	// sfx: oplog.slave.lag_master_time
	"OPLOG_SLAVE_LAG_MASTER_TIME": {"process.oplog.time", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("slave_lag_master_time"),
	}},
	// sfx: oplog.master.time
	"OPLOG_MASTER_TIME": {"process.oplog.time", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("master_time"),
	}},
	// sfx: oplog.master.lag_time_diff
	"OPLOG_MASTER_LAG_TIME_DIFF": {"process.oplog.time", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("master_lag_time_diff"),
	}},
	// sfx: skipped
	"OPLOG_RATE_GB_PER_HOUR": {"process.oplog.rate", map[string]pdata.AttributeValue{
		"aggregation": pdata.NewAttributeValueString("hour"),
	}},

	// Number of database operations on a MongoDB process since the process last started.

	// sfx: skipped
	"DB_STORAGE_TOTAL": {"process.db.storage", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("total"),
	}},

	// sfx: skipped
	"DB_DATA_SIZE_TOTAL": {"process.db.storage", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("data_size_total"),
	}},

	// Rate of database operations on a MongoDB process since the process last started found in the opcounters document that the serverStatus command collects.
	// sfx: opcounter.command
	"OPCOUNTER_CMD": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("cmd"),
		"role":      pdata.NewAttributeValueString("primary"),
	}},
	// sfx: opcounter.query
	"OPCOUNTER_QUERY": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("query"),
		"role":      pdata.NewAttributeValueString("primary"),
	}},
	// sfx: opcounter.update
	"OPCOUNTER_UPDATE": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("update"),
		"role":      pdata.NewAttributeValueString("primary"),
	}},
	// sfx: opcounter.delete
	"OPCOUNTER_DELETE": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("delete"),
		"role":      pdata.NewAttributeValueString("primary"),
	}},
	// sfx: opcounter.getmore
	"OPCOUNTER_GETMORE": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("getmore"),
		"role":      pdata.NewAttributeValueString("primary"),
	}},
	// sfx: opcounter.insert
	"OPCOUNTER_INSERT": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("insert"),
		"role":      pdata.NewAttributeValueString("primary"),
	}},

	// Rate of database operations on MongoDB secondaries found in the opcountersRepl document that the serverStatus command collects.
	// sfx: opcounter.repl.command
	"OPCOUNTER_REPL_CMD": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("cmd"),
		"role":      pdata.NewAttributeValueString("replica"),
	}},
	// sfx: opcounter.repl.update
	"OPCOUNTER_REPL_UPDATE": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("update"),
		"role":      pdata.NewAttributeValueString("replica"),
	}},
	// sfx: opcounter.repl.delete
	"OPCOUNTER_REPL_DELETE": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("delete"),
		"role":      pdata.NewAttributeValueString("replica"),
	}},
	// sfx: opcounter.repl.insert
	"OPCOUNTER_REPL_INSERT": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("insert"),
		"role":      pdata.NewAttributeValueString("replica"),
	}},

	// Average rate of documents returned, inserted, updated, or deleted per second during a selected time period.
	// sfx: document.metrics.returned
	"DOCUMENT_METRICS_RETURNED": {"process.db.document.rate", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("returned"),
	}},
	// sfx: document.metrics.inserted
	"DOCUMENT_METRICS_INSERTED": {"process.db.document.rate", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("inserted"),
	}},
	// sfx: document.metrics.updated
	"DOCUMENT_METRICS_UPDATED": {"process.db.document.rate", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("updated"),
	}},
	// sfx: document.metrics.deleted
	"DOCUMENT_METRICS_DELETED": {"process.db.document.rate", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("deleted"),
	}},

	// Average rate for operations per second during a selected time period that perform a sort but cannot perform the sort using an index.
	// sfx: operations_scan_and_order
	"OPERATIONS_SCAN_AND_ORDER": {"process.db.operations.rate", map[string]pdata.AttributeValue{
		"operation": pdata.NewAttributeValueString("scan_and_order"),
	}},

	// Average execution time in milliseconds per read, write, or command operation during a selected time period.
	// sfx: skipped
	"OP_EXECUTION_TIME_READS": {"process.db.operations.time", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("reads"),
	}},
	// sfx: skipped
	"OP_EXECUTION_TIME_WRITES": {"process.db.operations.time", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("writes"),
	}},
	// sfx: skipped
	"OP_EXECUTION_TIME_COMMANDS": {"process.db.operations.time", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("commands"),
	}},

	// Number of times the host restarted within the previous hour.
	// sfx: skipped
	"RESTARTS_IN_LAST_HOUR": {"process.restarts", map[string]pdata.AttributeValue{
		"span": pdata.NewAttributeValueString("hour"),
	}},

	// Average rate per second to scan index items during queries and query-plan evaluations found in the value of totalKeysExamined from the explain command.
	// sfx: query.executor.scanned
	"QUERY_EXECUTOR_SCANNED": {"process.db.query_executor.scanned", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("index items"),
	}},

	// Average rate of documents scanned per second during queries and query-plan evaluations found in the value of totalDocsExamined from the explain command.
	// sfx: query.executor.scanned_objects
	"QUERY_EXECUTOR_SCANNED_OBJECTS": {"process.db.query_executor.scanned", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("objects"),
	}},

	// Ratio of the number of index items scanned to the number of documents returned.
	// sfx: query.targeting.scanned_per_returned
	"QUERY_TARGETING_SCANNED_PER_RETURNED": {"process.db.query_targeting.scanned_per_returned", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("index items"),
	}},

	// Ratio of the number of documents scanned to the number of documents returned.
	// sfx: query.targeting.scanned_objects_per_returned
	"QUERY_TARGETING_SCANNED_OBJECTS_PER_RETURNED": {"process.db.query_targeting.scanned_per_returned", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("objects"),
	}},

	// CPU usage of processes on the host. For hosts with more than one CPU core, this value can exceed 100%.
	// sfx: system.cpu.user
	"SYSTEM_CPU_USER": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_USER": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.cpu.kernel
	"SYSTEM_CPU_KERNEL": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_KERNEL": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.cpu.nice
	"SYSTEM_CPU_NICE": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("nice"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: system.cpu.iowait
	"SYSTEM_CPU_IOWAIT": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("iowait"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_IOWAIT": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("iowait"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.cpu.irq
	"SYSTEM_CPU_IRQ": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("irq"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_IRQ": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("irq"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.cpu.softirq
	"SYSTEM_CPU_SOFTIRQ": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("softirq"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_SOFTIRQ": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("softirq"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.cpu.guest
	"SYSTEM_CPU_GUEST": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("guest"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_GUEST": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("guest"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.cpu.steal
	"SYSTEM_CPU_STEAL": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("steal"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_CPU_STEAL": {"system.cpu.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("steal"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// CPU usage of processes on the host scaled to a range of 0 to 100% by dividing by the number of CPU cores.
	// sfx: system.normalized.cpu.user
	"SYSTEM_NORMALIZED_CPU_USER": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_USER": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("user"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.normalized.cpu.kernel
	"SYSTEM_NORMALIZED_CPU_KERNEL": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_KERNEL": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("kernel"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.normalized.cpu.nice
	"SYSTEM_NORMALIZED_CPU_NICE": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("nice"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: system.normalized.cpu.iowait
	"SYSTEM_NORMALIZED_CPU_IOWAIT": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("iowait"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_IOWAIT": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("iowait"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.normalized.cpu.irq
	"SYSTEM_NORMALIZED_CPU_IRQ": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("irq"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_IRQ": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("irq"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.normalized.cpu.softirq
	"SYSTEM_NORMALIZED_CPU_SOFTIRQ": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("softirq"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_SOFTIRQ": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("softirq"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.normalized.cpu.guest
	"SYSTEM_NORMALIZED_CPU_GUEST": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("guest"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_GUEST": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("guest"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: system.normalized.cpu.steal
	"SYSTEM_NORMALIZED_CPU_STEAL": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("steal"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NORMALIZED_CPU_STEAL": {"system.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("steal"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Physical memory usage, in bytes, that the host uses.
	// sfx: skipped
	"SYSTEM_MEMORY_AVAILABLE": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("available"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_MEMORY_AVAILABLE": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("available"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SYSTEM_MEMORY_BUFFERS": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("buffers"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_MEMORY_BUFFERS": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("buffers"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SYSTEM_MEMORY_CACHED": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("cached"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_MEMORY_CACHED": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("cached"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SYSTEM_MEMORY_FREE": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_MEMORY_FREE": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SYSTEM_MEMORY_SHARED": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("shared"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_MEMORY_SHARED": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("shared"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SYSTEM_MEMORY_USED": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_MEMORY_USED": {"system.memory.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Average rate of physical bytes per second that the eth0 network interface received and transmitted.
	// sfx: skipped
	"SYSTEM_NETWORK_IN": {"system.network.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("receive"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NETWORK_IN": {"system.network.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("receive"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SYSTEM_NETWORK_OUT": {"system.network.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("transmit"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SYSTEM_NETWORK_OUT": {"system.network.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("transmit"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Total amount of memory that swap uses.
	// sfx: skipped
	"SWAP_USAGE_USED": {"system.paging.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SWAP_USAGE_USED": {"system.paging.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SWAP_USAGE_FREE": {"system.paging.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SWAP_USAGE_FREE": {"system.paging.usage", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Total amount of memory written and read from swap.
	// sfx: skipped
	"SWAP_IO_IN": {"system.paging.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("in"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SWAP_IO_IN": {"system.paging.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("in"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"SWAP_IO_OUT": {"system.paging.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("out"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_SWAP_IO_OUT": {"system.paging.io", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("out"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Memory usage, in bytes, that Atlas Search processes use.
	// sfx: skipped
	"FTS_MEMORY_RESIDENT": {"system.fts.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("resident"),
	}},
	// sfx: skipped
	"FTS_MEMORY_VIRTUAL": {"system.fts.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("virtual"),
	}},
	// sfx: skipped
	"FTS_MEMORY_MAPPED": {"system.fts.memory.usage", map[string]pdata.AttributeValue{
		"state": pdata.NewAttributeValueString("mapped"),
	}},

	// Disk space, in bytes, that Atlas Search indexes use.
	// sfx: skipped
	"FTS_DISK_UTILIZATION": {"system.fts.disk.utilization", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("used"),
	}},

	// Percentage of CPU that Atlas Search processes use.
	// sfx: skipped
	"FTS_PROCESS_CPU_USER": {"system.fts.cpu.usage", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("user"),
	}},
	// sfx: skipped
	"FTS_PROCESS_CPU_KERNEL": {"system.fts.cpu.usage", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("kernel"),
	}},
	// sfx: skipped
	"FTS_PROCESS_NORMAILIZED_CPU_USER": {"system.fts.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("user"),
	}},
	// sfx: skipped
	"FTS_PROCESS_NORMAILIZED_CPU_KERNEL": {"system.fts.cpu.normalized.usage", map[string]pdata.AttributeValue{
		"status": pdata.NewAttributeValueString("kernel"),
	}},

	// Process Disk Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)

	// Measures throughput of I/O operations for the disk partition used for MongoDB.
	// sfx: skipped
	"DISK_PARTITION_IOPS_READ": {"disk.partition.iops", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("read"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_IOPS_READ": {"disk.partition.iops", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("read"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"DISK_PARTITION_IOPS_WRITE": {"disk.partition.iops", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("write"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_IOPS_WRITE": {"disk.partition.iops", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("write"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"DISK_PARTITION_IOPS_TOTAL": {"disk.partition.iops", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("total"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_IOPS_TOTAL": {"disk.partition.iops", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("total"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"DISK_PARTITION_UTILIZATION": {"disk.partition.utilization", map[string]pdata.AttributeValue{
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_UTILIZATION": {"disk.partition.utilization", map[string]pdata.AttributeValue{
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// The percentage of time during which requests are being issued to and serviced by the partition.
	// This includes requests from any process, not just MongoDB processes.
	// sfx: skipped
	"DISK_PARTITION_LATENCY_READ": {"disk.partition.latency", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("read"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_LATENCY_READ": {"disk.partition.latency", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("read"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"DISK_PARTITION_LATENCY_WRITE": {"disk.partition.latency", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("write"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_LATENCY_WRITE": {"disk.partition.latency", map[string]pdata.AttributeValue{
		"direction":   pdata.NewAttributeValueString("write"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Measures latency per operation type of the disk partition used by MongoDB.
	// sfx: skipped
	"DISK_PARTITION_SPACE_FREE": {"disk.partition.space", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_SPACE_FREE": {"disk.partition.space", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"DISK_PARTITION_SPACE_USED": {"disk.partition.space", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},

	// sfx: skipped
	"MAX_DISK_PARTITION_SPACE_USED": {"disk.partition.space", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// sfx: skipped
	"DISK_PARTITION_SPACE_PERCENT_FREE": {"disk.partition.utilization", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_DISK_PARTITION_SPACE_PERCENT_FREE": {"disk.partition.utilization", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("free"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},
	// sfx: skipped
	"DISK_PARTITION_SPACE_PERCENT_USED": {"disk.partition.utilization", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("avg"),
	}},
	// sfx: skipped
	"MAX_DISK_PARTITION_SPACE_PERCENT_USED": {"disk.partition.utilization", map[string]pdata.AttributeValue{
		"status":      pdata.NewAttributeValueString("used"),
		"aggregation": pdata.NewAttributeValueString("max"),
	}},

	// Process Database Measurements (https://docs.atlas.mongodb.com/reference/api/process-disks-measurements/)
	// sfx: skipped
	"DATABASE_COLLECTION_COUNT": {"db.counts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("collection"),
	}},
	// sfx: skipped
	"DATABASE_INDEX_COUNT": {"db.counts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("index"),
	}},
	// sfx: skipped
	"DATABASE_EXTENT_COUNT": {"db.counts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("extent"),
	}},
	// sfx: skipped
	"DATABASE_OBJECT_COUNT": {"db.counts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("object"),
	}},
	// sfx: skipped
	"DATABASE_VIEW_COUNT": {"db.counts", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("view"),
	}},
	// sfx: skipped
	"DATABASE_AVERAGE_OBJECT_SIZE": {"db.size", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("object"),
	}},
	// sfx: skipped
	"DATABASE_AVERAGE_STORAGE_SIZE": {"db.size", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("storage"),
	}},
	// sfx: skipped
	"DATABASE_AVERAGE_INDEX_SIZE": {"db.size", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("index"),
	}},
	// sfx: skipped
	"DATABASE_AVERAGE_DATA_SIZE": {"db.size", map[string]pdata.AttributeValue{
		"type": pdata.NewAttributeValueString("data"),
	}},
}

func mappedMetricByName(name string) (MetricIntf, map[string]pdata.AttributeValue) {
	info, found := metricNameMapping[name]
	if !found {
		return nil, nil
	}

	metricinf := Metrics.ByName(info.metricName)
	return metricinf, info.attributes
}

func inferMetricType(mongoType string) (pdata.MetricDataType, string) {
	var dataType pdata.MetricDataType
	switch mongoType {
	case "BYTES", "MEGABYTES", "SECONDS", "KILOBYTES":
		dataType = pdata.MetricDataTypeSum
	default:
		dataType = pdata.MetricDataTypeGauge
	}

	return dataType, strings.ToLower(mongoType)
}

func buildMetricIntf(meas *mongodbatlas.Measurements) MetricIntf {
	metricType, metricUnits := inferMetricType(meas.Units)
	metricName := fmt.Sprintf("mongodb.atlas.%s", strings.ToLower(meas.Name))
	return &metricImpl{
		metricName,
		func(m pdata.Metric) {
			m.SetDataType(metricType)
			m.SetUnit(metricUnits)
		},
	}
}

func MeasurementsToMetric(meas *mongodbatlas.Measurements, buildUnrecognized bool) (*pdata.Metric, error) {
	intf, attrs := mappedMetricByName(meas.Name)
	if intf == nil {
		if buildUnrecognized {
			intf = buildMetricIntf(meas)
		} else {
			return nil, nil // Not an error- simply skipping undocumented metrics
		}
	}
	m := pdata.NewMetric()
	intf.Init(m)
	switch m.DataType() {
	case pdata.MetricDataTypeGauge:
		datapoints := m.Gauge().DataPoints()
		addDataPoints(datapoints, meas, attrs)
	case pdata.MetricDataTypeSum:
		datapoints := m.Sum().DataPoints()
		addDataPoints(datapoints, meas, attrs)
	default:
		return nil, fmt.Errorf("unrecognized data type for metric '%s'", meas.Name)
	}

	return &m, nil
}

func addDataPoints(datapoints pdata.NumberDataPointSlice, meas *mongodbatlas.Measurements, attrs map[string]pdata.AttributeValue) {
	for _, point := range meas.DataPoints {
		if point.Value != nil {
			dp := datapoints.AppendEmpty()
			curTime, err := time.Parse(time.RFC3339, point.Timestamp)
			// FIXME: handle this error
			if err != nil {
				break
			}
			for k, v := range attrs {
				dp.Attributes().Upsert(k, v)
			}
			dp.SetTimestamp(pdata.NewTimestampFromTime(curTime))
			dp.SetDoubleVal(float64(*point.Value))
		}
	}
}
