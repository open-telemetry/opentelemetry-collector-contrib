package redisreceiver

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// Called once at startup. Returns all of the metrics (except keyspace)
// we want to extract from Redis INFO.
func getDefaultRedisMetrics() []*redisMetric {
	return []*redisMetric{
		uptimeInSeconds(),

		usedCpuSys(),
		usedCpuSysChildren(),
		usedCpuUser(),

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

		rdbBgsaveInProgress(),

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
		key:    "uptime_in_seconds",
		name:   "redis/uptime",
		units:  "s",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func usedCpuSys() *redisMetric {
	return &redisMetric{
		key:    "used_cpu_sys",
		name:   "redis/cpu/time",
		units:  "s",
		mdType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
		labels: map[string]string{"state": "sys"},
	}
}

func usedCpuUser() *redisMetric {
	return &redisMetric{
		key:    "used_cpu_user",
		name:   "redis/cpu/time",
		units:  "s",
		mdType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
		labels: map[string]string{"state": "user"},
	}
}

func usedCpuSysChildren() *redisMetric {
	return &redisMetric{
		key:    "used_cpu_sys_children",
		name:   "redis/cpu/time",
		units:  "s",
		mdType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
		labels: map[string]string{"state": "children"},
	}
}

func connectedClients() *redisMetric {
	return &redisMetric{
		key:    "connected_clients",
		name:   "redis/clients/connected",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func clientRecentMaxInputBuffer() *redisMetric {
	return &redisMetric{
		key:    "client_recent_max_input_buffer",
		name:   "redis/clients/max_input_buffer",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func clientRecentMaxOutputBuffer() *redisMetric {
	return &redisMetric{
		key:    "client_recent_max_output_buffer",
		name:   "redis/clients/max_output_buffer",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func blockedClients() *redisMetric {
	return &redisMetric{
		key:    "blocked_clients",
		name:   "redis/clients/blocked",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func expiredKeys() *redisMetric {
	return &redisMetric{
		key:    "expired_keys",
		name:   "redis/keys/expired",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func evictedKeys() *redisMetric {
	return &redisMetric{
		key:    "evicted_keys",
		name:   "redis/keys/evicted",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func totalConnectionsReceived() *redisMetric {
	return &redisMetric{
		key:    "total_connections_received",
		name:   "redis/connections/received",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func rejectedConnections() *redisMetric {
	return &redisMetric{
		key:    "rejected_connections",
		name:   "redis/connections/rejected",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func usedMemory() *redisMetric {
	return &redisMetric{
		key:    "used_memory",
		name:   "redis/memory/used",
		units:  "By",
		desc:   "memory used",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func usedMemoryPeak() *redisMetric {
	return &redisMetric{
		key:    "used_memory_peak",
		name:   "redis/memory/peak",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
		units:  "By",
	}
}

func usedMemoryRss() *redisMetric {
	return &redisMetric{
		key:    "used_memory_rss",
		name:   "redis/memory/rss",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
		units:  "By",
	}
}

func usedMemoryLua() *redisMetric {
	return &redisMetric{
		key:    "used_memory_lua",
		name:   "redis/memory/lua",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
		units:  "By",
	}
}

func memFragmentationRatio() *redisMetric {
	return &redisMetric{
		key:    "mem_fragmentation_ratio",
		name:   "redis/memory/fragmentation_ratio",
		mdType: metricspb.MetricDescriptor_GAUGE_DOUBLE,
	}
}

func rdbChangesSinceLastSave() *redisMetric {
	return &redisMetric{
		key:    "rdb_changes_since_last_save",
		name:   "redis/rdb/changes_since_last_save",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func rdbBgsaveInProgress() *redisMetric {
	return &redisMetric{
		key:    "rdb_bgsave_in_progress",
		name:   "redis/rdb/bgsave_in_progress",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func instantaneousOpsPerSec() *redisMetric {
	return &redisMetric{
		key:    "instantaneous_ops_per_sec",
		name:   "redis/commands",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
		units:  "{ops}/s",
	}
}

func totalCommandsProcessed() *redisMetric {
	return &redisMetric{
		key:    "total_commands_processed",
		name:   "redis/commands/processed",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func totalNetInputBytes() *redisMetric {
	return &redisMetric{
		key:    "total_net_input_bytes",
		name:   "redis/net/input",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
		units:  "By",
	}
}

func totalNetOutputBytes() *redisMetric {
	return &redisMetric{
		key:    "total_net_output_bytes",
		name:   "redis/net/output",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
		units:  "By",
	}
}

func keyspaceHits() *redisMetric {
	return &redisMetric{
		key:    "keyspace_hits",
		name:   "redis/keyspace/hits",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func keyspaceMisses() *redisMetric {
	return &redisMetric{
		key:    "keyspace_misses",
		name:   "redis/keyspace/misses",
		mdType: metricspb.MetricDescriptor_CUMULATIVE_INT64,
	}
}

func latestForkUsec() *redisMetric {
	return &redisMetric{
		key:    "latest_fork_usec",
		name:   "redis/latest_fork",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
		units:  "us",
	}
}

func connectedSlaves() *redisMetric {
	return &redisMetric{
		key:    "connected_slaves",
		name:   "redis/slaves/connected",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func replBacklogFirstByteOffset() *redisMetric {
	return &redisMetric{
		key:    "repl_backlog_first_byte_offset",
		name:   "redis/replication/backlog_first_byte_offset",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}

func masterReplOffset() *redisMetric {
	return &redisMetric{
		key:    "master_repl_offset",
		name:   "redis/replication/offset",
		mdType: metricspb.MetricDescriptor_GAUGE_INT64,
	}
}
