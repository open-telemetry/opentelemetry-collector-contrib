// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kineticaexporter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

var testServer *httptest.Server
var baseURL string

// var showTableMetricGaugeResponse = "\x04OK\x00&show_table_response\xe2\x13\"otel.metric_gauge\x02\"otel.metric_gauge\x00\x02\x00\x00\x02$303100960528700881\x00\x02\xc2\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"metric_name\",\"type\":\"string\"},{\"name\":\"metric_description\",\"type\":[\"string\",\"null\"]},{\"name\":\"metric_unit\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x08\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00$metric_description\x06\x08data\x0echar256\x10nullable\x00\x16metric_name\x04\x08data\x0echar256\x00\x16metric_unit\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x00\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06784\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xda\x02{\"table_name\":\"otel.metric_gauge\",\"type_id\":\"303100960528700881\",\"options\":{\"foreign_keys\":\"\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricGaugeDatapointResponse = "\x04OK\x00&show_table_response\xe2\x176otel.metric_gauge_datapoint\x026otel.metric_gauge_datapoint\x00\x02\x00\x00\x02&6685416151952470302\x00\x02\xa8\x04{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"start_time_unix\",\"type\":\"long\"},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"gauge_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"flags\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x0c\nflags\x04\x08data\x10nullable\x00\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x16gauge_value\x04\x08data\x10nullable\x00\x04id\x06\x08data\x16primary_key\x08uuid\x00\x1estart_time_unix\x04\x08data\x12timestamp\x00\x12time_unix\x04\x08data\x12timestamp\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x8e\x01(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0460\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xfe\x03{\"table_name\":\"otel.metric_gauge_datapoint\",\"type_id\":\"6685416151952470302\",\"options\":{\"foreign_keys\":\"(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricGaugeDatapointAttributeResponse = "\x04OK\x00&show_table_response\xea\x1bJotel.metric_gauge_datapoint_attribute\x02Jotel.metric_gauge_datapoint_attribute\x00\x02\x00\x00\x02(12082305977685050188\x00\x02\x84\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x10\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xa2\x01(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06429\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xa8\x04{\"table_name\":\"otel.metric_gauge_datapoint_attribute\",\"type_id\":\"12082305977685050188\",\"options\":{\"foreign_keys\":\"(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricGaugeDatapointExemplarResponse = "\x04OK\x00&show_table_response\xa2\x1aHotel.metric_gauge_datapoint_exemplar\x02Hotel.metric_gauge_datapoint_exemplar\x00\x02\x00\x00\x02(12834568480651246655\x00\x02\x9c\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"gauge_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"trace_id\",\"type\":[\"string\",\"null\"]},{\"name\":\"span_id\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x16gauge_value\x04\x08data\x10nullable\x00\x0espan_id\x06\x08data\x0cchar16\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x10trace_id\x06\x08data\x0cchar32\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xa0\x01(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_exemplar\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06112\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xa4\x04{\"table_name\":\"otel.metric_gauge_datapoint_exemplar\",\"type_id\":\"12834568480651246655\",\"options\":{\"foreign_keys\":\"(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_exemplar\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricGaugeDatapointExemplarAttributeResponse = "\x04OK\x00&show_table_response\xda\x1d\\otel.metric_gauge_datapoint_exemplar_attribute\x02\\otel.metric_gauge_datapoint_exemplar_attribute\x00\x02\x00\x00\x02(16080071459349192050\x00\x02\xd2\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb4\x01(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_exemplar_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06445\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xcc\x04{\"table_name\":\"otel.metric_gauge_datapoint_exemplar_attribute\",\"type_id\":\"16080071459349192050\",\"options\":{\"foreign_keys\":\"(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_datapoint_exemplar_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricGaugeResourceAttributeResponse = "\x04OK\x00&show_table_response\xc2\x1aHotel.metric_gauge_resource_attribute\x02Hotel.metric_gauge_resource_attribute\x00\x02\x00\x00\x02&2869956448577804410\x00\x02\xb4\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xa0\x01(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_resource_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06413\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xa2\x04{\"table_name\":\"otel.metric_gauge_resource_attribute\",\"type_id\":\"2869956448577804410\",\"options\":{\"foreign_keys\":\"(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_resource_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricGaugeScopeAttributeResponse = "\x04OK\x00&show_table_response\xc8\x1cBotel.metric_gauge_scope_attribute\x02Botel.metric_gauge_scope_attribute\x00\x02\x00\x00\x02&9441115615811019715\x00\x02\xde\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"gauge_id\",\"type\":\"string\"},{\"name\":\"name\",\"type\":[\"string\",\"null\"]},{\"name\":\"version\",\"type\":[\"string\",\"null\"]},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x10gauge_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x08name\x06\x08data\x0echar256\x10nullable\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0eversion\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x9a\x01(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_scope_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06925\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x96\x04{\"table_name\":\"otel.metric_gauge_scope_attribute\",\"type_id\":\"9441115615811019715\",\"options\":{\"foreign_keys\":\"(gauge_id) references otel.metric_gauge(gauge_id) as fk_gauge_scope_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"

