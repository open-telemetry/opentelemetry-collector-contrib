package redisreceiver

func uptimeInSeconds() *redisMetric {
	return &redisMetric{
		key:        "uptime_in_seconds",
		units:      "seconds",
		desc:       "uptime",
		metricType: cumulativeInt,
	}
}

func uptimeInDays() *redisMetric {
	return &redisMetric{
		key:        "uptime_in_days",
		units:      "days",
		desc:       "uptime",
		metricType: cumulativeInt,
	}
}

func usedCpuSys() *redisMetric {
	return &redisMetric{
		key:        "used_cpu_sys",
		units:      "mhz",
		metricType: gaugeDouble,
	}
}

func usedCpuSysChildren() *redisMetric {
	return &redisMetric{
		key:        "used_cpu_sys_children",
		units:      "mhz",
		metricType: gaugeDouble,
	}
}

func usedCpuSysUserChildren() *redisMetric {
	return &redisMetric{
		key:        "used_cpu_sys_user_children",
		units:      "mhz",
		metricType: gaugeDouble,
	}
}

func lruClock() *redisMetric {
	return &redisMetric{
		key:        "lru_clock",
		units:      "ms",
		metricType: cumulativeInt,
	}
}

func connectedClients() *redisMetric {
	return &redisMetric{
		key:        "connected_clients",
		metricType: gaugeInt,
	}
}

func clientRecentMaxInputBuffer() *redisMetric {
	return &redisMetric{
		key:        "client_recent_max_input_buffer",
		metricType: gaugeInt,
	}
}

func clientRecentMaxOutputBuffer() *redisMetric {
	return &redisMetric{
		key:        "client_recent_max_output_buffer",
		metricType: gaugeInt,
	}
}

func blockedClients() *redisMetric {
	return &redisMetric{
		key:        "blocked_clients",
		metricType: gaugeInt,
	}
}

func expiredKeys() *redisMetric {
	return &redisMetric{
		key:        "expired_keys",
		metricType: cumulativeInt,
	}
}

func evictedKeys() *redisMetric {
	return &redisMetric{
		key:        "evicted_keys",
		metricType: cumulativeInt,
	}
}

func rejectedConnections() *redisMetric {
	return &redisMetric{
		key:        "rejected_connections",
		metricType: cumulativeInt,
	}
}

func usedMemory() *redisMetric {
	return &redisMetric{
		key:        "used_memory",
		units:      "bytes",
		desc:       "memory used",
		metricType: gaugeInt,
	}
}

func usedMemoryRss() *redisMetric {
	return &redisMetric{
		key:        "used_memory_rss",
		metricType: gaugeInt,
		units:      "bytes",
	}
}

func usedMemoryPeak() *redisMetric {
	return &redisMetric{
		key:        "used_memory_peak",
		metricType: gaugeInt,
		units:      "bytes",
	}
}

func usedMemoryLua() *redisMetric {
	return &redisMetric{
		key:        "used_memory_lua",
		metricType: gaugeInt,
		units:      "bytes",
	}
}

func memFragmentationRatio() *redisMetric {
	return &redisMetric{
		key:        "mem_fragmentation_ratio",
		metricType: gaugeDouble,
	}
}

func changesSinceLastSave() *redisMetric {
	return &redisMetric{
		key:        "changes_since_last_save",
		metricType: gaugeInt,
	}
}

func instantaneousOpsPerSec() *redisMetric {
	return &redisMetric{
		key:        "instantaneous_ops_per_sec",
		metricType: gaugeInt,
	}
}

func rdbBgsaveInProgress() *redisMetric {
	return &redisMetric{
		key:        "rdb_bgsave_in_progress",
		metricType: gaugeInt,
	}
}

func totalConnectionsReceived() *redisMetric {
	return &redisMetric{
		key:        "total_connections_received",
		metricType: cumulativeInt,
	}
}

func totalCommandsProcessed() *redisMetric {
	return &redisMetric{
		key:        "total_commands_processed",
		metricType: cumulativeInt,
	}
}

func totalNetInputBytes() *redisMetric {
	return &redisMetric{
		key:        "total_net_input_bytes",
		metricType: cumulativeInt,
	}
}

func totalNetOutputBytes() *redisMetric {
	return &redisMetric{
		key:        "total_net_output_bytes",
		metricType: cumulativeInt,
	}
}

func keyspaceHits() *redisMetric {
	return &redisMetric{
		key:        "keyspace_hits",
		metricType: cumulativeInt,
	}
}

func keyspaceMisses() *redisMetric {
	return &redisMetric{
		key:        "keyspace_misses",
		metricType: cumulativeInt,
	}
}

func latestForkUsec() *redisMetric {
	return &redisMetric{
		key:        "latest_fork_usec",
		metricType: gaugeInt,
	}
}

func connectedSlaves() *redisMetric {
	return &redisMetric{
		key:        "connected_slaves",
		metricType: gaugeInt,
	}
}

func replBacklogFirstByteOffset() *redisMetric {
	return &redisMetric{
		key:        "repl_backlog_first_byte_offset",
		metricType: gaugeInt,
	}
}

func masterReplOffset() *redisMetric {
	return &redisMetric{
		key:        "master_repl_offset",
		metricType: gaugeInt,
	}
}
