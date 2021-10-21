// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redisreceiver

import (
	"go.opentelemetry.io/collector/model/pdata"
)

// Called once at startup. Returns all of the metrics (except keyspace)
// we want to extract from Redis INFO.
func getDefaultRedisMetrics() []*redisMetric {
	return []*redisMetric{
		uptimeInSeconds(),

		usedCPUSys(),
		usedCPUSysChildren(),
		usedCPUUser(),

		connectedClients(),

		clientRecentMaxInputBuffer(),
		clientRecentMaxOutputBuffer(),

		blockedClients(),

		expiredKeys(),
		evictedKeys(),

		rejectedConnections(),

		usedMemory(),
		usedMemoryRss(),
		usedMemoryPeak(),
		usedMemoryLua(),

		memFragmentationRatio(),

		rdbChangesSinceLastSave(),

		instantaneousOpsPerSec(),

		totalConnectionsReceived(),
		totalCommandsProcessed(),

		totalNetInputBytes(),
		totalNetOutputBytes(),

		keyspaceHits(),
		keyspaceMisses(),

		latestForkUsec(),

		connectedSlaves(),

		replBacklogFirstByteOffset(),

		masterReplOffset(),
	}
}

func uptimeInSeconds() *redisMetric {
	return &redisMetric{
		key:         "uptime_in_seconds",
		name:        "redis/uptime",
		units:       "s",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Number of seconds since Redis server start",
	}
}

func usedCPUSys() *redisMetric {
	return &redisMetric{
		key:         "used_cpu_sys",
		name:        "redis/cpu/time",
		units:       "s",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeDouble,
		isMonotonic: true,
		labels:      map[string]pdata.AttributeValue{"state": pdata.NewAttributeValueString("sys")},
		desc:        "System CPU consumed by the Redis server in seconds since server start",
	}
}

func usedCPUUser() *redisMetric {
	return &redisMetric{
		key:         "used_cpu_user",
		name:        "redis/cpu/time",
		units:       "s",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeDouble,
		isMonotonic: true,
		labels:      map[string]pdata.AttributeValue{"state": pdata.NewAttributeValueString("user")},
		desc:        "User CPU consumed by the Redis server in seconds since server start",
	}
}

func usedCPUSysChildren() *redisMetric {
	return &redisMetric{
		key:         "used_cpu_sys_children",
		name:        "redis/cpu/time",
		units:       "s",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeDouble,
		isMonotonic: true,
		labels:      map[string]pdata.AttributeValue{"state": pdata.NewAttributeValueString("children")},
		desc:        "User CPU consumed by the background processes in seconds since server start",
	}
}

func connectedClients() *redisMetric {
	return &redisMetric{
		key:         "connected_clients",
		name:        "redis/clients/connected",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: false,
		desc:        "Number of client connections (excluding connections from replicas)",
	}
}

func clientRecentMaxInputBuffer() *redisMetric {
	return &redisMetric{
		key:       "client_recent_max_input_buffer",
		name:      "redis/clients/max_input_buffer",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		desc:      "Biggest input buffer among current client connections",
	}
}

func clientRecentMaxOutputBuffer() *redisMetric {
	return &redisMetric{
		key:       "client_recent_max_output_buffer",
		name:      "redis/clients/max_output_buffer",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		desc:      "Longest output list among current client connections",
	}
}

func blockedClients() *redisMetric {
	return &redisMetric{
		key:         "blocked_clients",
		name:        "redis/clients/blocked",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: false,
		desc:        "Number of clients pending on a blocking call",
	}
}

func expiredKeys() *redisMetric {
	return &redisMetric{
		key:         "expired_keys",
		name:        "redis/keys/expired",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Total number of key expiration events",
	}
}

func evictedKeys() *redisMetric {
	return &redisMetric{
		key:         "evicted_keys",
		name:        "redis/keys/evicted",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Number of evicted keys due to maxmemory limit",
	}
}

func totalConnectionsReceived() *redisMetric {
	return &redisMetric{
		key:         "total_connections_received",
		name:        "redis/connections/received",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Total number of connections accepted by the server",
	}
}