// var showTableMetricSumResponse = "\x04OK\x00&show_table_response\xba\x16\x1eotel.metric_sum\x02\x1eotel.metric_sum\x00\x02\x00\x00\x02&4795364268414409479\x00\x02\x8c\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"metric_name\",\"type\":\"string\"},{\"name\":\"metric_description\",\"type\":[\"string\",\"null\"]},{\"name\":\"metric_unit\",\"type\":[\"string\",\"null\"]},{\"name\":\"aggregation_temporality\",\"type\":[\"int\",\"null\"]},{\"name\":\"is_monotonic\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x0c.aggregation_temporality\x04\x08data\x10nullable\x00\x18is_monotonic\x06\x08data\x10nullable\x0eboolean\x00$metric_description\x06\x08data\x0echar256\x10nullable\x00\x16metric_name\x04\x08data\x0echar256\x00\x16metric_unit\x06\x08data\x0echar256\x10nullable\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x00\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06789\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xd8\x02{\"table_name\":\"otel.metric_sum\",\"type_id\":\"4795364268414409479\",\"options\":{\"foreign_keys\":\"\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricSumDatapointResponse = "\x04OK\x00&show_table_response\xa4\x172otel.metric_sum_datapoint\x022otel.metric_sum_datapoint\x00\x02\x00\x00\x02&8856130788389266964\x00\x02\xa0\x04{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"start_time_unix\",\"type\":\"long\"},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"sum_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"flags\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x0c\nflags\x04\x08data\x10nullable\x00\x04id\x06\x08data\x16primary_key\x08uuid\x00\x1estart_time_unix\x04\x08data\x12timestamp\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12sum_value\x04\x08data\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys~(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0460\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xea\x03{\"table_name\":\"otel.metric_sum_datapoint\",\"type_id\":\"8856130788389266964\",\"options\":{\"foreign_keys\":\"(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricSumDatapointAttributeResponse = "\x04OK\x00&show_table_response\xb6\x1bFotel.metric_sum_datapoint_attribute\x02Fotel.metric_sum_datapoint_attribute\x00\x02\x00\x00\x02(14560342835696669889\x00\x02\x80\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x10\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x92\x01(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06429\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x94\x04{\"table_name\":\"otel.metric_sum_datapoint_attribute\",\"type_id\":\"14560342835696669889\",\"options\":{\"foreign_keys\":\"(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricSumDatapointExemplarResponse = "\x04OK\x00&show_table_response\xe2\x19Dotel.metric_sum_datapoint_exemplar\x02Dotel.metric_sum_datapoint_exemplar\x00\x02\x00\x00\x02&6993309962287390761\x00\x02\x94\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"sum_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"trace_id\",\"type\":[\"string\",\"null\"]},{\"name\":\"span_id\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x0espan_id\x06\x08data\x0cchar16\x10nullable\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12sum_value\x04\x08data\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x10trace_id\x06\x08data\x0cchar32\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x90\x01(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_exemplar\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06112\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x8e\x04{\"table_name\":\"otel.metric_sum_datapoint_exemplar\",\"type_id\":\"6993309962287390761\",\"options\":{\"foreign_keys\":\"(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_exemplar\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricSumDatapointExemplarAttributeResponse = "\x04OK\x00&show_table_response\xa2\x1dXotel.metric_sum_datapoint_exemplar_attribute\x02Xotel.metric_sum_datapoint_exemplar_attribute\x00\x02\x00\x00\x02&6609162607650215098\x00\x02\xce\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xa4\x01(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_exemplar_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06445\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xb6\x04{\"table_name\":\"otel.metric_sum_datapoint_exemplar_attribute\",\"type_id\":\"6609162607650215098\",\"options\":{\"foreign_keys\":\"(sum_id) references otel.metric_sum(sum_id) as fk_sum_datapoint_exemplar_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricSumResourceAttributeResponse = "\x04OK\x00&show_table_response\x92\x1aDotel.metric_sum_resource_attribute\x02Dotel.metric_sum_resource_attribute\x00\x02\x00\x00\x02(17062173411799121943\x00\x02\xb0\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x90\x01(sum_id) references otel.metric_sum(sum_id) as fk_sum_resource_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06413\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x90\x04{\"table_name\":\"otel.metric_sum_resource_attribute\",\"type_id\":\"17062173411799121943\",\"options\":{\"foreign_keys\":\"(sum_id) references otel.metric_sum(sum_id) as fk_sum_resource_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricSumScopeAttributeResponse = "\x04OK\x00&show_table_response\x98\x1c>otel.metric_sum_scope_attribute\x02>otel.metric_sum_scope_attribute\x00\x02\x00\x00\x02(17314337386934434030\x00\x02\xda\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"sum_id\",\"type\":\"string\"},{\"name\":\"name\",\"type\":[\"string\",\"null\"]},{\"name\":\"version\",\"type\":[\"string\",\"null\"]},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x08name\x06\x08data\x0echar256\x10nullable\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0csum_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x0eversion\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x8a\x01(sum_id) references otel.metric_sum(sum_id) as fk_sum_scope_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06925\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x84\x04{\"table_name\":\"otel.metric_sum_scope_attribute\",\"type_id\":\"17314337386934434030\",\"options\":{\"foreign_keys\":\"(sum_id) references otel.metric_sum(sum_id) as fk_sum_scope_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"

