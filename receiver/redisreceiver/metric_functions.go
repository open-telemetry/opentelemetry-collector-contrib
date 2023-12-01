// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

// dataPointRecorders is called once at startup. Returns recorders for all metrics (except keyspace)
// we want to extract from Redis INFO.
func (rs *redisScraper) dataPointRecorders() map[string]any {
	return map[string]any{
		"blocked_clients":                 rs.mb.RecordRedisClientsBlockedDataPoint,
		"client_recent_max_input_buffer":  rs.mb.RecordRedisClientsMaxInputBufferDataPoint,
		"client_recent_max_output_buffer": rs.mb.RecordRedisClientsMaxOutputBufferDataPoint,
		"connected_clients":               rs.mb.RecordRedisClientsConnectedDataPoint,
		"connected_slaves":                rs.mb.RecordRedisSlavesConnectedDataPoint,
		"evicted_keys":                    rs.mb.RecordRedisKeysEvictedDataPoint,
		"expired_keys":                    rs.mb.RecordRedisKeysExpiredDataPoint,
		"instantaneous_ops_per_sec":       rs.mb.RecordRedisCommandsDataPoint,
		"keyspace_hits":                   rs.mb.RecordRedisKeyspaceHitsDataPoint,
		"keyspace_misses":                 rs.mb.RecordRedisKeyspaceMissesDataPoint,
		"latest_fork_usec":                rs.mb.RecordRedisLatestForkDataPoint,
		"master_repl_offset":              rs.mb.RecordRedisReplicationOffsetDataPoint,
		"maxmemory":                       rs.mb.RecordRedisMaxmemoryDataPoint,
		"mem_fragmentation_ratio":         rs.mb.RecordRedisMemoryFragmentationRatioDataPoint,
		"rdb_changes_since_last_save":     rs.mb.RecordRedisRdbChangesSinceLastSaveDataPoint,
		"rejected_connections":            rs.mb.RecordRedisConnectionsRejectedDataPoint,
		"repl_backlog_first_byte_offset":  rs.mb.RecordRedisReplicationBacklogFirstByteOffsetDataPoint,
		"slave_repl_offset":               rs.mb.RecordRedisSlaveReplicationOffsetDataPoint,
		"total_commands_processed":        rs.mb.RecordRedisCommandsProcessedDataPoint,
		"total_connections_received":      rs.mb.RecordRedisConnectionsReceivedDataPoint,
		"total_net_input_bytes":           rs.mb.RecordRedisNetInputDataPoint,
		"total_net_output_bytes":          rs.mb.RecordRedisNetOutputDataPoint,
		"uptime_in_seconds":               rs.mb.RecordRedisUptimeDataPoint,
		"used_cpu_sys":                    rs.recordUsedCPUSys,
		"used_cpu_sys_children":           rs.recordUsedCPUSysChildren,
		"used_cpu_sys_main_thread":        rs.recordUsedCPUSysMainThread,
		"used_cpu_user":                   rs.recordUsedCPUUser,
		"used_cpu_user_children":          rs.recordUsedCPUUserChildren,
		"used_cpu_user_main_thread":       rs.recordUsedCPUUserMainThread,
		"used_memory":                     rs.mb.RecordRedisMemoryUsedDataPoint,
		"used_memory_lua":                 rs.mb.RecordRedisMemoryLuaDataPoint,
		"used_memory_peak":                rs.mb.RecordRedisMemoryPeakDataPoint,
		"used_memory_rss":                 rs.mb.RecordRedisMemoryRssDataPoint,
	}
}

func (rs *redisScraper) recordUsedCPUSys(now pcommon.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateSys)
}

func (rs *redisScraper) recordUsedCPUSysChildren(now pcommon.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateSysChildren)
}

func (rs *redisScraper) recordUsedCPUSysMainThread(now pcommon.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateSysMainThread)
}

func (rs *redisScraper) recordUsedCPUUser(now pcommon.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateUser)
}

func (rs *redisScraper) recordUsedCPUUserChildren(now pcommon.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateUserChildren)
}

func (rs *redisScraper) recordUsedCPUUserMainThread(now pcommon.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, metadata.AttributeStateUserMainThread)
}
