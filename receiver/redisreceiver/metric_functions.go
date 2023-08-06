// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

// dataPointRecorders is called once at startup. Returns recorders for all metrics (except keyspace)
// we want to extract from Redis INFO.
func dataPointRecorders(rmb *metadata.ResourceMetricsBuilder) map[string]interface{} {
	return map[string]interface{}{
		"blocked_clients":                 rmb.RecordRedisClientsBlockedDataPoint,
		"client_recent_max_input_buffer":  rmb.RecordRedisClientsMaxInputBufferDataPoint,
		"client_recent_max_output_buffer": rmb.RecordRedisClientsMaxOutputBufferDataPoint,
		"connected_clients":               rmb.RecordRedisClientsConnectedDataPoint,
		"connected_slaves":                rmb.RecordRedisSlavesConnectedDataPoint,
		"evicted_keys":                    rmb.RecordRedisKeysEvictedDataPoint,
		"expired_keys":                    rmb.RecordRedisKeysExpiredDataPoint,
		"instantaneous_ops_per_sec":       rmb.RecordRedisCommandsDataPoint,
		"keyspace_hits":                   rmb.RecordRedisKeyspaceHitsDataPoint,
		"keyspace_misses":                 rmb.RecordRedisKeyspaceMissesDataPoint,
		"latest_fork_usec":                rmb.RecordRedisLatestForkDataPoint,
		"master_repl_offset":              rmb.RecordRedisReplicationOffsetDataPoint,
		"maxmemory":                       rmb.RecordRedisMaxmemoryDataPoint,
		"mem_fragmentation_ratio":         rmb.RecordRedisMemoryFragmentationRatioDataPoint,
		"rdb_changes_since_last_save":     rmb.RecordRedisRdbChangesSinceLastSaveDataPoint,
		"rejected_connections":            rmb.RecordRedisConnectionsRejectedDataPoint,
		"repl_backlog_first_byte_offset":  rmb.RecordRedisReplicationBacklogFirstByteOffsetDataPoint,
		"total_commands_processed":        rmb.RecordRedisCommandsProcessedDataPoint,
		"total_connections_received":      rmb.RecordRedisConnectionsReceivedDataPoint,
		"total_net_input_bytes":           rmb.RecordRedisNetInputDataPoint,
		"total_net_output_bytes":          rmb.RecordRedisNetOutputDataPoint,
		"uptime_in_seconds":               rmb.RecordRedisUptimeDataPoint,
		"used_cpu_sys": func(now pcommon.Timestamp, val float64) {
			rmb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateSys)
		},
		"used_cpu_sys_children": func(now pcommon.Timestamp, val float64) {
			rmb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateSysChildren)
		},
		"used_cpu_sys_main_thread": func(now pcommon.Timestamp, val float64) {
			rmb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateSysMainThread)
		},
		"used_cpu_user": func(now pcommon.Timestamp, val float64) {
			rmb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateUser)
		},
		"used_cpu_user_children": func(now pcommon.Timestamp, val float64) {
			rmb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateUserChildren)
		},
		"used_cpu_user_main_thread": func(now pcommon.Timestamp, val float64) {
			rmb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateUserMainThread)
		},
		"used_memory":      rmb.RecordRedisMemoryUsedDataPoint,
		"used_memory_lua":  rmb.RecordRedisMemoryLuaDataPoint,
		"used_memory_peak": rmb.RecordRedisMemoryPeakDataPoint,
		"used_memory_rss":  rmb.RecordRedisMemoryRssDataPoint,
	}
}