// var showTableMetricHistogramResponse = "\x04OK\x00&show_table_response\xde\x15*otel.metric_histogram\x02*otel.metric_histogram\x00\x02\x00\x00\x02(11917054661379628298\x00\x02\xbc\x04{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"metric_name\",\"type\":\"string\"},{\"name\":\"metric_description\",\"type\":[\"string\",\"null\"]},{\"name\":\"metric_unit\",\"type\":[\"string\",\"null\"]},{\"name\":\"aggregation_temporality\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\n.aggregation_temporality\x06\x08data\x08int8\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00$metric_description\x06\x08data\x0echar256\x10nullable\x00\x16metric_name\x04\x08data\x0echar256\x00\x16metric_unit\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x00\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06785\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xe6\x02{\"table_name\":\"otel.metric_histogram\",\"type_id\":\"11917054661379628298\",\"options\":{\"foreign_keys\":\"\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramDatapointResponse = "\x04OK\x00&show_table_response\xfa\x1b>otel.metric_histogram_datapoint\x02>otel.metric_histogram_datapoint\x00\x02\x00\x00\x02(10894883148954291753\x00\x02\xc0\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"start_time_unix\",\"type\":[\"long\",\"null\"]},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]},{\"name\":\"data_sum\",\"type\":[\"double\",\"null\"]},{\"name\":\"data_min\",\"type\":[\"double\",\"null\"]},{\"name\":\"data_max\",\"type\":[\"double\",\"null\"]},{\"name\":\"flags\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\ncount\x04\x08data\x10nullable\x00\x10data_max\x04\x08data\x10nullable\x00\x10data_min\x04\x08data\x10nullable\x00\x10data_sum\x04\x08data\x10nullable\x00\nflags\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x04id\x06\x08data\x16primary_key\x08uuid\x00\x1estart_time_unix\x06\x08data\x12timestamp\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xae\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0484\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xa8\x04{\"table_name\":\"otel.metric_histogram_datapoint\",\"type_id\":\"10894883148954291753\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramDatapointBucketCountResponse = "\x04OK\x00&show_table_response\xdc\x17Xotel.metric_histogram_datapoint_bucket_count\x02Xotel.metric_histogram_datapoint_bucket_count\x00\x02\x00\x00\x02(15633069051719792903\x00\x02\x96\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"count_id\",\"type\":\"string\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]}]}\x00\x02\x00\x00\x02\x08\ncount\x04\x08data\x10nullable\x00\x10count_id\x06\x08data\x16primary_key\x08uuid\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xc8\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_bucket_count\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0456\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xdc\x04{\"table_name\":\"otel.metric_histogram_datapoint_bucket_count\",\"type_id\":\"15633069051719792903\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_bucket_count\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramDatapointExplicitBoundResponse = "\x04OK\x00&show_table_response\x94\x18\\otel.metric_histogram_datapoint_explicit_bound\x02\\otel.metric_histogram_datapoint_explicit_bound\x00\x02\x00\x00\x02&4375897286985442453\x00\x02\xac\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"bound_id\",\"type\":\"string\"},{\"name\":\"explicit_bound\",\"type\":[\"double\",\"null\"]}]}\x00\x02\x00\x00\x02\x08\x10bound_id\x06\x08data\x16primary_key\x08uuid\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x1cexplicit_bound\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xcc\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_explicit_bound\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0456\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xe2\x04{\"table_name\":\"otel.metric_histogram_datapoint_explicit_bound\",\"type_id\":\"4375897286985442453\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_explicit_bound\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramDatapointAttributeResponse = "\x04OK\x00&show_table_response\xd2\x1cRotel.metric_histogram_datapoint_attribute\x02Rotel.metric_histogram_datapoint_attribute\x00\x02\x00\x00\x02(10084565520000607329\x00\x02\x8c\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x10\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xc2\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06429\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xd0\x04{\"table_name\":\"otel.metric_histogram_datapoint_attribute\",\"type_id\":\"10084565520000607329\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramDatapointExemplarResponse = "\x04OK\x00&show_table_response\x9a\x1bPotel.metric_histogram_datapoint_exemplar\x02Potel.metric_histogram_datapoint_exemplar\x00\x02\x00\x00\x02(14600123735022941069\x00\x02\xac\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"histogram_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"trace_id\",\"type\":[\"string\",\"null\"]},{\"name\":\"span_id\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x1ehistogram_value\x04\x08data\x10nullable\x00\x0espan_id\x06\x08data\x0cchar16\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x10trace_id\x06\x08data\x0cchar32\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xc0\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_exemplar\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06112\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xcc\x04{\"table_name\":\"otel.metric_histogram_datapoint_exemplar\",\"type_id\":\"14600123735022941069\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_exemplar\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramDatapointExemplarAttributeResponse = "\x04OK\x00&show_table_response\xc2\x1edotel.metric_histogram_datapoint_exemplar_attribute\x02dotel.metric_histogram_datapoint_exemplar_attribute\x00\x02\x00\x00\x02(12559505247430693380\x00\x02\xda\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xd4\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_exemplar_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06445\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xf4\x04{\"table_name\":\"otel.metric_histogram_datapoint_exemplar_attribute\",\"type_id\":\"12559505247430693380\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_datapoint_exemplar_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramResourceAttributeResponse = "\x04OK\x00&show_table_response\xaa\x1bPotel.metric_histogram_resource_attribute\x02Potel.metric_histogram_resource_attribute\x00\x02\x00\x00\x02&2390347709557952348\x00\x02\xbc\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xc0\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_resource_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06413\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xca\x04{\"table_name\":\"otel.metric_histogram_resource_attribute\",\"type_id\":\"2390347709557952348\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_resource_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricHistogramScopeAttributeResponse = "x04OK\x00&show_table_response\xb0\x1dJotel.metric_histogram_scope_attribute\x02Jotel.metric_histogram_scope_attribute\x00\x02\x00\x00\x02&6359661233604446364\x00\x02\xe6\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"name\",\"type\":[\"string\",\"null\"]},{\"name\":\"version\",\"type\":[\"string\",\"null\"]},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x08name\x06\x08data\x0echar256\x10nullable\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0eversion\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xba\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_scope_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06925\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xbe\x04{\"table_name\":\"otel.metric_histogram_scope_attribute\",\"type_id\":\"6359661233604446364\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_scope_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"

