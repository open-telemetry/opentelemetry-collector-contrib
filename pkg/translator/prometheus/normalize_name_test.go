// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestByte(t *testing.T) {

	require.Equal(t, "system_filesystem_usage_bytes", normalizeName(createGauge("system.filesystem.usage", "By"), ""))

}

func TestByteCounter(t *testing.T) {

	require.Equal(t, "system_io_bytes_total", normalizeName(createCounter("system.io", "By"), ""))
	require.Equal(t, "network_transmitted_bytes_total", normalizeName(createCounter("network_transmitted_bytes_total", "By"), ""))

}

func TestWhiteSpaces(t *testing.T) {

	require.Equal(t, "system_filesystem_usage_bytes", normalizeName(createGauge("\t system.filesystem.usage       ", "  By\t"), ""))

}

func TestNonStandardUnit(t *testing.T) {

	require.Equal(t, "system_network_dropped", normalizeName(createGauge("system.network.dropped", "{packets}"), ""))

}

func TestNonStandardUnitCounter(t *testing.T) {

	require.Equal(t, "system_network_dropped_total", normalizeName(createCounter("system.network.dropped", "{packets}"), ""))

}

func TestBrokenUnit(t *testing.T) {

	require.Equal(t, "system_network_dropped_packets", normalizeName(createGauge("system.network.dropped", "packets"), ""))
	require.Equal(t, "system_network_packets_dropped", normalizeName(createGauge("system.network.packets.dropped", "packets"), ""))
	require.Equal(t, "system_network_packets", normalizeName(createGauge("system.network.packets", "packets"), ""))

}

func TestBrokenUnitCounter(t *testing.T) {

	require.Equal(t, "system_network_dropped_packets_total", normalizeName(createCounter("system.network.dropped", "packets"), ""))
	require.Equal(t, "system_network_packets_dropped_total", normalizeName(createCounter("system.network.packets.dropped", "packets"), ""))
	require.Equal(t, "system_network_packets_total", normalizeName(createCounter("system.network.packets", "packets"), ""))

}

func TestRatio(t *testing.T) {

	require.Equal(t, "hw_gpu_memory_utilization_ratio", normalizeName(createGauge("hw.gpu.memory.utilization", "1"), ""))
	require.Equal(t, "hw_fan_speed_ratio", normalizeName(createGauge("hw.fan.speed_ratio", "1"), ""))
	require.Equal(t, "objects_total", normalizeName(createCounter("objects", "1"), ""))

}

func TestHertz(t *testing.T) {

	require.Equal(t, "hw_cpu_speed_limit_hertz", normalizeName(createGauge("hw.cpu.speed_limit", "Hz"), ""))

}

func TestPer(t *testing.T) {

	require.Equal(t, "broken_metric_speed_km_per_hour", normalizeName(createGauge("broken.metric.speed", "km/h"), ""))
	require.Equal(t, "astro_light_speed_limit_meters_per_second", normalizeName(createGauge("astro.light.speed_limit", "m/s"), ""))

}

func TestPercent(t *testing.T) {

	require.Equal(t, "broken_metric_success_ratio_percent", normalizeName(createGauge("broken.metric.success_ratio", "%"), ""))
	require.Equal(t, "broken_metric_success_percent", normalizeName(createGauge("broken.metric.success_percent", "%"), ""))

}

func TestDollar(t *testing.T) {

	require.Equal(t, "crypto_bitcoin_value_dollars", normalizeName(createGauge("crypto.bitcoin.value", "$"), ""))
	require.Equal(t, "crypto_bitcoin_value_dollars", normalizeName(createGauge("crypto.bitcoin.value.dollars", "$"), ""))

}

func TestEmpty(t *testing.T) {

	require.Equal(t, "test_metric_no_unit", normalizeName(createGauge("test.metric.no_unit", ""), ""))
	require.Equal(t, "test_metric_spaces", normalizeName(createGauge("test.metric.spaces", "   \t  "), ""))

}

func TestUnsupportedRunes(t *testing.T) {

	require.Equal(t, "unsupported_metric_temperature_F", normalizeName(createGauge("unsupported.metric.temperature", "°F"), ""))
	require.Equal(t, "unsupported_metric_weird", normalizeName(createGauge("unsupported.metric.weird", "+=.:,!* & #"), ""))
	require.Equal(t, "unsupported_metric_redundant_test_per_C", normalizeName(createGauge("unsupported.metric.redundant", "__test $/°C"), ""))

}