func rejectedConnections() *redisMetric {
	return &redisMetric{
		key:         "rejected_connections",
		name:        "redis/connections/rejected",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Number of connections rejected because of maxclients limit",
	}
}

func usedMemory() *redisMetric {
	return &redisMetric{
		key:       "used_memory",
		name:      "redis/memory/used",
		units:     "By",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		desc:      "Total number of bytes allocated by Redis using its allocator",
	}
}

func usedMemoryPeak() *redisMetric {
	return &redisMetric{
		key:       "used_memory_peak",
		name:      "redis/memory/peak",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		units:     "By",
		desc:      "Peak memory consumed by Redis (in bytes)",
	}
}

func usedMemoryRss() *redisMetric {
	return &redisMetric{
		key:       "used_memory_rss",
		name:      "redis/memory/rss",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		units:     "By",
		desc:      "Number of bytes that Redis allocated as seen by the operating system",
	}
}

func usedMemoryLua() *redisMetric {
	return &redisMetric{
		key:       "used_memory_lua",
		name:      "redis/memory/lua",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		units:     "By",
		desc:      "Number of bytes used by the Lua engine",
	}
}

func memFragmentationRatio() *redisMetric {
	return &redisMetric{
		key:       "mem_fragmentation_ratio",
		name:      "redis/memory/fragmentation_ratio",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeDouble,
		desc:      "Ratio between used_memory_rss and used_memory",
	}
}

func rdbChangesSinceLastSave() *redisMetric {
	return &redisMetric{
		key:         "rdb_changes_since_last_save",
		name:        "redis/rdb/changes_since_last_save",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: false,
		desc:        "Number of changes since the last dump",
	}
}

func instantaneousOpsPerSec() *redisMetric {
	return &redisMetric{
		key:       "instantaneous_ops_per_sec",
		name:      "redis/commands",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		units:     "{ops}/s",
		desc:      "Number of commands processed per second",
	}
}

func totalCommandsProcessed() *redisMetric {
	return &redisMetric{
		key:         "total_commands_processed",
		name:        "redis/commands/processed",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Total number of commands processed by the server",
	}
}

func totalNetInputBytes() *redisMetric {
	return &redisMetric{
		key:         "total_net_input_bytes",
		name:        "redis/net/input",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		units:       "By",
		desc:        "The total number of bytes read from the network",
	}
}

func totalNetOutputBytes() *redisMetric {
	return &redisMetric{
		key:         "total_net_output_bytes",
		name:        "redis/net/output",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		units:       "By",
		desc:        "The total number of bytes written to the network",
	}
}

func keyspaceHits() *redisMetric {
	return &redisMetric{
		key:         "keyspace_hits",
		name:        "redis/keyspace/hits",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Number of successful lookup of keys in the main dictionary",
	}
}

func keyspaceMisses() *redisMetric {
	return &redisMetric{
		key:         "keyspace_misses",
		name:        "redis/keyspace/misses",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: true,
		desc:        "Number of failed lookup of keys in the main dictionary",
	}
}

func latestForkUsec() *redisMetric {
	return &redisMetric{
		key:       "latest_fork_usec",
		name:      "redis/latest_fork",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		units:     "us",
		desc:      "Duration of the latest fork operation in microseconds",
	}
}

func connectedSlaves() *redisMetric {
	return &redisMetric{
		key:         "connected_slaves",
		name:        "redis/slaves/connected",
		pdType:      pdata.MetricDataTypeSum,
		valueType:   pdata.MetricValueTypeInt,
		isMonotonic: false,
		desc:        "Number of connected replicas",
	}
}

func replBacklogFirstByteOffset() *redisMetric {
	return &redisMetric{
		key:       "repl_backlog_first_byte_offset",
		name:      "redis/replication/backlog_first_byte_offset",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		desc:      "The master offset of the replication backlog buffer",
	}
}

func masterReplOffset() *redisMetric {
	return &redisMetric{
		key:       "master_repl_offset",
		name:      "redis/replication/offset",
		pdType:    pdata.MetricDataTypeGauge,
		valueType: pdata.MetricValueTypeInt,
		desc:      "The server's current replication offset",
	}
}