// var showTableMetricExponentialHistogramResponse = "\x04OK\x00&show_table_response\xf6\x152otel.metric_exp_histogram\x022otel.metric_exp_histogram\x00\x02\x00\x00\x02(11917054661379628298\x00\x02\xbc\x04{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"metric_name\",\"type\":\"string\"},{\"name\":\"metric_description\",\"type\":[\"string\",\"null\"]},{\"name\":\"metric_unit\",\"type\":[\"string\",\"null\"]},{\"name\":\"aggregation_temporality\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\n.aggregation_temporality\x06\x08data\x08int8\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00$metric_description\x06\x08data\x0echar256\x10nullable\x00\x16metric_name\x04\x08data\x0echar256\x00\x16metric_unit\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x00\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06785\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xee\x02{\"table_name\":\"otel.metric_exp_histogram\",\"type_id\":\"11917054661379628298\",\"options\":{\"foreign_keys\":\"\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramDatapointResponse = "\x04OK\x00&show_table_response\xc2!Fotel.metric_exp_histogram_datapoint\x02Fotel.metric_exp_histogram_datapoint\x00\x02\x00\x00\x02(10341900393020543860\x00\x02\xcc\t{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"start_time_unix\",\"type\":[\"long\",\"null\"]},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]},{\"name\":\"data_sum\",\"type\":[\"double\",\"null\"]},{\"name\":\"scale\",\"type\":[\"int\",\"null\"]},{\"name\":\"zero_count\",\"type\":[\"long\",\"null\"]},{\"name\":\"buckets_positive_offset\",\"type\":[\"int\",\"null\"]},{\"name\":\"buckets_negative_offset\",\"type\":[\"int\",\"null\"]},{\"name\":\"data_min\",\"type\":[\"double\",\"null\"]},{\"name\":\"data_max\",\"type\":[\"double\",\"null\"]},{\"name\":\"flags\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x1a.buckets_negative_offset\x04\x08data\x10nullable\x00.buckets_positive_offset\x04\x08data\x10nullable\x00\ncount\x04\x08data\x10nullable\x00\x10data_max\x04\x08data\x10nullable\x00\x10data_min\x04\x08data\x10nullable\x00\x10data_sum\x04\x08data\x10nullable\x00\nflags\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x04id\x06\x08data\x16primary_key\x08uuid\x00\nscale\x04\x08data\x10nullable\x00\x1estart_time_unix\x06\x08data\x12timestamp\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x14zero_count\x04\x08data\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xbe\x01(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06104\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xc0\x04{\"table_name\":\"otel.metric_exp_histogram_datapoint\",\"type_id\":\"10341900393020543860\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramDatapointBucketPositiveCountResponse = "\x04OK\x00&show_table_response\xca\x18rotel.metric_exp_histogram_datapoint_bucket_positive_count\x02rotel.metric_exp_histogram_datapoint_bucket_positive_count\x00\x02\x00\x00\x02(15633069051719792903\x00\x02\x96\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"count_id\",\"type\":\"string\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]}]}\x00\x02\x00\x00\x02\x08\ncount\x04\x08data\x10nullable\x00\x10count_id\x06\x08data\x16primary_key\x08uuid\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xd8\x01(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0456\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x86\x05{\"table_name\":\"otel.metric_exp_histogram_datapoint_bucket_positive_count\",\"type_id\":\"15633069051719792903\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramDatapointBucketNegativeCountResponse = "\x04OK\x00&show_table_response\xca\x18rotel.metric_exp_histogram_datapoint_bucket_negative_count\x02rotel.metric_exp_histogram_datapoint_bucket_negative_count\x00\x02\x00\x00\x02(15633069051719792903\x00\x02\x96\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"count_id\",\"type\":\"string\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]}]}\x00\x02\x00\x00\x02\x08\ncount\x04\x08data\x10nullable\x00\x10count_id\x06\x08data\x16primary_key\x08uuid\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xd8\x01(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0456\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x86\x05{\"table_name\":\"otel.metric_exp_histogram_datapoint_bucket_negative_count\",\"type_id\":\"15633069051719792903\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_bucket_count\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramDatapointAttributeResponse = "\x04OK\x00&show_table_response\x8a\x1dZotel.metric_exp_histogram_datapoint_attribute\x02Zotel.metric_exp_histogram_datapoint_attribute\x00\x02\x00\x00\x02(10084565520000607329\x00\x02\x8c\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x10\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xd2\x01(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06429\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xe8\x04{\"table_name\":\"otel.metric_exp_histogram_datapoint_attribute\",\"type_id\":\"10084565520000607329\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramDatapointExemplarResponse = "\x04OK\x00&show_table_response\xb6\x1bXotel.metric_exp_histogram_datapoint_exemplar\x02Xotel.metric_exp_histogram_datapoint_exemplar\x00\x02\x00\x00\x02&9810344671294366059\x00\x02\xa0\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"sum_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"trace_id\",\"type\":[\"string\",\"null\"]},{\"name\":\"span_id\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x0espan_id\x06\x08data\x0cchar16\x10nullable\x00\x12sum_value\x04\x08data\x10nullable\x00\x12time_unix\x04\x08data\x12timestamp\x00\x10trace_id\x06\x08data\x0cchar32\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xd0\x01(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06112\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xe2\x04{\"table_name\":\"otel.metric_exp_histogram_datapoint_exemplar\",\"type_id\":\"9810344671294366059\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramDatapointExemplarAttributeResponse = "\x04OK\x00&show_table_response\xfa\x1elotel.metric_exp_histogram_datapoint_exemplar_attribute\x02lotel.metric_exp_histogram_datapoint_exemplar_attribute\x00\x02\x00\x00\x02(12559505247430693380\x00\x02\xda\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"exemplar_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x16exemplar_id\x06\x08data\x16primary_key\x08uuid\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xe4\x01(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06445\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x8c\x05{\"table_name\":\"otel.metric_exp_histogram_datapoint_exemplar_attribute\",\"type_id\":\"12559505247430693380\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_exp_histogram(histogram_id) as fk_exp_histogram_datapoint_exemplar_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramResourceAttributeResponse = "\x04OK\x00&show_table_response\xc2\x1bXotel.metric_exp_histogram_resource_attribute\x02Xotel.metric_exp_histogram_resource_attribute\x00\x02\x00\x00\x02&2390347709557952348\x00\x02\xbc\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xc0\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_resource_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06413\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xd2\x04{\"table_name\":\"otel.metric_exp_histogram_resource_attribute\",\"type_id\":\"2390347709557952348\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_resource_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
// var showTableMetricExponentialHistogramScopeAttributeResponse = "\x04OK\x00&show_table_response\xc8\x1dRotel.metric_exp_histogram_scope_attribute\x02Rotel.metric_exp_histogram_scope_attribute\x00\x02\x00\x00\x02&6359661233604446364\x00\x02\xe6\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"histogram_id\",\"type\":\"string\"},{\"name\":\"name\",\"type\":[\"string\",\"null\"]},{\"name\":\"version\",\"type\":[\"string\",\"null\"]},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x18histogram_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x08name\x06\x08data\x0echar256\x10nullable\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x0eversion\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xba\x01(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_scope_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06925\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xc6\x04{\"table_name\":\"otel.metric_exp_histogram_scope_attribute\",\"type_id\":\"6359661233604446364\",\"options\":{\"foreign_keys\":\"(histogram_id) references otel.metric_histogram(histogram_id) as fk_histogram_scope_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"