func TestOtelReceivers(t *testing.T) {

	require.Equal(t, "active_directory_ds_replication_network_io_bytes_total", normalizeName(createCounter("active_directory.ds.replication.network.io", "By"), ""))
	require.Equal(t, "active_directory_ds_replication_sync_object_pending_total", normalizeName(createCounter("active_directory.ds.replication.sync.object.pending", "{objects}"), ""))
	require.Equal(t, "active_directory_ds_replication_object_rate_per_second", normalizeName(createGauge("active_directory.ds.replication.object.rate", "{objects}/s"), ""))
	require.Equal(t, "active_directory_ds_name_cache_hit_rate_percent", normalizeName(createGauge("active_directory.ds.name_cache.hit_rate", "%"), ""))
	require.Equal(t, "active_directory_ds_ldap_bind_last_successful_time_milliseconds", normalizeName(createGauge("active_directory.ds.ldap.bind.last_successful.time", "ms"), ""))
	require.Equal(t, "apache_current_connections", normalizeName(createGauge("apache.current_connections", "connections"), ""))
	require.Equal(t, "apache_workers_connections", normalizeName(createGauge("apache.workers", "connections"), ""))
	require.Equal(t, "apache_requests_total", normalizeName(createCounter("apache.requests", "1"), ""))
	require.Equal(t, "bigip_virtual_server_request_count_total", normalizeName(createCounter("bigip.virtual_server.request.count", "{requests}"), ""))
	require.Equal(t, "system_cpu_utilization_ratio", normalizeName(createGauge("system.cpu.utilization", "1"), ""))
	require.Equal(t, "system_disk_operation_time_seconds_total", normalizeName(createCounter("system.disk.operation_time", "s"), ""))
	require.Equal(t, "system_cpu_load_average_15m_ratio", normalizeName(createGauge("system.cpu.load_average.15m", "1"), ""))
	require.Equal(t, "memcached_operation_hit_ratio_percent", normalizeName(createGauge("memcached.operation_hit_ratio", "%"), ""))
	require.Equal(t, "mongodbatlas_process_asserts_per_second", normalizeName(createGauge("mongodbatlas.process.asserts", "{assertions}/s"), ""))
	require.Equal(t, "mongodbatlas_process_journaling_data_files_mebibytes", normalizeName(createGauge("mongodbatlas.process.journaling.data_files", "MiBy"), ""))
	require.Equal(t, "mongodbatlas_process_network_io_bytes_per_second", normalizeName(createGauge("mongodbatlas.process.network.io", "By/s"), ""))
	require.Equal(t, "mongodbatlas_process_oplog_rate_gibibytes_per_hour", normalizeName(createGauge("mongodbatlas.process.oplog.rate", "GiBy/h"), ""))
	require.Equal(t, "mongodbatlas_process_db_query_targeting_scanned_per_returned", normalizeName(createGauge("mongodbatlas.process.db.query_targeting.scanned_per_returned", "{scanned}/{returned}"), ""))
	require.Equal(t, "nginx_requests", normalizeName(createGauge("nginx.requests", "requests"), ""))
	require.Equal(t, "nginx_connections_accepted", normalizeName(createGauge("nginx.connections_accepted", "connections"), ""))
	require.Equal(t, "nsxt_node_memory_usage_kilobytes", normalizeName(createGauge("nsxt.node.memory.usage", "KBy"), ""))
	require.Equal(t, "redis_latest_fork_microseconds", normalizeName(createGauge("redis.latest_fork", "us"), ""))

}

func TestNamespace(t *testing.T) {
	require.Equal(t, "space_test", normalizeName(createGauge("test", ""), "space"))
	require.Equal(t, "space_test", normalizeName(createGauge("#test", ""), "space"))
}

func TestCleanUpString(t *testing.T) {
	require.Equal(t, "", CleanUpString(""))
	require.Equal(t, "a_b", CleanUpString("a b"))
	require.Equal(t, "hello_world", CleanUpString("hello, world!"))
	require.Equal(t, "hello_you_2", CleanUpString("hello you 2"))
	require.Equal(t, "1000", CleanUpString("$1000"))
	require.Equal(t, "", CleanUpString("*+$^=)"))
}

func TestUnitMapGetOrDefault(t *testing.T) {

	require.Equal(t, "", unitMapGetOrDefault(""))
	require.Equal(t, "seconds", unitMapGetOrDefault("s"))
	require.Equal(t, "invalid", unitMapGetOrDefault("invalid"))

}

func TestPerUnitMapGetOrDefault(t *testing.T) {

	require.Equal(t, "", perUnitMapGetOrDefault(""))
	require.Equal(t, "second", perUnitMapGetOrDefault("s"))
	require.Equal(t, "invalid", perUnitMapGetOrDefault("invalid"))

}

func TestRemoveItem(t *testing.T) {

	require.Equal(t, []string{}, removeItem([]string{}, "test"))
	require.Equal(t, []string{}, removeItem([]string{}, ""))
	require.Equal(t, []string{"a", "b", "c"}, removeItem([]string{"a", "b", "c"}, "d"))
	require.Equal(t, []string{"a", "b", "c"}, removeItem([]string{"a", "b", "c"}, ""))
	require.Equal(t, []string{"a", "b"}, removeItem([]string{"a", "b", "c"}, "c"))
	require.Equal(t, []string{"a", "c"}, removeItem([]string{"a", "b", "c"}, "b"))
	require.Equal(t, []string{"b", "c"}, removeItem([]string{"a", "b", "c"}, "a"))

}

func TestBuildPromCompliantNameWithNormalize(t *testing.T) {

	defer testutil.SetFeatureGateForTest(normalizeNameGate.ID, true)()
	require.Equal(t, "system_io_bytes_total", BuildPromCompliantName(createCounter("system.io", "By"), ""))
	require.Equal(t, "system_network_io_bytes_total", BuildPromCompliantName(createCounter("network.io", "By"), "system"))
	require.Equal(t, "_3_14_digits", BuildPromCompliantName(createGauge("3.14 digits", ""), ""))

}

func TestBuildPromCompliantNameWithoutNormalize(t *testing.T) {

	defer testutil.SetFeatureGateForTest(normalizeNameGate.ID, false)()
	require.Equal(t, "system_io", BuildPromCompliantName(createCounter("system.io", "By"), ""))
	require.Equal(t, "system_network_io", BuildPromCompliantName(createCounter("network.io", "By"), "system"))
	require.Equal(t, "system_network_I_O", BuildPromCompliantName(createCounter("network (I/O)", "By"), "system"))
	require.Equal(t, "_3_14_digits", BuildPromCompliantName(createGauge("3.14 digits", "By"), ""))

}
