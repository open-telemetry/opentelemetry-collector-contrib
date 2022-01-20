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

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"go.opentelemetry.io/collector/model/pdata"
)

// dataPointRecorders is called once at startup. Returns recorders for all metrics (except keyspace)
// we want to extract from Redis INFO.
func (rs *redisScraper) dataPointRecorders() map[string]interface{} {
	return map[string]interface{}{
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
		"mem_fragmentation_ratio":         rs.mb.RecordRedisMemoryFragmentationRatioDataPoint,
		"rdb_changes_since_last_save":     rs.mb.RecordRedisRdbChangesSinceLastSaveDataPoint,
		"rejected_connections":            rs.mb.RecordRedisConnectionsRejectedDataPoint,
		"repl_backlog_first_byte_offset":  rs.mb.RecordRedisReplicationBacklogFirstByteOffsetDataPoint,
		"total_commands_processed":        rs.mb.RecordRedisCommandsProcessedDataPoint,
		"total_connections_received":      rs.mb.RecordRedisConnectionsReceivedDataPoint,
		"total_net_input_bytes":           rs.mb.RecordRedisNetInputDataPoint,
		"total_net_output_bytes":          rs.mb.RecordRedisNetOutputDataPoint,
		"uptime_in_seconds":               rs.mb.RecordRedisUptimeDataPoint,
		"used_cpu_sys":                    rs.recordUsedCPUSys,
		"used_cpu_sys_children":           rs.recordUsedCPUSysChildren,
		"used_cpu_user":                   rs.recordUsedCPUSysUser,
		"used_memory":                     rs.mb.RecordRedisMemoryUsedDataPoint,
		"used_memory_lua":                 rs.mb.RecordRedisMemoryLuaDataPoint,
		"used_memory_peak":                rs.mb.RecordRedisMemoryPeakDataPoint,
		"used_memory_rss":                 rs.mb.RecordRedisMemoryRssDataPoint,
	}
}

func (rs *redisScraper) recordUsedCPUSys(now pdata.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, "sys")
}

func (rs *redisScraper) recordUsedCPUSysChildren(now pdata.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, "children")
}

func (rs *redisScraper) recordUsedCPUSysUser(now pdata.Timestamp, val float64) {
	rs.mb.RecordRedisCPUTimeDataPoint(now, val, "user")
}