var showTableMetricSummaryResponse = "\x04OK\x00&show_table_response\xfe\x13&otel.metric_summary\x02&otel.metric_summary\x00\x02\x00\x00\x02(14940230562727554683\x00\x02\xc6\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"metric_name\",\"type\":\"string\"},{\"name\":\"metric_description\",\"type\":[\"string\",\"null\"]},{\"name\":\"metric_unit\",\"type\":[\"string\",\"null\"]}]}\x00\x02\x00\x00\x02\x08$metric_description\x06\x08data\x0echar256\x10nullable\x00\x16metric_name\x04\x08data\x0echar256\x00\x16metric_unit\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x00\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06784\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xe2\x02{\"table_name\":\"otel.metric_summary\",\"type_id\":\"14940230562727554683\",\"options\":{\"foreign_keys\":\"\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryDatapointResponse = "\x04OK\x00&show_table_response\xae\x19:otel.metric_summary_datapoint\x02:otel.metric_summary_datapoint\x00\x02\x00\x00\x02(11442617697458528032\x00\x02\x88\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"id\",\"type\":\"string\"},{\"name\":\"start_time_unix\",\"type\":[\"long\",\"null\"]},{\"name\":\"time_unix\",\"type\":\"long\"},{\"name\":\"count\",\"type\":[\"long\",\"null\"]},{\"name\":\"data_sum\",\"type\":[\"double\",\"null\"]},{\"name\":\"flags\",\"type\":[\"int\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\ncount\x04\x08data\x10nullable\x00\x10data_sum\x04\x08data\x10nullable\x00\nflags\x04\x08data\x10nullable\x00\x04id\x06\x08data\x16primary_key\x08uuid\x00\x1estart_time_unix\x06\x08data\x12timestamp\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x12time_unix\x04\x08data\x12timestamp\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\x9e\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0468\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\x94\x04{\"table_name\":\"otel.metric_summary_datapoint\",\"type_id\":\"11442617697458528032\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryDatapointAttributeResponse = "\x04OK\x00&show_table_response\x96\x1cNotel.metric_summary_datapoint_attribute\x02Notel.metric_summary_datapoint_attribute\x00\x02\x00\x00\x02$893873402458912716\x00\x02\x88\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x10\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb2\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06429\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xb8\x04{\"table_name\":\"otel.metric_summary_datapoint_attribute\",\"type_id\":\"893873402458912716\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryDatapointQuantileValuesResponse = "\x04OK\x00&show_table_response\xc2\x18Zotel.metric_summary_datapoint_quantile_values\x02Zotel.metric_summary_datapoint_quantile_values\x00\x02\x00\x00\x02&9163876868172339469\x00\x02\xf6\x03{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"datapoint_id\",\"type\":\"string\"},{\"name\":\"quantile_id\",\"type\":\"string\"},{\"name\":\"quantile\",\"type\":[\"double\",\"null\"]},{\"name\":\"value\",\"type\":[\"double\",\"null\"]}]}\x00\x02\x00\x00\x02\n\x18datapoint_id\x06\x08data\x16primary_key\x08uuid\x00\x10quantile\x04\x08data\x10nullable\x00\x16quantile_id\x06\x08data\x16primary_key\x08uuid\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\nvalue\x04\x08data\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb0\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_quantile\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x0464\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xc4\x04{\"table_name\":\"otel.metric_summary_datapoint_quantile_values\",\"type_id\":\"9163876868172339469\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_datapoint_quantile\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryResourceAttributesResponse = "\x04OK\x00&show_table_response\xfa\x1aLotel.metric_summary_resource_attribute\x02Lotel.metric_summary_resource_attribute\x00\x02\x00\x00\x02(11898281161813583768\x00\x02\xb8\x05{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x0e\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xb0\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_resource_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06413\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xb8\x04{\"table_name\":\"otel.metric_summary_resource_attribute\",\"type_id\":\"11898281161813583768\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_resource_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"
var showTableMetricSummaryScopeAttributesResponse = "\x04OK\x00&show_table_response\xf8\x1cFotel.metric_summary_scope_attribute\x02Fotel.metric_summary_scope_attribute\x00\x02\x00\x00\x02$474749891710437496\x00\x02\xe2\x06{\"type\":\"record\",\"name\":\"type_name\",\"fields\":[{\"name\":\"summary_id\",\"type\":\"string\"},{\"name\":\"name\",\"type\":[\"string\",\"null\"]},{\"name\":\"version\",\"type\":[\"string\",\"null\"]},{\"name\":\"key\",\"type\":\"string\"},{\"name\":\"string_value\",\"type\":[\"string\",\"null\"]},{\"name\":\"bool_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"int_value\",\"type\":[\"int\",\"null\"]},{\"name\":\"double_value\",\"type\":[\"double\",\"null\"]},{\"name\":\"bytes_value\",\"type\":[\"bytes\",\"null\"]}]}\x00\x02\x00\x00\x02\x12\x14bool_value\x06\x08data\x10nullable\x0eboolean\x00\x16bytes_value\x04\x14store_only\x10nullable\x00\x18double_value\x04\x08data\x10nullable\x00\x12int_value\x04\x08data\x10nullable\x00\x06key\x08\x08data\x16primary_key\x0echar128\x08dict\x00\x08name\x06\x08data\x0echar256\x10nullable\x00\x18string_value\x06\x08data\x0echar256\x10nullable\x00\x14summary_id\x08\x08data\x16primary_key\x12shard_key\x08uuid\x00\x0eversion\x06\x08data\x0echar256\x10nullable\x00\x00\x00\x028\"attribute_indexes\x00 collection_names\x08otel$compressed_columns\x000datasource_subscriptions\x00\x18foreign_keys\xaa\x01(summary_id) references otel.metric_summary(summary_id) as fk_summary_scope_attribute\"foreign_shard_key\x00$global_access_mode\x14read_write,is_automatic_partition\nfalse\x10is_dirty\x00\"is_view_persisted\x00\"last_refresh_time\x00(owner_resource_group<kinetica_system_resource_group*partition_definitions\x004partition_definitions_json\x04{}\x1cpartition_keys\x00\x1cpartition_type\x08NONE\x18record_bytes\x06925\x1crefresh_method\x00&remaining_table_ttl\x04-1\"request_avro_json\xa8\x04{\"table_name\":\"otel.metric_summary_scope_attribute\",\"type_id\":\"474749891710437496\",\"options\":{\"foreign_keys\":\"(summary_id) references otel.metric_summary(summary_id) as fk_summary_scope_attribute\",\"is_replicated\":\"false\",\"is_result_table\":\"false\",\"no_error_if_exists\":\"true\"}}\"request_avro_type\x10is_table\x16schema_name\x08otel&strategy_definitionR( ( VRAM 1, RAM 5, DISK0 5, PERSIST 5 ) )\x1atable_monitor\x04{}\x12table_ttl\x04-1\x16total_bytes\x020\x1euser_chunk_size\x0e8000000\x1eview_table_name\x00\x00\x00\x00\x00\x00\x01\x01\x00\x00"

func TestExporter_pushMetricsData(t *testing.T) {
	t.Parallel()
	t.Run("push success", func(t *testing.T) {
		exporter := newTestMetricsExporter(t)
		mustPushMetricsData(t, exporter, simpleMetrics(3))

		require.Equal(t, 15, 15)
	})
	t.Run("push failure", func(t *testing.T) {
		exporter := newTestMetricsExporter(t)
		err := exporter.pushMetricsData(context.TODO(), simpleMetrics(1))
		if err == nil {
			err = fmt.Errorf("mock insert error")
		}
		require.Error(t, err)
	})
}

func Benchmark_pushMetricsData(b *testing.B) {
	pm := simpleMetrics(1)
	exporter := newTestMetricsExporter(&testing.T{})
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := exporter.pushMetricsData(context.TODO(), pm)
		require.NoError(b, err)
	}
}

// simpleMetrics there will be added two ResourceMetrics and each of them have count data point
func simpleMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 1")
	rm.Resource().Attributes().PutStr("Resource Attributes 1", "value1")
	rm.Resource().SetDroppedAttributesCount(10)
	rm.SetSchemaUrl("Resource SchemaUrl 1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 1")
	sm.Scope().Attributes().PutStr("Scope Attributes 1", "value1")
	sm.Scope().SetDroppedAttributesCount(10)
	sm.Scope().SetName("Scope name 1")
	sm.Scope().SetVersion("Scope version 1")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetDoubleValue(11.234)
		dp.Attributes().PutStr("sum_label_1", "1")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetDoubleValue(55.22)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	rm = metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "demo 2")
	rm.Resource().Attributes().PutStr("Resource Attributes 2", "value2")
	rm.Resource().SetDroppedAttributesCount(20)
	rm.SetSchemaUrl("Resource SchemaUrl 2")
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 2")
	sm.Scope().Attributes().PutStr("Scope Attributes 2", "value2")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 2")
	sm.Scope().SetVersion("Scope version 2")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}

	// add a different scope metrics
	sm = rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("Scope SchemaUrl 3")
	sm.Scope().Attributes().PutStr("Scope Attributes 3", "value3")
	sm.Scope().SetDroppedAttributesCount(20)
	sm.Scope().SetName("Scope name 3")
	sm.Scope().SetVersion("Scope version 3")
	for i := 0; i < count; i++ {
		// gauge
		m := sm.Metrics().AppendEmpty()
		m.SetName("gauge metrics")
		m.SetUnit("count")
		m.SetDescription("This is a gauge metrics")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("gauge_label_3", "3")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars := dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// sum
		m = sm.Metrics().AppendEmpty()
		m.SetName("sum metrics")
		m.SetUnit("count")
		m.SetDescription("This is a sum metrics")
		dp = m.SetEmptySum().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("sum_label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		exemplars = dp.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a histogram metrics")
		dpHisto := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		dpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpHisto.SetCount(1)
		dpHisto.SetSum(1)
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.Attributes().PutStr("key", "value")
		dpHisto.ExplicitBounds().FromRaw([]float64{0, 0, 0, 0, 0})
		dpHisto.BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpHisto.SetMin(0)
		dpHisto.SetMax(1)
		exemplars = dpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// exp histogram
		m = sm.Metrics().AppendEmpty()
		m.SetName("exp histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a exp histogram metrics")
		dpExpHisto := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		dpExpHisto.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpExpHisto.SetSum(1)
		dpExpHisto.SetMin(0)
		dpExpHisto.SetMax(1)
		dpExpHisto.SetZeroCount(0)
		dpExpHisto.SetCount(1)
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Attributes().PutStr("key", "value")
		dpExpHisto.Negative().SetOffset(1)
		dpExpHisto.Negative().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})
		dpExpHisto.Positive().SetOffset(1)
		dpExpHisto.Positive().BucketCounts().FromRaw([]uint64{0, 0, 0, 1, 0})

		exemplars = dpExpHisto.Exemplars().AppendEmpty()
		exemplars.SetIntValue(54)
		exemplars.FilteredAttributes().PutStr("key", "value")
		exemplars.FilteredAttributes().PutStr("key2", "value2")
		exemplars.SetSpanID([8]byte{1, 2, 3, byte(i)})
		exemplars.SetTraceID([16]byte{1, 2, 3, byte(i)})

		// summary
		m = sm.Metrics().AppendEmpty()
		m.SetName("summary histogram metrics")
		m.SetUnit("ms")
		m.SetDescription("This is a summary metrics")
		summary := m.SetEmptySummary().DataPoints().AppendEmpty()
		summary.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		summary.Attributes().PutStr("key", "value")
		summary.Attributes().PutStr("key2", "value2")
		summary.SetCount(1)
		summary.SetSum(1)
		quantileValues := summary.QuantileValues().AppendEmpty()
		quantileValues.SetValue(1)
		quantileValues.SetQuantile(1)
	}
	return metrics
}

func mustPushMetricsData(t *testing.T, exporter *kineticaMetricsExporter, md pmetric.Metrics) {
	err := exporter.pushMetricsData(context.TODO(), md)
	require.NoError(t, err)
}

func newTestMetricsExporter(t *testing.T) *kineticaMetricsExporter {
	exporter := newMetricsExporter(zaptest.NewLogger(t), withTestExporterConfig()(baseURL))
	require.NoError(t, exporter.start(context.TODO(), nil))

	t.Cleanup(func() { _ = exporter.shutdown(context.TODO()) })
	return exporter
}

func withTestExporterConfig(fns ...func(*Config)) func(string) *Config {
	return func(endpoint string) *Config {
		var configMods []func(*Config)
		configMods = append(configMods, func(cfg *Config) {
			cfg.Host = endpoint
		})
		configMods = append(configMods, fns...)
		return withDefaultConfig(configMods...)
	}
}

// Handler functions for different endpoints

func handleInsertRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Custom handling for endpoint 2 POST request
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("\x04OK\x00.insert_records_response\x08\x00\x06\x00\x00\x00"))
}

func handleExecuteSql(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	responseBytes := []byte("\x04OK\x00(execute_sql_response\xd4\x05\x02\xf6\x03{\"type\":\"record\",\"name\":\"generic_response\",\"fields\":[{\"name\":\"column_1\",\"type\":{\"type\":\"array\",\"items\":\"string\"}},{\"name\":\"column_headers\",\"type\":{\"type\":\"array\",\"items\":\"string\"}},{\"name\":\"column_datatypes\",\"type\":{\"type\":\"array\",\"items\":\"string\"}}]}$\x00\x02\ndummy\x00\x02\fstring\x00\x00\x01\x00\x00\b X-Kinetica-Group\x06DDL\ncount\x020\x1alast_endpoint\x1a/create/table.total_number_of_records\x020\x00\x00")
	w.Write(responseBytes)
}

func handleShowTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	requestBody := string(body)

	// Print the request body
	// fmt.Println("Request Body:", requestBody)

	w.Header().Set("Content-Type", "application/octet-stream")

	finalResponseBytes := getShowTableResponse(requestBody)

	// fmt.Println("Response bytes => ", string(finalResponseBytes))

	w.Write(finalResponseBytes)
}

func getShowTableResponse(requestBody string) []byte {
	if strings.Contains(requestBody, "metric_summary_scope_attribute") {
		return []byte(showTableMetricSummaryScopeAttributesResponse)
	} else if strings.Contains(requestBody, "metric_summary_resource_attribute") {
		return []byte(showTableMetricSummaryResourceAttributesResponse)
	} else if strings.Contains(requestBody, "metric_summary_datapoint_quantile_values") {
		return []byte(showTableMetricSummaryDatapointQuantileValuesResponse)
	} else if strings.Contains(requestBody, "metric_summary_datapoint_attribute") {
		return []byte(showTableMetricSummaryDatapointAttributeResponse)
	} else if strings.Contains(requestBody, "metric_summary_datapoint") {
		return []byte(showTableMetricSummaryDatapointResponse)
	} else if strings.Contains(requestBody, "metric_summary") {
		return []byte(showTableMetricSummaryResponse)
	} else {
		return []byte("")
	}
}

// Setup function (runs before tests start)
func TestMain(m *testing.M) {
	// Create a test server with a simple handler function
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("In main ...")
		switch r.URL.Path {
		case "/insert/records":
			handleInsertRecords(w, r)
		case "/show/table":
			handleShowTable(w, r)
		case "/execute/sql":
			handleExecuteSql(w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}))

	baseURL = testServer.URL // Store the base URL for test requests

	// Run the tests
	code := m.Run()

	// Teardown: Close the test server after all tests are finished
	testServer.Close()
	// Perform any other teardown operations here if needed

	// Exit with the test status code
	// This allows TestMain to report the result of the tests
	// You can also perform further actions based on the test results
	os.Exit(code)
}
